package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	testMux.ServeHTTP(rec, req)
	return rec
}

func TestProtectedRouteRejectsMissingCookie(t *testing.T) {
	resetDB(t)

	rec := do(httptest.NewRequest("GET", "/api/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a cookie, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProtectedRouteRejectsTamperedCookie(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sess.cookie.Value + "x"})

	rec := do(req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a tampered cookie, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMutationRejectedWithoutCSRFHeader(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	req := httptest.NewRequest("POST", "/api/doctors", strings.NewReader(`{"name":"Dr. NoCSRF"}`))
	req.AddCookie(sess.cookie) // cookie yes, X-CSRF-Token no

	rec := do(req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without a CSRF header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMutationRejectedWithWrongCSRFToken(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	req := httptest.NewRequest("POST", "/api/doctors", strings.NewReader(`{"name":"Dr. BadCSRF"}`))
	req.AddCookie(sess.cookie)
	req.Header.Set("X-CSRF-Token", "not-the-real-token")

	rec := do(req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a wrong CSRF token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogoutDeletesSessionRow(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)
	ctx := context.Background()

	var before int
	if err := testAdminPool.QueryRow(ctx, "SELECT count(*) FROM auth_sessions").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 1 {
		t.Fatalf("expected 1 auth session after login, got %d", before)
	}

	if rec := do(authedRequest(t, sess, "POST", "/api/logout", "")); rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from logout, got %d: %s", rec.Code, rec.Body.String())
	}

	var after int
	if err := testAdminPool.QueryRow(ctx, "SELECT count(*) FROM auth_sessions").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 0 {
		t.Fatalf("expected 0 auth sessions after logout, got %d", after)
	}

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.AddCookie(sess.cookie)
	if rec := do(req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 reusing the logged-out cookie, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	resetDB(t)
	loginTestUser(t)

	wrongPassword := do(httptest.NewRequest("POST", "/api/login",
		strings.NewReader(`{"email":"test@clinic.com","password":"wrong"}`)))
	unknownEmail := do(httptest.NewRequest("POST", "/api/login",
		strings.NewReader(`{"email":"nobody@clinic.com","password":"hunter2"}`)))

	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", wrongPassword.Code)
	}
	if unknownEmail.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email: expected 401, got %d", unknownEmail.Code)
	}
	if wrongPassword.Body.String() != unknownEmail.Body.String() {
		t.Fatalf("responses differ and leak which emails exist:\n  wrong password: %s\n  unknown email:  %s",
			wrongPassword.Body.String(), unknownEmail.Body.String())
	}
	if len(wrongPassword.Result().Cookies()) != 0 {
		t.Fatal("failed login must not set a cookie")
	}
}
