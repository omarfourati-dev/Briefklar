// Package server wires all handlers into one http.Handler.
package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/letters"
	"github.com/omarfourati-dev/briefklar/internal/metrics"
	"github.com/omarfourati-dev/briefklar/internal/respond"
	"github.com/omarfourati-dev/briefklar/internal/users"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Store   Pinger
	Auth    *auth.Service
	Letters *letters.Handler
	Users   *users.Handler
	Metrics *metrics.Metrics
	Static  fs.FS
	Log     *slog.Logger
}

func New(d Deps) http.Handler {
	mux := http.NewServeMux()
	authed := func(h http.HandlerFunc) http.Handler { return d.Auth.Require(h) }
	admin := func(h http.HandlerFunc) http.Handler { return d.Auth.Require(d.Auth.RequireRole("admin", h)) }

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.Store.Ping(ctx); err != nil {
			respond.Problem(w, http.StatusServiceUnavailable, "Service Unavailable", "Datenbank nicht erreichbar.")
			return
		}
		respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", d.Metrics.Handler()) // Caddy answers /metrics with 404 publicly

	mux.HandleFunc("POST /api/auth/login", d.Auth.Login)
	mux.Handle("GET /api/auth/me", authed(d.Auth.Me))
	mux.Handle("POST /api/auth/password", authed(d.Auth.ChangePassword))
	if d.Letters != nil {
		mux.Handle("POST /api/letters/preview", authed(d.Letters.Preview))
		mux.Handle("POST /api/letters/explain", authed(d.Letters.Explain))
	}
	if d.Users != nil {
		mux.Handle("GET /api/users", admin(d.Users.List))
		mux.Handle("POST /api/users", admin(d.Users.Create))
		mux.Handle("PATCH /api/users/{id}", admin(d.Users.Update))
	}
	apiNotFound := func(w http.ResponseWriter, r *http.Request) {
		respond.Problem(w, http.StatusNotFound, "Not Found", "")
	}
	mux.HandleFunc("/api/", apiNotFound)
	mux.HandleFunc("/api", apiNotFound) // without it the mux redirects /api to /api/
	// Share target without a service worker (first visit, old browser): the shared file is lost, open the app.
	mux.HandleFunc("POST /app/share-target", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/neu", http.StatusSeeOther)
	})
	mux.Handle("/", staticHandler(d.Static))

	return securityHeaders(logRequests(d.Log, mux))
}

// The app, the API and every other path: only our own code, no referrer leaves the site.
const strictCSP = "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// The public landing page shows Google AdSense (auto ads plus Google's consent message). AdSense loads scripts, frames and
// pixels from changing Google hosts, so this static page without user data allows HTTPS sources. Letters are only ever
// handled under /app/, which keeps strictCSP. AdSense needs the page URL as referrer to serve ads for this site.
const landingCSP = "default-src 'self'; script-src 'self' 'unsafe-inline' https:; img-src 'self' data: blob: https:; " +
	"frame-src https:; connect-src 'self' https:; style-src 'self' 'unsafe-inline'; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

func isLanding(path string) bool { return path == "/" || path == "/index.html" }

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if isLanding(r.URL.Path) {
			h.Set("Content-Security-Policy", landingCSP)
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		} else {
			h.Set("Content-Security-Policy", strictCSP)
			h.Set("Referrer-Policy", "no-referrer")
		}
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }

// Unwrap lets http.ResponseController reach the real writer (Flush, deadlines).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests logs method, path, status and duration – never bodies or query strings.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path != "/healthz" && r.URL.Path != "/metrics" {
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "ms", time.Since(start).Milliseconds())
		}
	})
}
