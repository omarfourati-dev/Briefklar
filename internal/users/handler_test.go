package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/store"
	"github.com/omarfourati-dev/briefklar/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Start(m)) }

func setup(t *testing.T) (*Handler, *store.Store, store.User) {
	st, err := store.Open(context.Background(), testdb.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	testdb.Reset(t)
	admin, _ := st.CreateUser(context.Background(), "admin@x.de", "h", "Admin", "admin", 20)
	return &Handler{Store: st, Now: time.Now, DefaultLimit: 20}, st, admin
}

func asAdmin(admin store.User, method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	return req.WithContext(auth.WithClaims(req.Context(), auth.Claims{UserID: admin.ID, Role: "admin"}))
}

func TestCreateAndList(t *testing.T) {
	h, _, admin := setup(t)
	rec := httptest.NewRecorder()
	h.Create(rec, asAdmin(admin, "POST", "/api/users", `{"email":"neu@x.de","name":"Neu","password":"ein-langes-passwort","role":"user"}`))
	if rec.Code != 201 || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.Create(rec, asAdmin(admin, "POST", "/api/users", `{"email":"neu@x.de","name":"Neu","password":"ein-langes-passwort","role":"user"}`))
	if rec.Code != 409 {
		t.Fatalf("duplicate: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.List(rec, asAdmin(admin, "GET", "/api/users", ""))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"email":"neu@x.de"`) || !strings.Contains(rec.Body.String(), `"usedToday":0`) {
		t.Fatalf("list: %s", rec.Body)
	}
}

func TestCreateValidation(t *testing.T) {
	h, _, admin := setup(t)
	for _, body := range []string{
		`{"email":"kein-at","name":"A","password":"ein-langes-passwort","role":"user"}`,
		`{"email":"a@x.de","name":"","password":"ein-langes-passwort","role":"user"}`,
		`{"email":"a@x.de","name":"A","password":"kurz","role":"user"}`,
		`{"email":"a@x.de","name":"A","password":"ein-langes-passwort","role":"demo"}`,
		`{"email":"a@x.de","name":"A","password":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","role":"user"}`,
		`{"email":"a@x.de","name":"A","password":"ein-langes-passwort","role":"user","dailyLimit":-1}`,
	} {
		rec := httptest.NewRecorder()
		h.Create(rec, asAdmin(admin, "POST", "/api/users", body))
		if rec.Code != 400 {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}
}

func TestDisableAndSelfProtection(t *testing.T) {
	h, st, admin := setup(t)
	other, _ := st.CreateUser(context.Background(), "o@x.de", "h", "O", "user", 20)

	req := asAdmin(admin, "PATCH", "/api/users/"+other.ID, `{"enabled":false}`)
	req.SetPathValue("id", other.ID)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}

	req = asAdmin(admin, "PATCH", "/api/users/"+admin.ID, `{"enabled":false}`)
	req.SetPathValue("id", admin.ID)
	rec = httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != 409 {
		t.Fatalf("self disable: %d", rec.Code)
	}

	req = asAdmin(admin, "PATCH", "/api/users/unknown", `{"enabled":false}`)
	req.SetPathValue("id", "unknown")
	rec = httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != 404 {
		t.Fatalf("unknown: %d", rec.Code)
	}
}
