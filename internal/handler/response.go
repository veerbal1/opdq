package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/veerbal/opdq/internal/domain"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErrorMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeError(w http.ResponseWriter, err error) {
	var status int
	var message string

	switch {
	case errors.Is(err, domain.ErrSessionNotFound), errors.Is(err, domain.ErrAppointmentNotFound):
		status = http.StatusNotFound
		message = err.Error()
	case errors.Is(err, domain.ErrIllegalTransition), errors.Is(err, domain.ErrSessionEnded), errors.Is(err, domain.ErrSessionClosed),
		errors.Is(err, domain.ErrVersionConflict):
		status = http.StatusConflict
		message = err.Error()
	case errors.Is(err, domain.ErrInvalidSessionTimes), errors.Is(err, domain.ErrInvalidCapacity):
		status = http.StatusBadRequest
		message = err.Error()
	default:
		status = http.StatusInternalServerError
		message = "internal server error"
		slog.Error("unhandled error", "error", err)
	}

	writeErrorMessage(w, status, message)
}
