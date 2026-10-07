package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/omarfourati-dev/briefklar/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const (
	DemoEmail    = "demo@briefklar.app"
	DemoPassword = "demo-briefklar"
)

type Users interface {
	UserByEmail(ctx context.Context, email string) (store.User, error)
	UserByID(ctx context.Context, id string) (store.User, error)
	SetPassword(ctx context.Context, id, hash string) error
	IsActive(ctx context.Context, id string) (bool, error)
	CreateUser(ctx context.Context, email, hash, name, role string, dailyLimit int) (store.User, error)
	SetEnabled(ctx context.Context, id string, enabled bool) (store.User, error)
}

type Service struct {
	Users    Users
	Tokens   *Tokens
	Throttle *Throttle
	OnLogin  func(outcome string) // metrics hook: success | failed | throttled
	Log      *slog.Logger
	dummy    []byte
}

func NewService(users Users, tokens *Tokens) *Service {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		panic("auth: no randomness: " + err.Error())
	}
	dummy, err := bcrypt.GenerateFromPassword(random, bcrypt.DefaultCost)
	if err != nil {
		panic("auth: dummy hash: " + err.Error())
	}
	return &Service{Users: users, Tokens: tokens, Throttle: NewThrottle(), OnLogin: func(string) {},
		Log: slog.Default(), dummy: dummy}
}

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// ValidatePassword is the single password rule: at least 12 characters, at most 72 bytes (bcrypt limit).
func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < 12 {
		return errors.New("Das Passwort braucht mindestens 12 Zeichen.")
	}
	if len([]byte(pw)) > 72 {
		return errors.New("Das Passwort darf höchstens 72 Bytes lang sein (Umlaute zählen doppelt).")
	}
	return nil
}

// authenticate compares against a dummy hash for unknown e-mails, so the answer takes the same time
// and does not reveal which addresses have an account. It returns an error only if the lookup failed
// for another reason than a missing user (e.g. database down).
func (s *Service) authenticate(ctx context.Context, email, password string) (store.User, bool, error) {
	u, err := s.Users.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.User{}, false, err
	}
	hash := s.dummy
	if err == nil {
		hash = []byte(u.PasswordHash)
	}
	match := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	return u, err == nil && match && u.Enabled, nil
}

// Bootstrap creates the admin from the environment and the public demo account. Safe to run on every start.
// Create only: an existing account keeps its password. Without demo, an existing demo account is disabled.
func Bootstrap(ctx context.Context, users Users, adminEmail, adminPassword string, demo bool, dailyLimit int) error {
	ensure := func(email, password, name, role string, limit int) error {
		if _, err := users.UserByEmail(ctx, email); err == nil || !errors.Is(err, store.ErrNotFound) {
			return err
		}
		hash, err := HashPassword(password)
		if err != nil {
			return err
		}
		_, err = users.CreateUser(ctx, email, hash, name, role, limit)
		return err
	}
	if (adminEmail == "") != (adminPassword == "") {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	if adminEmail != "" {
		if err := ValidatePassword(adminPassword); err != nil {
			return fmt.Errorf("ADMIN_PASSWORD: %w", err)
		}
		if err := ensure(adminEmail, adminPassword, "Administrator", "admin", dailyLimit); err != nil {
			return err
		}
	}
	if demo {
		return ensure(DemoEmail, DemoPassword, "Demo", "demo", 0)
	}
	u, err := users.UserByEmail(ctx, DemoEmail)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.Enabled {
		_, err = users.SetEnabled(ctx, u.ID, false)
	}
	return err
}
