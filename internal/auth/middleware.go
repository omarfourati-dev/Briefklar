package auth

import (
	"net/http"
	"strings"

	"github.com/omarfourati-dev/briefklar/internal/respond"
)

// Require accepts a valid bearer token of an account that is still enabled.
func (s *Service) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			unauthorized(w)
			return
		}
		c, err := s.Tokens.Parse(token)
		if err != nil {
			unauthorized(w)
			return
		}
		active, err := s.Users.IsActive(r.Context(), c.UserID)
		if err != nil {
			respond.Problem(w, http.StatusServiceUnavailable, "Service Unavailable", "Bitte später erneut versuchen.")
			return
		}
		if !active {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), c)))
	})
}

func (s *Service) RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := ClaimsFrom(r.Context()); !ok || c.Role != role {
			respond.Problem(w, http.StatusForbidden, "Forbidden", "Dafür fehlen die Rechte.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	respond.Problem(w, http.StatusUnauthorized, "Unauthorized", "Bitte anmelden.")
}

func unavailable(w http.ResponseWriter) {
	respond.Problem(w, http.StatusServiceUnavailable, "Service Unavailable", "Bitte später erneut versuchen.")
}
