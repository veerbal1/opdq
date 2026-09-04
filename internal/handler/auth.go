package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/veerbal/opdq/internal/auth"
)

const sessionDuration = 7 * 24 * time.Hour

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	ClinicID  int64  `json:"clinic_id"`
	Role      string `json:"role"`
	CSRFToken string `json:"csrf_token"`
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorMessage(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.store.GetStaffUserByEmail(r.Context(), req.Email)

	hash := auth.DummyHash
	if err == nil {
		hash = user.PasswordHash
	}

	cmpErr := auth.CheckPassword(hash, req.Password)
	if err != nil || cmpErr != nil {
		writeErrorMessage(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		writeError(w, err)
		return
	}

	csrfToken, err := auth.NewToken()
	if err != nil {
		writeError(w, err)
		return
	}

	expiresAt := time.Now().Add(sessionDuration)

	_, err = h.store.CreateAuthSession(r.Context(), auth.HashToken(token), user.ID, user.ClinicID, csrfToken, expiresAt)
	if err != nil {
		writeError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, LoginResponse{
		UserID:    user.ID,
		Name:      user.Name,
		ClinicID:  user.ClinicID,
		Role:      string(user.Role),
		CSRFToken: csrfToken,
	})
}
