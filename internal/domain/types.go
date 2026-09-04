package domain

import (
	"time"

	"github.com/google/uuid"
)

type Contact struct {
	Channel string
	Address string
}

type Appointment struct {
	ID          int64
	ClinicID    int64
	SessionID   int64
	TokenNo     int
	PatientName string
	Contact     Contact
	QueuedAt    time.Time
	Priority    int
	State       State
}

type SessionStatus string

const (
	Open      SessionStatus = "open"
	Closed    SessionStatus = "closed"
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

	AvgConsultSec int
}

type SessionWithDoctor struct {
	Session
	DoctorName string
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

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleReceptionist Role = "receptionist"
)

type StaffUser struct {
	ID           int64
	ClinicID     int64
	Name         string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

type AuthSession struct {
	ID        int64
	UserID    int64
	ClinicID  int64
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
}
