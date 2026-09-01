package domain

import "time"

type Appointment struct {
	ID          int64
	ClinicID    int64
	SessionID   int64
	TokenNo     int
	PatientName string
	Contact     string
	QueuedAt    time.Time
	Priority    int
	State       State
}

type SessionStatus string

const (
	Scheduled SessionStatus = "scheduled"
	Active    SessionStatus = "active"
	Completed SessionStatus = "completed"
	Cancelled SessionStatus = "cancelled"
)

type Session struct {
	ID          int64
	ClinicID    int64
	DoctorID    int64
	SessionDate time.Time
	StartsAt    time.Time
	EndsAt      time.Time
	Capacity    int
	DelayMin    int
	Status      SessionStatus
	Version     int
}
