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
	mux.HandleFunc("POST /sessions", h.CreateSessionHandler)
	mux.HandleFunc("POST /clinics", h.CreateClinicHandler)
	mux.HandleFunc("POST /clinics/{id}/doctors", h.CreateDoctorHandler)
	mux.HandleFunc("POST /sessions/{id}/walkins", h.CreateWalkInHandler)
	mux.HandleFunc("POST /appointments/{id}/transition", h.TransitionAppointmentHandler)
	mux.HandleFunc("GET /sessions/{id}/queue", h.QueueForSessionHandler)
	mux.HandleFunc("POST /login", h.LoginHandler)

	return mux
}
