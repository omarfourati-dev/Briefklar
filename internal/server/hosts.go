package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/omarfourati-dev/briefklar/internal/respond"
)

// origins splits the site with AdSense from the app (APP_ORIGIN, e.g. https://app.briefklar.omarfourati.de): the ad
// scripts on the landing page run in their own origin, so they can neither read the session of the app
// (sessionStorage is per origin) nor plant one. Without APP_ORIGIN (local development) everything stays on one origin.
type origins struct {
	app  *url.URL // nil: one origin for everything
	site string   // scheme://host of the landing page, derived from the app host without "app."
}

func newOrigins(appOrigin string) origins {
	if appOrigin == "" {
		return origins{}
	}
	u, err := url.Parse(appOrigin)
	if err != nil || u.Host == "" {
		panic("server: invalid AppOrigin " + appOrigin) // config.Load validates it first
	}
	return origins{app: u, site: u.Scheme + "://" + strings.TrimPrefix(u.Host, "app.")}
}

func (o origins) split() bool { return o.app != nil }

func (o origins) isApp(r *http.Request) bool { return o.split() && strings.EqualFold(r.Host, o.app.Host) }

// routeHosts sends app paths from the site to the app origin and keeps the landing page off the app origin.
// /healthz and /metrics answer on every host: Docker and Prometheus reach the container by its own name.
func (o origins) routeHosts(next http.Handler) http.Handler {
	if !o.split() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/healthz" || p == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		if o.isApp(r) {
			switch {
			case p == "/":
				http.Redirect(w, r, "/app/", http.StatusFound)
			case p == "/robots.txt":
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
			case p == "/app" || strings.HasPrefix(p, "/app/") || p == "/api" || strings.HasPrefix(p, "/api/"):
				next.ServeHTTP(w, r)
			default: // landing files belong to the site
				http.Redirect(w, r, o.site+r.URL.RequestURI(), http.StatusFound)
			}
			return
		}
		switch {
		case p == "/app" || strings.HasPrefix(p, "/app/"):
			target := *o.app
			target.Path = p
			if p == "/app" {
				target.Path = "/app/"
			}
			target.RawQuery = r.URL.RawQuery
			code := http.StatusMovedPermanently
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				code = http.StatusPermanentRedirect // keeps method and body (share target of an older install)
			}
			http.Redirect(w, r, target.String(), code)
		case p == "/api" || strings.HasPrefix(p, "/api/"):
			// no login and no letters where the ads run
			respond.Problem(w, http.StatusNotFound, "Not Found", "")
		default:
			next.ServeHTTP(w, r)
		}
	})
}
