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
	mux.HandleFunc("POST /api/login", h.LoginHandler)

	mux.HandleFunc("POST /api/doctors", h.protected(h.CreateDoctorHandler))
	mux.HandleFunc("POST /api/sessions", h.protected(h.CreateSessionHandler))
	mux.HandleFunc("POST /api/sessions/{id}/walkins", h.protected(h.CreateWalkInHandler))
	mux.HandleFunc("POST /api/appointments/{id}/transition", h.protected(h.TransitionAppointmentHandler))

	mux.HandleFunc("POST /api/logout", h.protected(h.LogoutHandler))

	mux.HandleFunc("GET /api/me", h.RequireAuth(h.MeHandler))
	mux.HandleFunc("GET /api/sessions/{id}/queue", h.RequireAuth(h.QueueForSessionHandler))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErrorMessage(w, http.StatusNotFound, "not found")
	})

	mux.Handle("/", spaHandler())

	return mux
}
