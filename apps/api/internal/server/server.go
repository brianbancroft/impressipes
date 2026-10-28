package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/brianbancroft/impressipies/apps/api/internal/family"
)

const sessionCookie = "impressipies_session"

type server struct {
	store *family.Store
}

// New returns the API's root handler, with a fresh store and a bootstrap admin invite.
func New() http.Handler {
	store := family.NewStore()
	if inv, err := store.CreateInvite(""); err == nil {
		log.Printf("first run: open /invite/%s to create the admin account", inv.Token)
	}
	return NewWithStore(store)
}

func NewWithStore(store *family.Store) http.Handler {
	s := &server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /me", s.me)
	mux.HandleFunc("GET /members", s.admin(s.listMembers))
	mux.HandleFunc("POST /members/{id}/disable", s.admin(s.setStatus(family.StatusDisabled)))
	mux.HandleFunc("POST /members/{id}/enable", s.admin(s.setStatus(family.StatusActive)))
	mux.HandleFunc("POST /invites", s.admin(s.createInvite))
	mux.HandleFunc("GET /invites/{token}", s.getInvite)
	mux.HandleFunc("POST /invites/{token}/accept", s.acceptInvite)
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// current resolves the request's session. A disabled member gets 403 so the
// web app can show the disabled state rather than a sign-in prompt.
func (s *server) current(w http.ResponseWriter, r *http.Request) *family.Member {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return nil
	}
	m, err := s.store.Session(c.Value)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthenticated")
		return nil
	}
	if m.Status == family.StatusDisabled {
		writeErr(w, http.StatusForbidden, "disabled")
		return nil
	}
	return m
}

func (s *server) admin(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := s.current(w, r)
		if m == nil {
			return
		}
		if m.Role != family.RoleAdmin {
			writeErr(w, http.StatusForbidden, "admin only")
			return
		}
		next(w, r)
	}
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	if m := s.current(w, r); m != nil {
		writeJSON(w, http.StatusOK, m)
	}
}

func (s *server) listMembers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Members())
}

func (s *server) setStatus(status string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.store.SetStatus(r.PathValue("id"), status); err != nil {
			writeErr(w, http.StatusNotFound, "member not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *server) createInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MemberID string `json:"memberId"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	inv, err := s.store.CreateInvite(body.MemberID)
	switch {
	case errors.Is(err, family.ErrNotFound):
		writeErr(w, http.StatusNotFound, "member not found")
	case errors.Is(err, family.ErrDisabled):
		writeErr(w, http.StatusConflict, "member is disabled; enable them before re-inviting")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "could not create invite")
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"token": inv.Token, "path": "/invite/" + inv.Token})
	}
}

func (s *server) getInvite(w http.ResponseWriter, r *http.Request) {
	inv, m, err := s.store.Invite(r.PathValue("token"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"state": "invalid"})
		return
	}
	state := "valid"
	switch {
	case inv.Used:
		state = "used"
	case s.expired(inv):
		state = "expired"
	case m != nil && m.Status == family.StatusDisabled:
		state = "disabled"
	}
	resp := map[string]any{"state": state, "reinvite": m != nil}
	if m != nil {
		resp["name"], resp["color"] = m.Name, m.Color
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) expired(inv *family.Invite) bool {
	return !inv.ExpiresAt.After(timeNow())
}

func (s *server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		Color        string `json:"color"`
		CredentialID string `json:"credentialId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || body.Color == "" || body.CredentialID == "" {
		writeErr(w, http.StatusBadRequest, "name, color and credentialId are required")
		return
	}
	m, sess, err := s.store.Accept(r.PathValue("token"), body.Name, body.Color, body.CredentialID)
	switch {
	case errors.Is(err, family.ErrNotFound):
		writeErr(w, http.StatusNotFound, "invite not found")
	case errors.Is(err, family.ErrInviteUsed), errors.Is(err, family.ErrExpired):
		writeErr(w, http.StatusGone, err.Error())
	case errors.Is(err, family.ErrDisabled):
		writeErr(w, http.StatusForbidden, "disabled")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "could not accept invite")
	default:
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookie, Value: sess, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 365,
		})
		writeJSON(w, http.StatusCreated, m)
	}
}

var timeNow = func() time.Time { return time.Now() }
