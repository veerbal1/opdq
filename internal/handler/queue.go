package handler

import (
	"net/http"
	"strconv"
)

type QueueItem struct {
	TokenNo     int    `json:"token_no"`
	PatientName string `json:"patient_name"`
	Priority    int    `json:"priority"`
}

func (h *Handler) QueueForSessionHandler(w http.ResponseWriter, r *http.Request) {
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

	appointments, err := h.store.QueueForSession(r.Context(), sess.ClinicID, sessionID)
	if err != nil {
		writeError(w, err)
		return
	}

	queue := make([]QueueItem, 0, len(appointments))
	for _, a := range appointments {
		queue = append(queue, QueueItem{TokenNo: a.TokenNo, PatientName: a.PatientName, Priority: a.Priority})
	}

	writeJSON(w, http.StatusOK, queue)
}
