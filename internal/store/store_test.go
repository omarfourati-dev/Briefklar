package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Start(m)) }

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), testdb.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	testdb.Reset(t)
	return s
}

func TestMigrationsRunTwice(t *testing.T) {
	open(t)
	open(t) // second start must not fail on existing tables
}

func TestUsers(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	u, err := s.CreateUser(ctx, "Anna@Firma.DE", "hash", "Anna", "user", 20)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "anna@firma.de" || !u.Enabled || u.ID == "" {
		t.Fatalf("got %+v", u)
	}
	if _, err := s.CreateUser(ctx, "anna@firma.de", "h", "A", "user", 20); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate: %v", err)
	}
	byMail, err := s.UserByEmail(ctx, "ANNA@firma.de")
	if err != nil || byMail.ID != u.ID {
		t.Fatalf("by email: %+v %v", byMail, err)
	}
	if _, err := s.UserByID(ctx, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := s.UserByEmail(ctx, "nobody@x.de"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}

	off, err := s.SetEnabled(ctx, u.ID, false)
	if err != nil || off.Enabled {
		t.Fatalf("disable: %+v %v", off, err)
	}
	if active, _ := s.IsActive(ctx, u.ID); active {
		t.Fatal("disabled user is active")
	}
	if active, err := s.IsActive(ctx, "not-a-uuid"); active || err != nil {
		t.Fatalf("bad id active=%v err=%v", active, err)
	}
	if err := s.SetPassword(ctx, u.ID, "new"); err != nil {
		t.Fatal(err)
	}
	again, _ := s.UserByID(ctx, u.ID)
	if again.PasswordHash != "new" {
		t.Fatal("password not changed")
	}
}

func TestQuota(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	u, _ := s.CreateUser(ctx, "q@x.de", "h", "Q", "user", 2)
	day := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	for want := 1; want <= 2; want++ {
		used, ok, err := s.TakeQuota(ctx, u.ID, day)
		if err != nil || !ok || used != want {
			t.Fatalf("take %d: used=%d ok=%v err=%v", want, used, ok, err)
		}
	}
	if _, ok, _ := s.TakeQuota(ctx, u.ID, day); ok {
		t.Fatal("third call must be refused")
	}
	if _, ok, _ := s.TakeQuota(ctx, u.ID, day.AddDate(0, 0, 1)); !ok {
		t.Fatal("next day must be allowed")
	}
	list, err := s.ListUsers(ctx, day)
	if err != nil || len(list) != 1 || list[0].UsedToday != 2 {
		t.Fatalf("list: %+v %v", list, err)
	}

	demo, _ := s.CreateUser(ctx, "demo@x.de", "h", "Demo", "demo", 0)
	if _, ok, _ := s.TakeQuota(ctx, demo.ID, day); ok {
		t.Fatal("limit 0 must always refuse")
	}
}
