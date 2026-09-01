package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/veerbal/opdq/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{
		pool: pool,
	}
}

func (s *Store) CreateClinic(ctx context.Context, name string) (domain.Clinic, error) {
	var clinic domain.Clinic
	clinic.Name = name

	err := s.pool.QueryRow(ctx, "INSERT INTO clinics (name) VALUES ($1) RETURNING id, public_id", name).Scan(&clinic.ID, &clinic.PublicID)

	if err != nil {
		return domain.Clinic{}, fmt.Errorf("create clinic: %w", err)
	}

	return clinic, nil
}

func (s *Store) CreateDoctor(ctx context.Context, name string, clinicID int64) (domain.Doctor, error) {
	var doctor domain.Doctor
	doctor.Name = name
	doctor.ClinicID = clinicID

	err := s.pool.QueryRow(ctx, "INSERT INTO doctors (name, clinic_id) VALUES ($1, $2) RETURNING id", name, clinicID).Scan(&doctor.ID)

	if err != nil {
		return domain.Doctor{}, fmt.Errorf("create doctor: %w", err)
	}

	return doctor, nil
}

func (s *Store) CreateSession(ctx context.Context, clinicID, doctorID int64, sessionDate, startsAt, endsAt time.Time, capacity int) (domain.Session, error) {
	var session domain.Session
	session.DoctorID = doctorID
	session.ClinicID = clinicID
	session.SessionDate = sessionDate
	session.StartsAt = startsAt
	session.EndsAt = endsAt
	session.Capacity = capacity

	err := s.pool.QueryRow(ctx, "INSERT INTO sessions (doctor_id, clinic_id, session_date, starts_at, ends_at, capacity) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, status", doctorID, clinicID, sessionDate, startsAt, endsAt, capacity).Scan(&session.ID, &session.Status)

	if err != nil {
		return domain.Session{}, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *Store) CreateWalkIn(ctx context.Context, sessionID int64, patientName, contact string, priority int) (domain.Appointment, error) {
	var clinicID int64
	var endsAt time.Time
	err := s.pool.QueryRow(ctx, "SELECT clinic_id, ends_at FROM sessions WHERE id = $1;", sessionID).Scan(&clinicID, &endsAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Appointment{}, domain.ErrSessionNotFound
		}
		return domain.Appointment{}, fmt.Errorf("create walk-in: fetch session: %w", err)
	}

	// 2. time check — agar session khatam ho chuka, reject karo
	if endsAt.Before(time.Now()) {
		return domain.Appointment{}, domain.ErrSessionEnded
	}

	// 3. next token number nikalo (naive — SELECT MAX(token_no)+1)
	var tokenNumber int
	err = s.pool.QueryRow(ctx, "SELECT COALESCE(MAX(token_no), 0) + 1 FROM appointments WHERE session_id = $1;", sessionID).Scan(&tokenNumber)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: next token: %w", err)
	}

	// 4. insert karo appointment (clinic_id, session_id, token_no, patient_name, contact, queued_at=now, priority, state='waiting')
	var appointment domain.Appointment
	appointment.ClinicID = clinicID
	appointment.SessionID = sessionID
	appointment.TokenNo = tokenNumber
	appointment.PatientName = patientName
	appointment.Contact = contact
	appointment.Priority = priority
	appointment.State = domain.Waiting

	err = s.pool.QueryRow(ctx,
		"INSERT INTO appointments (clinic_id, session_id, token_no, patient_name, contact, queued_at, priority, state) VALUES ($1, $2, $3, $4, $5, now(), $6, $7) RETURNING id, queued_at",
		clinicID, sessionID, tokenNumber, patientName, contact, priority, domain.Waiting,
	).Scan(&appointment.ID, &appointment.QueuedAt)

	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: insert: %w", err)
	}

	return appointment, nil
}

func (s *Store) TransitionAppointment(ctx context.Context, appointmentID int64, to domain.State) (domain.Appointment, error) {
	var currentState domain.State
	err := s.pool.QueryRow(ctx, "SELECT state from appointments WHERE id = $1", appointmentID).Scan(&currentState)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Appointment{}, domain.ErrAppointmentNotFound
		}
		return domain.Appointment{}, fmt.Errorf("transition appointment: fetch state: %w", err)
	}

	err = domain.Transition(currentState, to)
	if err != nil {
		return domain.Appointment{}, domain.ErrIllegalTransition
	}

	var appointment domain.Appointment
	updateQuery := "UPDATE appointments SET state = $1, queued_at = CASE WHEN $1 = 'waiting' THEN now() ELSE queued_at END WHERE id = $2 RETURNING id, clinic_id, session_id, token_no, patient_name, contact, queued_at, priority, state;"
	err = s.pool.QueryRow(ctx, updateQuery, to, appointmentID).Scan(
		&appointment.ID,
		&appointment.ClinicID,
		&appointment.SessionID,
		&appointment.TokenNo,
		&appointment.PatientName,
		&appointment.Contact,
		&appointment.QueuedAt,
		&appointment.Priority,
		&appointment.State,
	)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("transition appointment: update: %w", err)
	}

	return appointment, nil
}

func (s *Store) QueueForSession(ctx context.Context, sessionID int64) ([]domain.Appointment, error) {
	query := "SELECT id, clinic_id, session_id, token_no, patient_name, contact, queued_at, priority, state FROM appointments WHERE session_id = $1 AND state = 'waiting' ORDER BY priority DESC, queued_at ASC"
	rows, err := s.pool.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("queue for session: %w", err)
	}
	defer rows.Close()

	var appointments []domain.Appointment

	for rows.Next() {
		var a domain.Appointment
		err := rows.Scan(&a.ID, &a.ClinicID, &a.SessionID, &a.TokenNo, &a.PatientName, &a.Contact, &a.QueuedAt, &a.Priority, &a.State)
		if err != nil {
			return nil, fmt.Errorf("queue for session: %w", err)
		}
		appointments = append(appointments, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("queue for session: %w", err)
	}

	return appointments, nil
}
