package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/metrics"
)

type pinger struct{ err error }

func (p pinger) Ping(context.Context) error { return p.err }

func newServer(t *testing.T) http.Handler {
	return newServerWith(t, pinger{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newServerWith(t *testing.T, p pinger, log *slog.Logger) http.Handler {
	tok, err := auth.NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"index.html":               {Data: []byte("<h1>Landing</h1>")},
		"robots.txt":               {Data: []byte("User-agent: *")},
		"app/index.html":           {Data: []byte("<app-root></app-root>")},
		"app/main-ABCD1234.js":     {Data: []byte("console.log(1)")},
		"app/chunk-DOVpZU-H.js":    {Data: []byte("console.log(2)")},
		"app/sw.js":                {Data: []byte("self.addEventListener('fetch', () => {})")},
		"app/manifest.webmanifest": {Data: []byte(`{"name":"Briefklar"}`)},
	}
	return New(Deps{Store: p, Auth: auth.NewService(nil, tok), Metrics: metrics.New(), Static: files, Log: log})
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestStaticRoutes(t *testing.T) {
	h := newServer(t)
	cases := []struct {
		path, contains, cache string
		code                  int
	}{
		{"/", "Landing", "no-cache", 200},
		{"/robots.txt", "User-agent", "", 200},
		{"/app/", "app-root", "no-cache", 200},
		{"/app/neu", "app-root", "no-cache", 200}, // Angular route, served by index.html
		{"/app/main-ABCD1234.js", "console", "immutable", 200},
		{"/app/chunk-DOVpZU-H.js", "console", "immutable", 200}, // newer Angular: mixed case, - and _
	}
	for _, c := range cases {
		rec := get(h, c.path)
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.contains) || !strings.Contains(rec.Header().Get("Cache-Control"), c.cache) {
			t.Errorf("%s: %d %q cache=%q", c.path, rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
	}
	if rec := get(h, "/app"); rec.Code != http.StatusMovedPermanently {
		t.Errorf("/app: %d", rec.Code)
	}
	if rec := get(h, "/app/missing.js"); rec.Code != 404 {
		t.Errorf("missing asset: %d", rec.Code)
	}
}

// The service worker and the manifest must never be cached by the browser, or updates would hang for days.
func TestPWAFiles(t *testing.T) {
	h := newServer(t)
	sw := get(h, "/app/sw.js")
	if sw.Code != 200 || sw.Header().Get("Cache-Control") != "no-cache" || !strings.Contains(sw.Header().Get("Content-Type"), "javascript") {
		t.Errorf("sw.js: %d cache=%q type=%q", sw.Code, sw.Header().Get("Cache-Control"), sw.Header().Get("Content-Type"))
	}
	m := get(h, "/app/manifest.webmanifest")
	if m.Code != 200 || m.Header().Get("Cache-Control") != "no-cache" || m.Header().Get("Content-Type") != "application/manifest+json" {
		t.Errorf("manifest: %d cache=%q type=%q", m.Code, m.Header().Get("Cache-Control"), m.Header().Get("Content-Type"))
	}
}

// Without a service worker (first visit, old browser) a share lands on the server: send the user to the app.
func TestShareTargetWithoutServiceWorker(t *testing.T) {
	rec := httptest.NewRecorder()
	newServer(t).ServeHTTP(rec, httptest.NewRequest("POST", "/app/share-target", strings.NewReader("x")))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/neu" {
		t.Errorf("share-target: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestSecurityHeadersAndAPI(t *testing.T) {
	h := newServer(t)
	rec := get(h, "/")
	for _, hdr := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Referrer-Policy"} {
		if rec.Header().Get(hdr) == "" {
			t.Errorf("missing %s", hdr)
		}
	}
	if rec := get(h, "/api/nope"); rec.Code != 404 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("unknown api: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(h, "/api/auth/me"); rec.Code != 401 {
		t.Errorf("me without token: %d", rec.Code)
	}
	if rec := get(h, "/healthz"); rec.Code != 200 {
		t.Errorf("healthz: %d", rec.Code)
	}
	if rec := get(h, "/metrics"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Errorf("metrics: %d", rec.Code)
	}
}

func TestSecurityHeaderValues(t *testing.T) {
	rec := get(newServer(t), "/")
	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; " +
			"frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
		"Permissions-Policy":     "camera=(self), microphone=(), geolocation=()",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestAPIWithoutSlashIsProblem404(t *testing.T) {
	if rec := get(newServer(t), "/api"); rec.Code != 404 || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("/api: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestHealthzDatabaseDown(t *testing.T) {
	h := newServerWith(t, pinger{err: errors.New("down")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if rec := get(h, "/healthz"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz: %d", rec.Code)
	}
}

func TestRequestLogHasNoQueryString(t *testing.T) {
	var buf bytes.Buffer
	h := newServerWith(t, pinger{}, slog.New(slog.NewTextHandler(&buf, nil)))
	get(h, "/api/nope?token=secret")
	get(h, "/app/neu?x=geheim")
	out := buf.String()
	if !strings.Contains(out, "/api/nope") || !strings.Contains(out, "/app/neu") {
		t.Fatalf("requests not logged: %q", out)
	}
	for _, bad := range []string{"secret", "geheim", "token="} {
		if strings.Contains(out, bad) {
			t.Errorf("log contains %q: %s", bad, out)
		}
	}
}
