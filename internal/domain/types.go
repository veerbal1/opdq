package domain

import (
	"time"

	"github.com/google/uuid"
)

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
	Open      SessionStatus = "open"
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

type Clinic struct {
	ID       int64
	Name     string
	PublicID uuid.UUID
}

type Doctor struct {
	ID       int64
	ClinicID int64
	Name     string
}
