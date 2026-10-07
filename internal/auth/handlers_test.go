package auth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/store"
	"github.com/omarfourati-dev/briefklar/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Start(m)) }

const pw = "correct-horse-battery"

func setup(t *testing.T) (*Service, *store.Store, http.Handler) {
	t.Helper()
	st, err := store.Open(context.Background(), testdb.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	testdb.Reset(t)
	tok, _ := NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	svc := NewService(st, tok)
	svc.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", svc.Login)
	mux.Handle("GET /api/auth/me", svc.Require(http.HandlerFunc(svc.Me)))
	mux.Handle("POST /api/auth/password", svc.Require(http.HandlerFunc(svc.ChangePassword)))
	mux.Handle("GET /admin-only", svc.Require(svc.RequireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))))
	return svc, st, mux
}

func call(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, h http.Handler, email, password string) string {
	t.Helper()
	rec := call(h, "POST", "/api/auth/login", "", `{"email":"`+email+`","password":"`+password+`"}`)
	if rec.Code != 200 {
		t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
	}
	var out struct {
		Token string                `json:"token"`
		User  struct{ Role string } `json:"user"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Token
}

func create(t *testing.T, st *store.Store, email, role string) store.User {
	hash, _ := HashPassword(pw)
	u, err := st.CreateUser(context.Background(), email, hash, "Test "+role, role, 20)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestLoginAndMe(t *testing.T) {
	_, st, h := setup(t)
	create(t, st, "anna@x.de", "user")
	token := login(t, h, "ANNA@x.de", pw)
	rec := call(h, "GET", "/api/auth/me", token, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"email":"anna@x.de"`) {
		t.Fatalf("me: %d %s", rec.Code, rec.Body)
	}
}

func TestWrongPasswordAndUnknownUserLookAlike(t *testing.T) {
	_, st, h := setup(t)
	create(t, st, "anna@x.de", "user")
	a := call(h, "POST", "/api/auth/login", "", `{"email":"anna@x.de","password":"wrong-password-1"}`)
	b := call(h, "POST", "/api/auth/login", "", `{"email":"nobody@x.de","password":"wrong-password-1"}`)
	if a.Code != 401 || b.Code != 401 || a.Body.String() != b.Body.String() {
		t.Fatalf("a=%d %s b=%d %s", a.Code, a.Body, b.Code, b.Body)
	}
}

func TestThrottledLoginReturns429(t *testing.T) {
	_, st, h := setup(t)
	create(t, st, "anna@x.de", "user")
	for i := 0; i < 5; i++ {
		call(h, "POST", "/api/auth/login", "", `{"email":"anna@x.de","password":"wrong-password-1"}`)
	}
	rec := call(h, "POST", "/api/auth/login", "", `{"email":"anna@x.de","password":"`+pw+`"}`)
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestDisabledUserTokenRejected(t *testing.T) {
	_, st, h := setup(t)
	u := create(t, st, "anna@x.de", "user")
	token := login(t, h, "anna@x.de", pw)
	if _, err := st.SetEnabled(context.Background(), u.ID, false); err != nil {
		t.Fatal(err)
	}
	if rec := call(h, "GET", "/api/auth/me", token, ""); rec.Code != 401 {
		t.Fatalf("disabled user still allowed: %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/auth/login", "", `{"email":"anna@x.de","password":"`+pw+`"}`); rec.Code != 401 {
		t.Fatalf("disabled user can log in: %d", rec.Code)
	}
}

func TestRoles(t *testing.T) {
	_, st, h := setup(t)
	create(t, st, "user@x.de", "user")
	create(t, st, "admin@x.de", "admin")
	if rec := call(h, "GET", "/admin-only", login(t, h, "user@x.de", pw), ""); rec.Code != 403 {
		t.Fatalf("user on admin route: %d", rec.Code)
	}
	if rec := call(h, "GET", "/admin-only", login(t, h, "admin@x.de", pw), ""); rec.Code != 200 {
		t.Fatalf("admin on admin route: %d", rec.Code)
	}
	if rec := call(h, "GET", "/api/auth/me", "", ""); rec.Code != 401 {
		t.Fatalf("no token: %d", rec.Code)
	}
}

func TestChangePassword(t *testing.T) {
	_, st, h := setup(t)
	create(t, st, "anna@x.de", "user")
	token := login(t, h, "anna@x.de", pw)
	if rec := call(h, "POST", "/api/auth/password", token, `{"currentPassword":"falsch-falsch-1","newPassword":"ein-neues-passwort"}`); rec.Code != 400 {
		t.Fatalf("wrong current: %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/auth/password", token, `{"currentPassword":"`+pw+`","newPassword":"kurz"}`); rec.Code != 400 {
		t.Fatalf("short new: %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/auth/password", token, `{"currentPassword":"`+pw+`","newPassword":"ein-neues-passwort"}`); rec.Code != 204 {
		t.Fatalf("change: %d %s", rec.Code, rec.Body)
	}
	login(t, h, "anna@x.de", "ein-neues-passwort")
}

func TestBootstrapAndDemo(t *testing.T) {
	_, st, h := setup(t)
	ctx := context.Background()
	if err := Bootstrap(ctx, st, "admin@x.de", "admin-password-123", true, 20); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(ctx, st, "admin@x.de", "admin-password-123", true, 20); err != nil {
		t.Fatal("bootstrap must be idempotent:", err)
	}
	login(t, h, "admin@x.de", "admin-password-123")
	demo := login(t, h, DemoEmail, DemoPassword)
	if rec := call(h, "POST", "/api/auth/password", demo, `{"currentPassword":"`+DemoPassword+`","newPassword":"ein-neues-passwort"}`); rec.Code != 403 {
		t.Fatalf("demo password change: %d", rec.Code)
	}
}
