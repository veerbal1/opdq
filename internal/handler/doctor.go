package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type CreateDoctorRequest struct {
	Name string `json:"name"`
}

type CreateDoctorResponse struct {
	ID       int64 `json:"id"`
	ClinicID int64 `json:"clinic_id"`
}

func (h *Handler) CreateDoctorHandler(w http.ResponseWriter, r *http.Request) {
	clinicID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid clinic id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateDoctorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doctor, err := h.store.CreateDoctor(r.Context(), req.Name, clinicID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateDoctorResponse{ID: doctor.ID, ClinicID: doctor.ClinicID})
}
