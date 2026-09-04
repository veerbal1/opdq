package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/veerbal/opdq/internal/domain"
)

type CreateSessionRequest struct {
	DoctorID int64  `json:"doctor_id"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Capacity int    `json:"capacity"`
}

type CreateSessionResponse struct {
	ID int64 `json:"id"`
}

func (h *Handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	session, err := h.store.CreateSession(r.Context(), sess.ClinicID, req.DoctorID, sessionDate, startsAt, endsAt, req.Capacity)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, CreateSessionResponse{ID: session.ID})
}

type SessionItem struct {
	ID            int64  `json:"id"`
	DoctorID      int64  `json:"doctor_id"`
	DoctorName    string `json:"doctor_name"`
	StartsAt      string `json:"starts_at"`
	EndsAt        string `json:"ends_at"`
	Capacity      int    `json:"capacity"`
	DelayMin      int    `json:"delay_min"`
	AvgConsultSec int    `json:"avg_consult_sec"`
	Status        string `json:"status"`
	Version       int    `json:"version"`
}

func (h *Handler) SessionsForDateHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	date := time.Now()
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeErrorMessage(w, http.StatusBadRequest, "invalid date, expected YYYY-MM-DD")
			return
		}
		date = parsed
	}
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	sessions, err := h.store.SessionsForDate(r.Context(), sess.ClinicID, day)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]SessionItem, 0, len(sessions))
	for _, x := range sessions {
		items = append(items, SessionItem{
			ID:            x.ID,
			DoctorID:      x.DoctorID,
			DoctorName:    x.DoctorName,
			StartsAt:      x.StartsAt.Format(time.RFC3339),
			EndsAt:        x.EndsAt.Format(time.RFC3339),
			Capacity:      x.Capacity,
			DelayMin:      x.DelayMin,
			AvgConsultSec: x.AvgConsultSec,
			Status:        string(x.Status),
			Version:       x.Version,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

type SetDelayRequest struct {
	DelayMin int `json:"delay_min"`
	Version  int `json:"version"`
}

type CloseSessionRequest struct {
	Version int `json:"version"`
}

func (h *Handler) SetDelayHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid session id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req SetDelayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DelayMin < 0 {
		writeErrorMessage(w, http.StatusBadRequest, "delay_min cannot be negative")
		return
	}

	updated, err := h.store.SetSessionDelay(r.Context(), sess.ClinicID, sessionID, req.DelayMin, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionItemFrom(updated, ""))
}

func (h *Handler) CloseSessionHandler(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFrom(r.Context())
	if !ok {
		writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid session id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CloseSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.store.CloseSession(r.Context(), sess.ClinicID, sessionID, req.Version)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, sessionItemFrom(updated, ""))
}

func sessionItemFrom(x domain.Session, doctorName string) SessionItem {
	return SessionItem{
		ID:            x.ID,
		DoctorID:      x.DoctorID,
		DoctorName:    doctorName,
		StartsAt:      x.StartsAt.Format(time.RFC3339),
		EndsAt:        x.EndsAt.Format(time.RFC3339),
		Capacity:      x.Capacity,
		DelayMin:      x.DelayMin,
		AvgConsultSec: x.AvgConsultSec,
		Status:        string(x.Status),
		Version:       x.Version,
	}
}
