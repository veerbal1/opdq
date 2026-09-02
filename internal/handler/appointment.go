package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/veerbal/opdq/internal/domain"
)

type TransitionAppointmentRequest struct {
	To string `json:"to"`
}

type TransitionAppointmentResponse struct {
	ID    int64  `json:"id"`
	State string `json:"state"`
}

func (h *Handler) TransitionAppointmentHandler(w http.ResponseWriter, r *http.Request) {
	appointmentID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid appointment id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req TransitionAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	appointment, err := h.store.TransitionAppointment(r.Context(), appointmentID, domain.State(req.To))
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, TransitionAppointmentResponse{ID: appointment.ID, State: string(appointment.State)})
}
