package server

import (
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

// The landing page with AdSense and the app live on different origins (APP_ORIGIN): scripts of the ads can never
// read the session of the app (sessionStorage is per origin) nor plant one.
const appOrigin = "https://app.briefklar.test"

func newSplitServer(t *testing.T) http.Handler {
	tok, err := auth.NewTokens([]byte(strings.Repeat("k", 32)), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"index.html":      {Data: []byte("<h1>Landing</h1>")},
		"llms.txt":        {Data: []byte("# Briefklar")},
		"robots.txt":      {Data: []byte("User-agent: *\nAllow: /")},
		"app/index.html":  {Data: []byte("<app-root></app-root>")},
		"app/sw.js":       {Data: []byte("self.addEventListener('fetch', () => {})")},
		"app/favicon.svg": {Data: []byte("<svg/>")},
	}
	return New(Deps{Store: pinger{}, Auth: auth.NewService(nil, tok), Metrics: metrics.New(), Static: files,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), AppOrigin: appOrigin})
}

func on(h http.Handler, method, host, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader("{}"))
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTheSiteSendsTheAppToItsOwnOrigin(t *testing.T) {
	h := newSplitServer(t)
	cases := []struct {
		method, target string
		code           int
		location       string
	}{
		{"GET", "/app/neu?von=start", http.StatusMovedPermanently, appOrigin + "/app/neu?von=start"},
		{"GET", "/app/login", http.StatusMovedPermanently, appOrigin + "/app/login"},
		{"GET", "/app", http.StatusMovedPermanently, appOrigin + "/app/"},
		{"GET", "/app/", http.StatusMovedPermanently, appOrigin + "/app/"},
		// a share from an app installed before the move keeps its method and body
		{"POST", "/app/share-target", http.StatusPermanentRedirect, appOrigin + "/app/share-target"},
	}
	for _, c := range cases {
		rec := on(h, c.method, "briefklar.test", c.target)
		if rec.Code != c.code || rec.Header().Get("Location") != c.location {
			t.Errorf("%s %s: %d %q", c.method, c.target, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestNoLoginAndNoAPIOnTheSiteWithAds(t *testing.T) {
	h := newSplitServer(t)
	for _, c := range []struct{ method, target string }{
		{"POST", "/api/auth/login"}, {"GET", "/api/auth/me"}, {"POST", "/api/letters/preview"}, {"GET", "/api/users"},
	} {
		if rec := on(h, c.method, "briefklar.test", c.target); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s on the site: %d, want 404", c.method, c.target, rec.Code)
		}
	}
	// the app origin answers the API (401 without a session – not 404)
	if rec := on(h, "GET", "app.briefklar.test", "/api/auth/me"); rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/auth/me on the app: %d", rec.Code)
	}
}

func TestTheSiteKeepsLandingHealthAndMetrics(t *testing.T) {
	h := newSplitServer(t)
	landing := on(h, "GET", "briefklar.test", "/")
	if landing.Code != 200 || !strings.Contains(landing.Body.String(), "Landing") {
		t.Fatalf("landing: %d %q", landing.Code, landing.Body.String())
	}
	if csp := landing.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "https:") {
		t.Errorf("landing keeps the AdSense CSP: %q", csp)
	}
	if rec := on(h, "GET", "briefklar.test", "/llms.txt"); rec.Code != 200 {
		t.Errorf("/llms.txt: %d", rec.Code)
	}
	// Docker health check and Prometheus reach the container by its own name, not by a public host
	for _, host := range []string{"briefklar.test", "briefklar:8080", "localhost:8080"} {
		if rec := on(h, "GET", host, "/healthz"); rec.Code != 200 {
			t.Errorf("/healthz via %s: %d", host, rec.Code)
		}
		if rec := on(h, "GET", host, "/metrics"); rec.Code != 200 {
			t.Errorf("/metrics via %s: %d", host, rec.Code)
		}
	}
}

func TestTheAppOriginShowsOnlyTheAppWithTheStrictCSP(t *testing.T) {
	h := newSplitServer(t)
	root := on(h, "GET", "app.briefklar.test", "/")
	if root.Code != http.StatusFound || root.Header().Get("Location") != "/app/" {
		t.Errorf("/ on the app: %d %q", root.Code, root.Header().Get("Location"))
	}
	app := on(h, "GET", "APP.briefklar.test", "/app/neu") // host names are case-insensitive
	if app.Code != 200 || !strings.Contains(app.Body.String(), "app-root") {
		t.Errorf("/app/neu on the app: %d %q", app.Code, app.Body.String())
	}
	for _, target := range []string{"/", "/app/neu", "/app/sw.js", "/api/auth/me"} {
		csp := on(h, "GET", "app.briefklar.test", target).Header().Get("Content-Security-Policy")
		if csp != strictCSP {
			t.Errorf("%s on the app: CSP %q", target, csp)
		}
	}
	// the landing page belongs to the site: no copy on the app origin (duplicate content), robots stay out
	if rec := on(h, "GET", "app.briefklar.test", "/llms.txt"); rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://briefklar.test/llms.txt" {
		t.Errorf("/llms.txt on the app: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	robots := on(h, "GET", "app.briefklar.test", "/robots.txt")
	if robots.Code != 200 || !strings.Contains(robots.Body.String(), "Disallow: /") {
		t.Errorf("robots.txt on the app: %d %q", robots.Code, robots.Body.String())
	}
}

func TestWithoutAppOriginEverythingStaysOnOneOrigin(t *testing.T) {
	h := newServer(t) // local development without APP_ORIGIN
	if rec := on(h, "GET", "localhost:8080", "/app/neu"); rec.Code != 200 {
		t.Errorf("/app/neu: %d", rec.Code)
	}
	if rec := on(h, "GET", "localhost:8080", "/api/auth/me"); rec.Code != http.StatusUnauthorized {
		t.Errorf("/api/auth/me: %d", rec.Code)
	}
}
