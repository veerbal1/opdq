package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/veerbal/opdq/internal/handler"
)

func TestFullFlow(t *testing.T) {
	resetDB(t)
	req := httptest.NewRequest("POST", "/clinics", strings.NewReader(`{"name":"Test Clinic"}`))
	rec := httptest.NewRecorder()
	testMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var clinicResp handler.CreateClinicResponse
	if err := json.NewDecoder(rec.Body).Decode(&clinicResp); err != nil {
		t.Fatal(err)
	}

	doctorReq := httptest.NewRequest("POST", fmt.Sprintf("/clinics/%d/doctors", clinicResp.ID), strings.NewReader(`{"name":"Dr. Test"}`))
	doctorRec := httptest.NewRecorder()
	testMux.ServeHTTP(doctorRec, doctorReq)

	if doctorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", doctorRec.Code, doctorRec.Body.String())
	}

	var doctorResp handler.CreateDoctorResponse
	if err := json.NewDecoder(doctorRec.Body).Decode(&doctorResp); err != nil {
		t.Fatal(err)
	}

	sessionBody := fmt.Sprintf(`{"clinic_id":%d,"doctor_id":%d,"starts_at":"2026-09-03T10:00:00+05:30","ends_at":"2026-09-03T18:00:00+05:30","capacity":30}`, clinicResp.ID, doctorResp.ID)
	sessionReq := httptest.NewRequest("POST", "/sessions", strings.NewReader(sessionBody))
	sessionRec := httptest.NewRecorder()
	testMux.ServeHTTP(sessionRec, sessionReq)

	if sessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", sessionRec.Code, sessionRec.Body.String())
	}

	var sessionResp handler.CreateSessionResponse
	if err := json.NewDecoder(sessionRec.Body).Decode(&sessionResp); err != nil {
		t.Fatal(err)
	}

	walkinReq := httptest.NewRequest("POST", fmt.Sprintf("/sessions/%d/walkins", sessionResp.ID), strings.NewReader(`{"patient_name":"Ravi","contact":"9999999999","priority":0}`))
	walkinRec := httptest.NewRecorder()
	testMux.ServeHTTP(walkinRec, walkinReq)

	if walkinRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", walkinRec.Code, walkinRec.Body.String())
	}

	var walkinResp handler.CreateWalkInResponse
	if err := json.NewDecoder(walkinRec.Body).Decode(&walkinResp); err != nil {
		t.Fatal(err)
	}
	if walkinResp.TokenNo != 1 {
		t.Fatalf("expected token_no 1, got %d", walkinResp.TokenNo)
	}

	queueBeforeReq := httptest.NewRequest("GET", fmt.Sprintf("/sessions/%d/queue", sessionResp.ID), nil)
	queueBeforeRec := httptest.NewRecorder()
	testMux.ServeHTTP(queueBeforeRec, queueBeforeReq)

	if queueBeforeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", queueBeforeRec.Code, queueBeforeRec.Body.String())
	}
	if !strings.Contains(queueBeforeRec.Body.String(), `"token_no":1`) {
		t.Fatalf("expected token_no 1 in queue, got %s", queueBeforeRec.Body.String())
	}

	transitionReq := httptest.NewRequest("POST", fmt.Sprintf("/appointments/%d/transition", walkinResp.ID), strings.NewReader(`{"to":"in_consultation"}`))
	transitionRec := httptest.NewRecorder()
	testMux.ServeHTTP(transitionRec, transitionReq)

	if transitionRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", transitionRec.Code, transitionRec.Body.String())
	}

	queueReq := httptest.NewRequest("GET", fmt.Sprintf("/sessions/%d/queue", sessionResp.ID), nil)
	queueRec := httptest.NewRecorder()
	testMux.ServeHTTP(queueRec, queueReq)

	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", queueRec.Code, queueRec.Body.String())
	}
	if queueRec.Body.String() != "[]\n" {
		t.Fatalf("expected empty queue, got %s", queueRec.Body.String())
	}

}
