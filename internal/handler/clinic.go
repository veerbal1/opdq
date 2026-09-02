package handler

import (
	"encoding/json"
	"net/http"
)

type CreateClinicRequest struct {
	Name string `json:"name"`
}

type CreateClinicResponse struct {
	ID       int64  `json:"id"`
	PublicID string `json:"public_id"`
}

func (h *Handler) CreateClinicHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateClinicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	clinic, err := h.store.CreateClinic(r.Context(), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateClinicResponse{ID: clinic.ID, PublicID: clinic.PublicID.String()})
}
