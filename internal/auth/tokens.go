package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/omarfourati-dev/briefklar/internal/store"
)

const issuer = "briefklar"

type Claims struct {
	UserID string
	Email  string
	Name   string
	Role   string
}

type ctxKey struct{}

func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

type tokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokens(secret []byte, ttl time.Duration) (*Tokens, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 bytes")
	}
	return &Tokens{secret: secret, ttl: ttl, now: time.Now}, nil
}

func (t *Tokens) Issue(u store.User) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(t.ttl)
	c := tokenClaims{Email: u.Email, Name: u.Name, Role: u.Role, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: issuer, Subject: u.ID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	return s, exp, err
}

func (t *Tokens) Parse(token string) (Claims, error) {
	var c tokenClaims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now))
	if err != nil {
		return Claims{}, err
	}
	return Claims{UserID: c.Subject, Email: c.Email, Name: c.Name, Role: c.Role}, nil
}
