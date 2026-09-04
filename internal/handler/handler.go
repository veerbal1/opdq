package handler

import (
	"net/http"

	"github.com/veerbal/opdq/internal/store"
)

type Handler struct {
	store         *store.Store
	secureCookies bool
}

func NewHandler(s *store.Store, secureCookies bool) *Handler {
	return &Handler{
		store:         s,
		secureCookies: secureCookies,
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.HealthCheck)
	mux.HandleFunc("POST /login", h.LoginHandler)

	mux.HandleFunc("POST /doctors", h.protected(h.CreateDoctorHandler))
	mux.HandleFunc("POST /sessions", h.protected(h.CreateSessionHandler))
	mux.HandleFunc("POST /sessions/{id}/walkins", h.protected(h.CreateWalkInHandler))
	mux.HandleFunc("POST /appointments/{id}/transition", h.protected(h.TransitionAppointmentHandler))

	mux.HandleFunc("POST /logout", h.protected(h.LogoutHandler))

	mux.HandleFunc("GET /me", h.RequireAuth(h.MeHandler))
	mux.HandleFunc("GET /sessions/{id}/queue", h.RequireAuth(h.QueueForSessionHandler))

	return mux
}
