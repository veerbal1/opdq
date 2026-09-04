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
	mux.HandleFunc("GET /api/board/{id}", h.BoardHandler)
	mux.HandleFunc("GET /api/q/{public_id}", h.PatientHandler)
	mux.HandleFunc("GET /api/q/{public_id}/qr.png", h.PatientQRHandler)

	mux.HandleFunc("POST /api/doctors", h.protected(h.CreateDoctorHandler))
	mux.HandleFunc("POST /api/sessions", h.protected(h.CreateSessionHandler))
	mux.HandleFunc("POST /api/sessions/{id}/walkins", h.protected(h.CreateWalkInHandler))
	mux.HandleFunc("POST /api/appointments/{id}/transition", h.protected(h.TransitionAppointmentHandler))

	mux.HandleFunc("POST /api/sessions/{id}/delay", h.protected(h.SetDelayHandler))
	mux.HandleFunc("POST /api/sessions/{id}/close", h.protected(h.CloseSessionHandler))

	mux.HandleFunc("POST /api/logout", h.protected(h.LogoutHandler))

	mux.HandleFunc("GET /api/me", h.RequireAuth(h.MeHandler))
	mux.HandleFunc("GET /api/sessions", h.RequireAuth(h.SessionsForDateHandler))
	mux.HandleFunc("GET /api/doctors", h.RequireAuth(h.DoctorsHandler))
	mux.HandleFunc("GET /api/sessions/{id}/queue", h.RequireAuth(h.QueueForSessionHandler))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeErrorMessage(w, http.StatusNotFound, "not found")
	})

	mux.Handle("/", spaHandler())

	return mux
}
