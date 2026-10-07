package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"

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
	_, _ = rand.Read(random)
	dummy, _ := bcrypt.GenerateFromPassword(random, bcrypt.DefaultCost)
	return &Service{Users: users, Tokens: tokens, Throttle: NewThrottle(), OnLogin: func(string) {},
		Log: slog.Default(), dummy: dummy}
}

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// authenticate compares against a dummy hash for unknown e-mails, so the answer takes the same time
// and does not reveal which addresses have an account.
func (s *Service) authenticate(ctx context.Context, email, password string) (store.User, bool) {
	u, err := s.Users.UserByEmail(ctx, email)
	hash := s.dummy
	if err == nil {
		hash = []byte(u.PasswordHash)
	}
	match := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	return u, err == nil && match && u.Enabled
}

// Bootstrap creates the admin from the environment and the public demo account. Safe to run on every start.
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
	if adminEmail != "" && adminPassword != "" {
		if len(adminPassword) < 12 {
			return fmt.Errorf("ADMIN_PASSWORD must have at least 12 characters")
		}
		if err := ensure(adminEmail, adminPassword, "Administrator", "admin", dailyLimit); err != nil {
			return err
		}
	}
	if demo {
		return ensure(DemoEmail, DemoPassword, "Demo", "demo", 0)
	}
	return nil
}
