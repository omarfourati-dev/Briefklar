package server

import (
	"context"
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
	tok, _ := auth.NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	files := fstest.MapFS{
		"index.html":           {Data: []byte("<h1>Landing</h1>")},
		"robots.txt":           {Data: []byte("User-agent: *")},
		"app/index.html":       {Data: []byte("<app-root></app-root>")},
		"app/main-ABCD1234.js": {Data: []byte("console.log(1)")},
	}
	return New(Deps{Store: pinger{}, Auth: auth.NewService(nil, tok), Metrics: metrics.New(), Static: files,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
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
