// Package users is the admin API: list, create, disable and re-enable accounts.
package users

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/omarfourati-dev/briefklar/internal/auth"
	"github.com/omarfourati-dev/briefklar/internal/respond"
	"github.com/omarfourati-dev/briefklar/internal/store"
)

type Store interface {
	ListUsers(ctx context.Context, day time.Time) ([]store.User, error)
	CreateUser(ctx context.Context, email, hash, name, role string, dailyLimit int) (store.User, error)
	SetEnabled(ctx context.Context, id string, enabled bool) (store.User, error)
}

type Handler struct {
	Store        Store
	Now          func() time.Time
	DefaultLimit int
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListUsers(r.Context(), h.Now())
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if list == nil {
		list = []store.User{}
	}
	respond.JSON(w, http.StatusOK, list)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		DailyLimit *int   `json:"dailyLimit"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Ungültige Eingabe.")
		return
	}
	in.Email, in.Name = strings.TrimSpace(in.Email), strings.TrimSpace(in.Name)
	limit := h.DefaultLimit
	if in.DailyLimit != nil {
		limit = *in.DailyLimit
	}
	addr, mailErr := mail.ParseAddress(in.Email)
	pwErr := auth.ValidatePassword(in.Password)
	switch {
	case mailErr != nil || addr.Address != in.Email || len(in.Email) > 200:
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Bitte eine gültige E-Mail-Adresse angeben.")
		return
	case in.Name == "" || utf8.RuneCountInString(in.Name) > 100:
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Bitte einen Namen angeben (höchstens 100 Zeichen).")
		return
	case pwErr != nil:
		respond.Problem(w, http.StatusBadRequest, "Bad Request", pwErr.Error())
		return
	case in.Role != "user" && in.Role != "admin":
		respond.Problem(w, http.StatusBadRequest, "Bad Request", `Rolle muss "user" oder "admin" sein.`)
		return
	case limit < 0 || limit > 1000:
		respond.Problem(w, http.StatusBadRequest, "Bad Request", "Das Tageslimit muss zwischen 0 und 1000 liegen.")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	u, err := h.Store.CreateUser(r.Context(), in.Email, hash, in.Name, in.Role, limit)
	if errors.Is(err, store.ErrEmailTaken) {
		respond.Problem(w, http.StatusConflict, "Conflict", "Diese E-Mail-Adresse hat schon ein Konto.")
		return
	}
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	respond.JSON(w, http.StatusCreated, u)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil || in.Enabled == nil {
		respond.Problem(w, http.StatusBadRequest, "Bad Request", `Erwartet: {"enabled": true|false}`)
		return
	}
	id := r.PathValue("id")
	c, ok := auth.ClaimsFrom(r.Context())
	if !ok {
		respond.Problem(w, http.StatusUnauthorized, "Unauthorized", "Bitte anmelden.")
		return
	}
	if id == c.UserID && !*in.Enabled {
		respond.Problem(w, http.StatusConflict, "Conflict", "Das eigene Konto kann nicht gesperrt werden.")
		return
	}
	u, err := h.Store.SetEnabled(r.Context(), id, *in.Enabled)
	if errors.Is(err, store.ErrNotFound) {
		respond.Problem(w, http.StatusNotFound, "Not Found", "Konto nicht gefunden.")
		return
	}
	if err != nil {
		respond.Problem(w, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	respond.JSON(w, http.StatusOK, u)
}
