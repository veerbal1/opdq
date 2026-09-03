package handler_test

import (
	"context"
	"strings"
	"testing"
)

func TestAppointmentEventsAreAppendOnly(t *testing.T) {
	ctx := context.Background()

	_, err := testPool.Exec(ctx, "UPDATE appointment_events SET to_state = 'done' WHERE id = 1")
	if err == nil {
		t.Fatal("UPDATE on appointment_events succeeded, expected permission denied")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied, got: %v", err)
	}

	_, err = testPool.Exec(ctx, "DELETE FROM appointment_events WHERE id = 1")
	if err == nil {
		t.Fatal("DELETE on appointment_events succeeded, expected permission denied")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied, got: %v", err)
	}
}
