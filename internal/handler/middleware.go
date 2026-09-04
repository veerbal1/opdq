package handler

import (
	"context"
	"net/http"

	"github.com/veerbal/opdq/internal/auth"
	"github.com/veerbal/opdq/internal/domain"
)

type contextKey struct{}

var sessionKey contextKey

func (h *Handler) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil {
			writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		sess, err := h.store.GetAuthSession(r.Context(), auth.HashToken(c.Value))
		if err != nil {
			writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next(w, r.WithContext(ctx))
	}
}

func sessionFrom(ctx context.Context) (domain.AuthSession, bool) {
	sess, ok := ctx.Value(sessionKey).(domain.AuthSession)
	return sess, ok
}

func (h *Handler) RequireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sessionFrom(r.Context())
		if !ok {
			writeErrorMessage(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		header := r.Header.Get("X-CSRF-Token")
		if header != sess.CSRFToken {
			writeErrorMessage(w, http.StatusForbidden, "invalid csrf token")
			return
		}
		next(w, r)
	}
}

func (h *Handler) protected(next http.HandlerFunc) http.HandlerFunc {
	return h.RequireAuth(h.RequireCSRF(next))
}
