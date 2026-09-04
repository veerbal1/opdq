package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/veerbal/opdq/internal/handler"
)

// newSession creates a doctor and one open sitting for today, and returns the
// session as the API reports it (so the caller has a real version to guard on).
func newSession(t *testing.T, sess testSession) handler.SessionItem {
	t.Helper()

	docRec := do(authedRequest(t, sess, "POST", "/api/doctors", `{"name":"Dr. Version"}`))
	if docRec.Code != http.StatusCreated {
		t.Fatalf("doctor: %d %s", docRec.Code, docRec.Body.String())
	}
	var doc handler.CreateDoctorResponse
	if err := json.NewDecoder(docRec.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}

	body := fmt.Sprintf(`{"doctor_id":%d,"starts_at":%q,"ends_at":%q,"capacity":30}`,
		doc.ID,
		time.Now().Add(-time.Hour).Format(time.RFC3339),
		time.Now().Add(8*time.Hour).Format(time.RFC3339))

	if rec := do(authedRequest(t, sess, "POST", "/api/sessions", body)); rec.Code != http.StatusCreated {
		t.Fatalf("session: %d %s", rec.Code, rec.Body.String())
	}

	listRec := do(authedRequest(t, sess, "GET", "/api/sessions", ""))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list sessions: %d %s", listRec.Code, listRec.Body.String())
	}
	var list []handler.SessionItem
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 session today, got %d", len(list))
	}
	return list[0]
}

func TestSessionsForDateReturnsTodaysSittings(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)

	created := newSession(t, sess)
	if created.DoctorName != "Dr. Version" {
		t.Fatalf("expected the doctor's name to be joined in, got %q", created.DoctorName)
	}
	if created.Version != 1 || created.DelayMin != 0 || created.Status != "open" {
		t.Fatalf("unexpected new session: %+v", created)
	}

	// a different day must not see it
	other := do(authedRequest(t, sess, "GET", "/api/sessions?date=2020-01-01", ""))
	if other.Body.String() != "[]\n" {
		t.Fatalf("expected no sessions on an unrelated date, got %s", other.Body.String())
	}
}

// Two receptionists both read version 1; the second write must be refused rather
// than silently overwriting the first.
func TestSetDelayRejectsStaleVersion(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)
	s := newSession(t, sess)

	path := fmt.Sprintf("/api/sessions/%d/delay", s.ID)

	first := do(authedRequest(t, sess, "POST", path,
		fmt.Sprintf(`{"delay_min":30,"version":%d}`, s.Version)))
	if first.Code != http.StatusOK {
		t.Fatalf("first delay: expected 200, got %d: %s", first.Code, first.Body.String())
	}

	var updated handler.SessionItem
	if err := json.NewDecoder(first.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.DelayMin != 30 {
		t.Fatalf("expected delay_min 30, got %d", updated.DelayMin)
	}
	if updated.Version != s.Version+1 {
		t.Fatalf("expected version to advance to %d, got %d", s.Version+1, updated.Version)
	}

	// same version again — this is the stale write
	second := do(authedRequest(t, sess, "POST", path,
		fmt.Sprintf(`{"delay_min":45,"version":%d}`, s.Version)))
	if second.Code != http.StatusConflict {
		t.Fatalf("stale delay: expected 409, got %d: %s", second.Code, second.Body.String())
	}

	// and the first write must still be the one that stuck
	after := do(authedRequest(t, sess, "GET", "/api/sessions", ""))
	var list []handler.SessionItem
	if err := json.NewDecoder(after.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list[0].DelayMin != 30 {
		t.Fatalf("the rejected write leaked through: delay_min is %d, expected 30", list[0].DelayMin)
	}
}

func TestCloseSession(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)
	s := newSession(t, sess)

	path := fmt.Sprintf("/api/sessions/%d/close", s.ID)

	stale := do(authedRequest(t, sess, "POST", path, `{"version":99}`))
	if stale.Code != http.StatusConflict {
		t.Fatalf("close with a wrong version: expected 409, got %d: %s", stale.Code, stale.Body.String())
	}

	ok := do(authedRequest(t, sess, "POST", path, fmt.Sprintf(`{"version":%d}`, s.Version)))
	if ok.Code != http.StatusOK {
		t.Fatalf("close: expected 200, got %d: %s", ok.Code, ok.Body.String())
	}
	var closed handler.SessionItem
	if err := json.NewDecoder(ok.Body).Decode(&closed); err != nil {
		t.Fatal(err)
	}
	if closed.Status != "closed" {
		t.Fatalf("expected status closed, got %q", closed.Status)
	}
}

// A whole second clinic, built directly in the database, must be invisible to
// the first clinic's user — not forbidden, invisible. Its sittings must not
// appear in the list, and writing to one must fail.
func TestAnotherClinicsSessionIsInvisible(t *testing.T) {
	resetDB(t)
	sess := loginTestUser(t)
	mine := newSession(t, sess)

	ctx := t.Context()

	var otherClinic, otherDoctor, otherSession int64
	if err := testAdminPool.QueryRow(ctx,
		"INSERT INTO clinics (name) VALUES ('Other Clinic') RETURNING id").Scan(&otherClinic); err != nil {
		t.Fatal(err)
	}
	if err := testAdminPool.QueryRow(ctx,
		"INSERT INTO doctors (name, clinic_id) VALUES ('Dr. Elsewhere', $1) RETURNING id",
		otherClinic).Scan(&otherDoctor); err != nil {
		t.Fatal(err)
	}
	if err := testAdminPool.QueryRow(ctx,
		`INSERT INTO sessions (clinic_id, doctor_id, session_date, starts_at, ends_at, capacity)
		 VALUES ($1, $2, CURRENT_DATE, now() - interval '1 hour', now() + interval '8 hours', 30)
		 RETURNING id`, otherClinic, otherDoctor).Scan(&otherSession); err != nil {
		t.Fatal(err)
	}

	listRec := do(authedRequest(t, sess, "GET", "/api/sessions", ""))
	var list []handler.SessionItem
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("the other clinic's session leaked into the list: %+v", list)
	}

	// Writing to it must fail, and the failure must not distinguish "not yours"
	// from "does not exist" — both are a plain conflict, no 403.
	delay := do(authedRequest(t, sess, "POST",
		fmt.Sprintf("/api/sessions/%d/delay", otherSession), `{"delay_min":10,"version":1}`))
	if delay.Code == http.StatusOK {
		t.Fatal("updated a session belonging to another clinic")
	}
	if delay.Code == http.StatusForbidden {
		t.Fatalf("403 confirms the session exists; expected the same answer as for a missing one, got %s", delay.Body.String())
	}

	// The queue of a session in another clinic must come back empty, not 403.
	queueRec := do(authedRequest(t, sess, "GET",
		fmt.Sprintf("/api/sessions/%d/queue", otherSession), ""))
	if queueRec.Code != http.StatusOK || queueRec.Body.String() != "[]\n" {
		t.Fatalf("expected an empty queue for another clinic's session, got %d %s",
			queueRec.Code, queueRec.Body.String())
	}
}
