package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/veerbal/opdq/internal/handler"
)

func TestConcurrentWalkIns(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	doctorReq := authedRequest(t, sess, "POST", "/doctors", `{"name":"Dr. Race"}`)
	doctorRec := httptest.NewRecorder()
	testMux.ServeHTTP(doctorRec, doctorReq)
	var doctorResp handler.CreateDoctorResponse
	if err := json.NewDecoder(doctorRec.Body).Decode(&doctorResp); err != nil {
		t.Fatalf("doctor: %d %s", doctorRec.Code, doctorRec.Body.String())
	}

	startsAt := time.Now().Add(-time.Hour).Format(time.RFC3339)
	endsAt := time.Now().Add(8 * time.Hour).Format(time.RFC3339)
	sessionBody := fmt.Sprintf(
		`{"doctor_id":%d,"starts_at":%q,"ends_at":%q,"capacity":30}`,
		doctorResp.ID, startsAt, endsAt)

	sessionReq := authedRequest(t, sess, "POST", "/sessions", sessionBody)
	sessionRec := httptest.NewRecorder()
	testMux.ServeHTTP(sessionRec, sessionReq)
	var sessionResp handler.CreateSessionResponse
	if err := json.NewDecoder(sessionRec.Body).Decode(&sessionResp); err != nil {
		t.Fatalf("session: %d %s", sessionRec.Code, sessionRec.Body.String())
	}

	const n = 20
	var wg sync.WaitGroup
	results := make(chan int, n)
	bodies := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := authedRequest(t, sess, "POST",
				fmt.Sprintf("/sessions/%d/walkins", sessionResp.ID),
				`{"patient_name":"Patient","contact":"999","priority":0}`)
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
		switch code {
		case http.StatusCreated:
			success++
		case http.StatusInternalServerError:
			failed++
		default:
			t.Fatalf("unexpected status %d — requests are not reaching CreateWalkIn", code)
		}
	}

	t.Logf("%d concurrent walk-ins: %d succeeded, %d failed", n, success, failed)

	if success < 1 {
		t.Fatal("expected at least one walk-in to succeed")
	}
	if success+failed != n {
		t.Fatalf("expected %d total responses, got %d + %d", n, success, failed)
	}

	for body := range bodies {
		t.Logf("response: %s", body)
	}
}
