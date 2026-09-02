package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type CreateWalkInRequest struct {
	PatientName string `json:"patient_name"`
	Contact     string `json:"contact"`
	Priority    int    `json:"priority"`
}

type CreateWalkInResponse struct {
	ID      int64  `json:"id"`
	TokenNo int    `json:"token_no"`
	State   string `json:"state"`
}

func (h *Handler) CreateWalkInHandler(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid session id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateWalkInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	appointment, err := h.store.CreateWalkIn(r.Context(), sessionID, req.PatientName, req.Contact, req.Priority)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateWalkInResponse{
		ID:      appointment.ID,
		TokenNo: appointment.TokenNo,
		State:   string(appointment.State),
	})
}
