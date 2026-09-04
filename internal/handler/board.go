package handler

import (
	"net/http"
	"strconv"

	"github.com/veerbal/opdq/internal/domain"
)

type BoardResponse struct {
	SessionID  int64  `json:"session_id"`
	DoctorName string `json:"doctor_name"`
	DelayMin   int    `json:"delay_min"`
	Status     string `json:"status"`
	NowServing *int   `json:"now_serving"`
	Next       []int  `json:"next"`
}

func (h *Handler) BoardHandler(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid session id")
		return
	}

	board, err := h.store.BoardForSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := BoardResponse{
		SessionID:  board.SessionID,
		DoctorName: board.DoctorName,
		DelayMin:   board.DelayMin,
		Status:     string(board.Status),
		Next:       []int{},
	}

	for _, e := range board.Entries {
		if e.State == domain.InConsultation {
			token := e.TokenNo
			resp.NowServing = &token
			continue
		}
		if len(resp.Next) < 5 {
			resp.Next = append(resp.Next, e.TokenNo)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
