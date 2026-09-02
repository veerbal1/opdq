package handler

import (
	"encoding/json"
	"net/http"
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
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// sessionDate := time.Date(req.StartsAt, startsAt.Month(), startsAt.Day(), 0, 0, 0, 0, startsAt.Location())
}
