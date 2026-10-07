package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"sync"
	"sync/atomic"
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

func migrationRows(t *testing.T, s *Store) (rows, files int) {
	t.Helper()
	if err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	return rows, len(names)
}

func TestMigrationsRunTwice(t *testing.T) {
	open(t)
	s := open(t) // second start must not fail on existing tables
	if rows, files := migrationRows(t, s); rows != files {
		t.Fatalf("schema_migrations rows=%d, migration files=%d", rows, files)
	}
	var tables int
	err := s.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables WHERE table_name IN ('app_user', 'usage_day')`).Scan(&tables)
	if err != nil || tables != 2 {
		t.Fatalf("tables=%d err=%v", tables, err)
	}
}

func TestInvalidAndUnknownIDs(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	day := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	unknown := "00000000-0000-0000-0000-000000000000"

	if err := s.SetPassword(ctx, "not-a-uuid", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetPassword bad id: %v", err)
	}
	if used, ok, err := s.TakeQuota(ctx, "not-a-uuid", day); used != 0 || ok || err != nil {
		t.Fatalf("TakeQuota bad id: used=%d ok=%v err=%v", used, ok, err)
	}
	if err := s.SetPassword(ctx, unknown, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetPassword unknown id: %v", err)
	}
	if _, err := s.SetEnabled(ctx, unknown, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetEnabled unknown id: %v", err)
	}
	if _, err := s.SetEnabled(ctx, "not-a-uuid", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetEnabled bad id: %v", err)
	}
	if used, ok, err := s.TakeQuota(ctx, unknown, day); used != 0 || ok || err != nil {
		t.Fatalf("TakeQuota unknown id: used=%d ok=%v err=%v", used, ok, err)
	}
}

func TestConcurrentFirstStart(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	if _, err := s.pool.Exec(ctx, `DROP TABLE IF EXISTS usage_day, app_user, schema_migrations`); err != nil {
		t.Fatal(err)
	}
	const n = 4
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, err := Open(ctx, testdb.URL)
			if err == nil {
				st.Close()
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Open: %v", err)
		}
	}
	if rows, files := migrationRows(t, s); rows != files {
		t.Fatalf("schema_migrations rows=%d, migration files=%d", rows, files)
	}
}

func TestQuotaIsAtomic(t *testing.T) {
	ctx := context.Background()
	s := open(t)
	u, err := s.CreateUser(ctx, "atomic@x.de", "h", "A", "user", 5)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := s.TakeQuota(ctx, u.ID, day); err != nil {
				t.Error(err)
			} else if ok {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 5 {
		t.Fatalf("granted %d, want 5", granted.Load())
	}
	var count int
	err = s.pool.QueryRow(ctx, `SELECT count FROM usage_day WHERE user_id = $1::uuid`, u.ID).Scan(&count)
	if err != nil || count != 5 {
		t.Fatalf("usage_day.count=%d err=%v", count, err)
	}
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
