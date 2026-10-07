package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/omarfourati-dev/briefklar/internal/respond"
	"github.com/omarfourati-dev/briefklar/internal/store"
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
	if ok, retry := s.Throttle.Reserve(ip, in.Email); !ok {
		s.OnLogin("throttled")
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		respond.Problem(w, http.StatusTooManyRequests, "Too Many Requests", "Zu viele Fehlversuche. Bitte in 15 Minuten erneut versuchen.")
		return
	}
	u, ok, err := s.authenticate(r.Context(), in.Email, in.Password)
	if err != nil {
		s.Throttle.Release(ip, in.Email)
		s.Log.Error("login lookup failed", "error", err)
		unavailable(w)
		return
	}
	if !ok {
		s.OnLogin("failed") // the reservation made by Reserve stays and counts as the failure
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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Ungültige Anfrage.")
		return
	}
	if err := ValidatePassword(in.NewPassword); err != nil {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}
	u, err := s.Users.UserByID(r.Context(), c.UserID)
	if errors.Is(err, store.ErrNotFound) {
		unauthorized(w)
		return
	}
	if err != nil {
		s.Log.Error("password change lookup failed", "error", err)
		unavailable(w)
		return
	}
	_, ok, err := s.authenticate(r.Context(), u.Email, in.CurrentPassword)
	if err != nil {
		s.Log.Error("password change lookup failed", "error", err)
		unavailable(w)
		return
	}
	if !ok {
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
