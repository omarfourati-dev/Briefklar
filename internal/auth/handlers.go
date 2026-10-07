package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/respond"
)

type me struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil || in.Email == "" || in.Password == "" {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "E-Mail und Passwort angeben.")
		return
	}
	ip := clientIP(r)
	if ok, retry := s.Throttle.Allow(ip, in.Email); !ok {
		s.OnLogin("throttled")
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		respond.Problem(w, http.StatusTooManyRequests, "Too Many Requests", "Zu viele Fehlversuche. Bitte in 15 Minuten erneut versuchen.")
		return
	}
	u, ok := s.authenticate(r.Context(), in.Email, in.Password)
	if !ok {
		s.Throttle.Fail(ip, in.Email)
		s.OnLogin("failed")
		respond.Problem(w, http.StatusUnauthorized, "Login failed", "E-Mail oder Passwort ist falsch.")
		return
	}
	s.Throttle.Success(ip, in.Email)
	s.OnLogin("success")
	token, exp, err := s.Tokens.Issue(u)
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"token": token, "expiresAt": exp.UTC().Format(time.RFC3339), "user": me{u.Email, u.Name, u.Role},
	})
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	c, _ := ClaimsFrom(r.Context())
	respond.JSON(w, http.StatusOK, me{c.Email, c.Name, c.Role})
}

func (s *Service) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c, _ := ClaimsFrom(r.Context())
	if c.Role == "demo" {
		respond.Problem(w, http.StatusForbidden, "Forbidden", "Das Demo-Passwort kann nicht geändert werden.")
		return
	}
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil || len(in.NewPassword) < 12 || len(in.NewPassword) > 200 {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Das neue Passwort braucht mindestens 12 Zeichen.")
		return
	}
	u, err := s.Users.UserByID(r.Context(), c.UserID)
	if err != nil {
		unauthorized(w)
		return
	}
	if _, ok := s.authenticate(r.Context(), u.Email, in.CurrentPassword); !ok {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Das aktuelle Passwort ist falsch.")
		return
	}
	hash, err := HashPassword(in.NewPassword)
	if err == nil {
		err = s.Users.SetPassword(r.Context(), u.ID, hash)
	}
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clientIP trusts X-Forwarded-For because the container is only reachable through Caddy (no published port).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
