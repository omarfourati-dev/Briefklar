package users

import (
	"context"
	"encoding/json"
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
	admin, err := st.CreateUser(context.Background(), "admin@x.de", "h", "Admin", "admin", 20)
	if err != nil {
		t.Fatal(err)
	}
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
	if rec.Code != 201 || leaksSecret(t, rec.Body.Bytes()) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.Create(rec, asAdmin(admin, "POST", "/api/users", `{"email":"neu@x.de","name":"Neu","password":"ein-langes-passwort","role":"user"}`))
	if rec.Code != 409 {
		t.Fatalf("duplicate: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.List(rec, asAdmin(admin, "GET", "/api/users", ""))
	if leaksSecret(t, rec.Body.Bytes()) {
		t.Fatalf("list leak: %s", rec.Body)
	}
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"email":"neu@x.de"`) || !strings.Contains(rec.Body.String(), `"usedToday":0`) {
		t.Fatalf("list: %s", rec.Body)
	}
}

func TestCreateValidation(t *testing.T) {
	h, _, admin := setup(t)
	for _, body := range []string{
		`{"email":"kein-at","name":"A","password":"ein-langes-passwort","role":"user"}`,
		`{"email":"X <a@x.de>","name":"A","password":"ein-langes-passwort","role":"user"}`,
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
	other, err := st.CreateUser(context.Background(), "o@x.de", "h", "O", "user", 20)
	if err != nil {
		t.Fatal(err)
	}

	req := asAdmin(admin, "PATCH", "/api/users/"+other.ID, `{"enabled":false}`)
	req.SetPathValue("id", other.ID)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":false`) || leaksSecret(t, rec.Body.Bytes()) {
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

// leaksSecret reports whether any JSON key (at any depth, also inside arrays) mentions "hash" or "password".
func leaksSecret(t *testing.T, body []byte) bool {
	t.Helper()
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("keine gültige JSON-Antwort: %s", body)
	}
	var walk func(any) bool
	walk = func(x any) bool {
		switch n := x.(type) {
		case map[string]any:
			for k, val := range n {
				lk := strings.ToLower(k)
				if strings.Contains(lk, "hash") || strings.Contains(lk, "password") || walk(val) {
					return true
				}
			}
		case []any:
			for _, e := range n {
				if walk(e) {
					return true
				}
			}
		}
		return false
	}
	return walk(v)
}

func TestUpdateReenableAndInvalidBody(t *testing.T) {
	h, st, admin := setup(t)
	other, err := st.CreateUser(context.Background(), "o@x.de", "h", "O", "user", 20)
	if err != nil {
		t.Fatal(err)
	}
	patch := func(body string) *httptest.ResponseRecorder {
		req := asAdmin(admin, "PATCH", "/api/users/"+other.ID, body)
		req.SetPathValue("id", other.ID)
		rec := httptest.NewRecorder()
		h.Update(rec, req)
		return rec
	}
	if rec := patch(`{"enabled":false}`); rec.Code != 200 || leaksSecret(t, rec.Body.Bytes()) {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	rec := patch(`{"enabled":true}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"enabled":true`) || leaksSecret(t, rec.Body.Bytes()) {
		t.Fatalf("re-enable: %d %s", rec.Code, rec.Body)
	}
	for _, body := range []string{`{}`, `kein json`, `{"enabled":"ja"}`} {
		if rec := patch(body); rec.Code != 400 {
			t.Errorf("%s: %d", body, rec.Code)
		}
	}
}

func TestUpdateWithoutClaims(t *testing.T) {
	h, _, _ := setup(t)
	req := httptest.NewRequest("PATCH", "/api/users/x", strings.NewReader(`{"enabled":false}`))
	req.SetPathValue("id", "x")
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != 401 {
		t.Fatalf("ohne Claims: %d", rec.Code)
	}
}

func TestNameLengthCountsCharacters(t *testing.T) {
	h, _, admin := setup(t)
	name := strings.Repeat("ä", 100) // 200 bytes, 100 characters
	rec := httptest.NewRecorder()
	h.Create(rec, asAdmin(admin, "POST", "/api/users", `{"email":"u@x.de","name":"`+name+`","password":"ein-langes-passwort","role":"user"}`))
	if rec.Code != 201 {
		t.Fatalf("100 Zeichen: %d %s", rec.Code, rec.Body)
	}
}
