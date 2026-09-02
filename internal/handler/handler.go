package handler

import (
	"net/http"

	"github.com/veerbal/opdq/internal/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{
		store: s,
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", h.HealthCheck)
	mux.HandleFunc("POST /sessions", h.CreateSessionHandler)
	return mux
}
