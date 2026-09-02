package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type CreateSessionRequest struct {
	ClinicID int64  `json:"clinic_id"`
	DoctorID int64  `json:"doctor_id"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Capacity int    `json:"capacity"`
}

type CreateSessionResponse struct {
	ID int64 `json:"id"`
}

func (h *Handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid starts_at")
		return
	}

	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid ends_at")
		return
	}

	sessionDate := time.Date(startsAt.Year(), startsAt.Month(), startsAt.Day(), 0, 0, 0, 0, startsAt.Location())

	session, err := h.store.CreateSession(r.Context(), req.ClinicID, req.DoctorID, sessionDate, startsAt, endsAt, req.Capacity)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateSessionResponse{ID: session.ID})
}
