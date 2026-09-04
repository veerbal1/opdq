package handler

import (
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/veerbal/opdq/internal/domain"
)

type PatientResponse struct {
	TokenNo    int     `json:"token_no"`
	State      string  `json:"state"`
	NowServing *int    `json:"now_serving"`
	Position   *int    `json:"position"`
	ETA        *string `json:"eta"`
	Message    string  `json:"message"`
}

// eta answers "when is this patient likely to be called".
//
//	start of the queue = the later of (session start + delay) and now
//	                     — a session that began an hour ago does not owe
//	                       anyone an ETA in the past
//	then add one average consultation for every patient ahead
func eta(v domain.PatientView, now time.Time) time.Time {
	from := v.SessionStart.Add(time.Duration(v.DelayMin) * time.Minute)
	if from.Before(now) {
		from = now
	}
	return from.Add(time.Duration(v.Ahead) * time.Duration(v.AvgConsultSec) * time.Second)
}

// PatientHandler is public. The unguessable public_id is the credential, so an
// unknown one must look exactly like any other unknown one.
func (h *Handler) PatientHandler(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("public_id")
	if publicID == "" {
		writeErrorMessage(w, http.StatusNotFound, "not found")
		return
	}

	v, err := h.store.PatientViewByPublicID(r.Context(), publicID)
	if err != nil {
		// Never distinguish a malformed id from a real one that does not exist.
		writeErrorMessage(w, http.StatusNotFound, "not found")
		return
	}

	resp := PatientResponse{
		TokenNo:    v.TokenNo,
		State:      string(v.State),
		NowServing: v.NowServing,
	}

	// Every state gets an honest answer, not just the happy one.
	switch v.State {
	case domain.Waiting:
		ahead := v.Ahead
		at := eta(v, time.Now()).Format(time.RFC3339)
		resp.Position = &ahead
		resp.ETA = &at
		resp.Message = "waiting"
	case domain.InConsultation:
		resp.Message = "you are with the doctor"
	case domain.Done:
		resp.Message = "visit complete"
	case domain.Absent:
		resp.Message = "you were marked absent, please see reception"
	}

	writeJSON(w, http.StatusOK, resp)
}

// publicBaseURL is where the patient link points. The app is plain HTTP behind
// Caddy in production, so the scheme cannot be read off the request — it comes
// from the same config flag that decides Secure cookies.
func (h *Handler) publicBaseURL(r *http.Request) string {
	scheme := "http"
	if h.secureCookies {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// PatientQRHandler returns a PNG of the patient's own /q/ link, so the console
// can show it and the patient can scan it straight off the tablet.
func (h *Handler) PatientQRHandler(w http.ResponseWriter, r *http.Request) {
	publicID := r.PathValue("public_id")
	if publicID == "" {
		writeErrorMessage(w, http.StatusNotFound, "not found")
		return
	}

	// Refuse to print a QR for a link that leads nowhere.
	if _, err := h.store.PatientViewByPublicID(r.Context(), publicID); err != nil {
		writeErrorMessage(w, http.StatusNotFound, "not found")
		return
	}

	png, err := qrcode.Encode(h.publicBaseURL(r)+"/q/"+publicID, qrcode.Medium, 256)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(png)
}
