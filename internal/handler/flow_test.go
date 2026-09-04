package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/veerbal/opdq/internal/handler"
)

func TestFullFlow(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	doctorReq := authedRequest(t, sess, "POST", "/doctors", `{"name":"Dr. Test"}`)
	doctorRec := httptest.NewRecorder()
	testMux.ServeHTTP(doctorRec, doctorReq)

	if doctorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", doctorRec.Code, doctorRec.Body.String())
	}

	var doctorResp handler.CreateDoctorResponse
	if err := json.NewDecoder(doctorRec.Body).Decode(&doctorResp); err != nil {
		t.Fatal(err)
	}

	startsAt := time.Now().Add(-time.Hour).Format(time.RFC3339)
	endsAt := time.Now().Add(8 * time.Hour).Format(time.RFC3339)
	sessionBody := fmt.Sprintf(
		`{"doctor_id":%d,"starts_at":%q,"ends_at":%q,"capacity":30}`,
		doctorResp.ID, startsAt, endsAt)

	sessionReq := authedRequest(t, sess, "POST", "/sessions", sessionBody)
	sessionRec := httptest.NewRecorder()
	testMux.ServeHTTP(sessionRec, sessionReq)

	if sessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", sessionRec.Code, sessionRec.Body.String())
	}

	var sessionResp handler.CreateSessionResponse
	if err := json.NewDecoder(sessionRec.Body).Decode(&sessionResp); err != nil {
		t.Fatal(err)
	}

	walkinReq := authedRequest(t, sess, "POST",
		fmt.Sprintf("/sessions/%d/walkins", sessionResp.ID),
		`{"patient_name":"Ravi","contact":"9999999999","priority":0}`)
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

	queueBeforeReq := authedRequest(t, sess, "GET",
		fmt.Sprintf("/sessions/%d/queue", sessionResp.ID), "")
	queueBeforeRec := httptest.NewRecorder()
	testMux.ServeHTTP(queueBeforeRec, queueBeforeReq)

	if queueBeforeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", queueBeforeRec.Code, queueBeforeRec.Body.String())
	}
	if !strings.Contains(queueBeforeRec.Body.String(), `"token_no":1`) {
		t.Fatalf("expected token_no 1 in queue, got %s", queueBeforeRec.Body.String())
	}

	transitionReq := authedRequest(t, sess, "POST",
		fmt.Sprintf("/appointments/%d/transition", walkinResp.ID), `{"to":"in_consultation"}`)
	transitionRec := httptest.NewRecorder()
	testMux.ServeHTTP(transitionRec, transitionReq)

	if transitionRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", transitionRec.Code, transitionRec.Body.String())
	}

	queueReq := authedRequest(t, sess, "GET",
		fmt.Sprintf("/sessions/%d/queue", sessionResp.ID), "")
	queueRec := httptest.NewRecorder()
	testMux.ServeHTTP(queueRec, queueReq)

	if queueRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", queueRec.Code, queueRec.Body.String())
	}
	if queueRec.Body.String() != "[]\n" {
		t.Fatalf("expected empty queue, got %s", queueRec.Body.String())
	}

	rows, err := testPool.Query(context.Background(),
		`SELECT from_state, to_state FROM appointment_events WHERE appointment_id = $1 ORDER BY id`,
		walkinResp.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type event struct {
		from *string
		to   string
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.from, &e.to); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].from != nil || events[0].to != "waiting" {
		t.Fatalf("event 1: expected NULL -> waiting, got %v -> %q", events[0].from, events[0].to)
	}
	if events[1].from == nil || *events[1].from != "waiting" || events[1].to != "in_consultation" {
		t.Fatalf("event 2: expected waiting -> in_consultation")
	}
}
