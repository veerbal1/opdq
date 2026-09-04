package handler

import (
	"encoding/json"
	"net/http"
)

type CreateDoctorRequest struct {
	Name string `json:"name"`
}

type CreateDoctorResponse struct {
	ID       int64 `json:"id"`
	ClinicID int64 `json:"clinic_id"`
}

func (h *Handler) CreateDoctorHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateDoctorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doctor, err := h.store.CreateDoctor(r.Context(), req.Name, sess.ClinicID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateDoctorResponse{ID: doctor.ID, ClinicID: doctor.ClinicID})
}

type DoctorItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) DoctorsHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	doctors, err := h.store.DoctorsForClinic(r.Context(), sess.ClinicID)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]DoctorItem, 0, len(doctors))
	for _, d := range doctors {
		items = append(items, DoctorItem{ID: d.ID, Name: d.Name})
	}

	writeJSON(w, http.StatusOK, items)
}
