package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/veerbal/opdq/internal/handler"
)

func TestConcurrentWalkIns(t *testing.T) {
	resetDB(t)

	clinicReq := httptest.NewRequest("POST", "/clinics", strings.NewReader(`{"name":"Race Clinic"}`))
	clinicRec := httptest.NewRecorder()
	testMux.ServeHTTP(clinicRec, clinicReq)
	var clinicResp handler.CreateClinicResponse
	json.NewDecoder(clinicRec.Body).Decode(&clinicResp)

	doctorReq := httptest.NewRequest("POST", fmt.Sprintf("/clinics/%d/doctors", clinicResp.ID), strings.NewReader(`{"name":"Dr. Race"}`))
	doctorRec := httptest.NewRecorder()
	testMux.ServeHTTP(doctorRec, doctorReq)
	var doctorResp handler.CreateDoctorResponse
	json.NewDecoder(doctorRec.Body).Decode(&doctorResp)

	sessionBody := fmt.Sprintf(`{"clinic_id":%d,"doctor_id":%d,"starts_at":"2026-09-03T10:00:00+05:30","ends_at":"2026-09-03T18:00:00+05:30","capacity":30}`, clinicResp.ID, doctorResp.ID)
	sessionReq := httptest.NewRequest("POST", "/sessions", strings.NewReader(sessionBody))
	sessionRec := httptest.NewRecorder()
	testMux.ServeHTTP(sessionRec, sessionReq)
	var sessionResp handler.CreateSessionResponse
	json.NewDecoder(sessionRec.Body).Decode(&sessionResp)

	const n = 20
	var wg sync.WaitGroup
	results := make(chan int, n)
	bodies := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", fmt.Sprintf("/sessions/%d/walkins", sessionResp.ID), strings.NewReader(`{"patient_name":"Patient","contact":"999","priority":0}`))
			rec := httptest.NewRecorder()
			testMux.ServeHTTP(rec, req)
			results <- rec.Code
			bodies <- rec.Body.String()
		}()
	}

	wg.Wait()
	close(results)
	close(bodies)

	success, failed := 0, 0
	for code := range results {
		if code == http.StatusCreated {
			success++
		} else {
			failed++
		}
	}

	t.Logf("20 concurrent walk-ins: %d succeeded, %d failed", success, failed)
	for body := range bodies {
		t.Logf("response: %s", body)
	}
}
