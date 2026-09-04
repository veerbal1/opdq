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
	if err := domain.ValidateSession(startsAt, endsAt, capacity); err != nil {
		return domain.Session{}, err
	}

	var session domain.Session
	session.DoctorID = doctorID
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

func (s *Store) CreateWalkIn(ctx context.Context, clinicID, sessionID int64, patientName string, contact domain.Contact, priority int, actorID *int64) (domain.Appointment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var endsAt time.Time
	err = tx.QueryRow(ctx, "SELECT clinic_id, ends_at FROM sessions WHERE id = $1", sessionID).Scan(&clinicID, &endsAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Appointment{}, domain.ErrSessionNotFound
		}
		return domain.Appointment{}, fmt.Errorf("create walk-in: fetch session: %w", err)
	}

	if endsAt.Before(time.Now()) {
		return domain.Appointment{}, domain.ErrSessionEnded
	}

	var tokenNumber int
	err = tx.QueryRow(ctx, "SELECT COALESCE(MAX(token_no), 0) + 1 FROM appointments WHERE session_id = $1", sessionID).Scan(&tokenNumber)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: next token: %w", err)
	}

	var channel, address *string
	if contact.Channel != "" {
		channel = &contact.Channel
		address = &contact.Address
	}

	var appointment domain.Appointment
	appointment.ClinicID = clinicID
	appointment.SessionID = sessionID
	appointment.TokenNo = tokenNumber
	appointment.PatientName = patientName
	appointment.Contact = contact
	appointment.Priority = priority
	appointment.State = domain.Waiting

	err = tx.QueryRow(ctx,
		`INSERT INTO appointments (clinic_id, session_id, token_no, patient_name, contact_channel, contact_address, queued_at, priority, state)
		 VALUES ($1, $2, $3, $4, $5, $6, now(), $7, $8)
		 RETURNING id, queued_at`,
		clinicID, sessionID, tokenNumber, patientName, channel, address, priority, domain.Waiting,
	).Scan(&appointment.ID, &appointment.QueuedAt)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: insert: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO appointment_events (clinic_id, appointment_id, from_state, to_state, actor_id, reason)
		 VALUES ($1, $2, NULL, $3, $4, NULL)`,
		clinicID, appointment.ID, domain.Waiting, actorID,
	)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Appointment{}, fmt.Errorf("create walk-in: commit: %w", err)
	}

	return appointment, nil
}

func (s *Store) TransitionAppointment(ctx context.Context, clinicID, appointmentID int64, to domain.State, actorID *int64, reason string) (domain.Appointment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("transition appointment: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentState domain.State
	err = tx.QueryRow(ctx, "SELECT state FROM appointments WHERE id = $1 AND clinic_id = $2 FOR UPDATE", appointmentID, clinicID).Scan(&currentState)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Appointment{}, domain.ErrAppointmentNotFound
		}
		return domain.Appointment{}, fmt.Errorf("transition appointment: fetch state: %w", err)
	}

	if err := domain.Transition(currentState, to); err != nil {
		return domain.Appointment{}, domain.ErrIllegalTransition
	}

	updateQuery := `
		UPDATE appointments
		SET state        = $1,
		    queued_at    = CASE WHEN $1 = 'waiting'         THEN now() ELSE queued_at END,
		    started_at   = CASE WHEN $1 = 'in_consultation' THEN now() ELSE started_at END,
		    completed_at = CASE WHEN $1 = 'done'            THEN now() ELSE completed_at END,
		    version      = version + 1
		WHERE id = $2 AND clinic_id = $3
		RETURNING id, clinic_id, session_id, token_no, patient_name,
		          contact_channel, contact_address, queued_at, priority, state`

	var appointment domain.Appointment
	var channel, address *string
	err = tx.QueryRow(ctx, updateQuery, to, appointmentID, clinicID).Scan(
		&appointment.ID, &appointment.ClinicID, &appointment.SessionID,
		&appointment.TokenNo, &appointment.PatientName, &channel, &address,
		&appointment.QueuedAt, &appointment.Priority, &appointment.State,
	)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("transition appointment: update: %w", err)
	}
	if channel != nil && address != nil {
		appointment.Contact = domain.Contact{Channel: *channel, Address: *address}
	}

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO appointment_events (clinic_id, appointment_id, from_state, to_state, actor_id, reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		appointment.ClinicID, appointment.ID, currentState, to, actorID, reasonPtr,
	)
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("transition appointment: insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Appointment{}, fmt.Errorf("transition appointment: commit: %w", err)
	}

	return appointment, nil
}

func (s *Store) QueueForSession(ctx context.Context, clinicID, sessionID int64) ([]domain.Appointment, error) {
	query := "SELECT id, clinic_id, session_id, token_no, patient_name, contact_channel, contact_address, queued_at, priority, state FROM appointments WHERE session_id = $1 AND clinic_id = $2 AND state IN ('waiting', 'in_consultation') ORDER BY CASE WHEN state = 'in_consultation' THEN 0 ELSE 1 END, priority DESC, queued_at ASC"
	rows, err := s.pool.Query(ctx, query, sessionID, clinicID)

	if err != nil {
		return nil, fmt.Errorf("queue for session: %w", err)
	}

	defer rows.Close()

	var appointments []domain.Appointment

	for rows.Next() {
		var a domain.Appointment
		var channel, address *string
		err := rows.Scan(&a.ID, &a.ClinicID, &a.SessionID, &a.TokenNo, &a.PatientName, &channel, &address, &a.QueuedAt, &a.Priority, &a.State)
		if err != nil {
			return nil, fmt.Errorf("queue for session: %w", err)
		}
		if channel != nil && address != nil {
			a.Contact = domain.Contact{Channel: *channel, Address: *address}
		}
		appointments = append(appointments, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("queue for session: %w", err)
	}

	return appointments, nil
}

func (s *Store) CreateStaffUser(ctx context.Context, clinicID int64, name, email, passwordHash string, role domain.Role) (domain.StaffUser, error) {
	user := domain.StaffUser{
		ClinicID:     clinicID,
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
	}

	err := s.pool.QueryRow(ctx,
		`INSERT INTO staff_users (clinic_id, name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		clinicID, name, email, passwordHash, role,
	).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return domain.StaffUser{}, fmt.Errorf("create staff user: %w", err)
	}

	return user, nil
}

func (s *Store) GetStaffUserByEmail(ctx context.Context, email string) (domain.StaffUser, error) {
	var user domain.StaffUser
	err := s.pool.QueryRow(ctx,
		`SELECT id, clinic_id, name, email, password_hash, role, created_at
		 FROM staff_users WHERE email = $1`, email,
	).Scan(&user.ID, &user.ClinicID, &user.Name, &user.Email,
		&user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StaffUser{}, domain.ErrInvalidCredentials
		}
		return domain.StaffUser{}, fmt.Errorf("get staff user by email: %w", err)
	}
	return user, nil
}

func (s *Store) CreateAuthSession(ctx context.Context, tokenHash []byte, userID, clinicID int64, csrfToken string, expiresAt time.Time) (domain.AuthSession, error) {
	session := domain.AuthSession{
		UserID:    userID,
		ClinicID:  clinicID,
		CSRFToken: csrfToken,
		ExpiresAt: expiresAt,
	}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO auth_sessions (token_hash, user_id, clinic_id, csrf_token, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		tokenHash, userID, clinicID, csrfToken, expiresAt,
	).Scan(&session.ID, &session.CreatedAt)
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("create auth session: %w", err)
	}
	return session, nil
}

func (s *Store) GetAuthSession(ctx context.Context, tokenHash []byte) (domain.AuthSession, error) {
	var sess domain.AuthSession
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, clinic_id, csrf_token, created_at, expires_at
		 FROM auth_sessions WHERE token_hash = $1 AND expires_at > now();`, tokenHash,
	).Scan(&sess.ID, &sess.UserID, &sess.ClinicID, &sess.CSRFToken,
		&sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AuthSession{}, domain.ErrUnauthorized
		}
		return domain.AuthSession{}, fmt.Errorf("get auth session: %w", err)
	}
	return sess, nil
}

func (s *Store) DeleteAuthSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM auth_sessions WHERE token_hash = $1", tokenHash)
	if err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}
	return nil
}

func (s *Store) GetStaffUserByID(ctx context.Context, clinicID, userID int64) (domain.StaffUser, error) {
	var user domain.StaffUser
	err := s.pool.QueryRow(ctx,
		`SELECT id, clinic_id, name, email, password_hash, role, created_at
		 FROM staff_users WHERE id = $1 AND clinic_id = $2`, userID, clinicID,
	).Scan(&user.ID, &user.ClinicID, &user.Name, &user.Email,
		&user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StaffUser{}, domain.ErrUnauthorized
		}
		return domain.StaffUser{}, fmt.Errorf("get staff user by id: %w", err)
	}
	return user, nil
}

func (s *Store) SessionsForDate(ctx context.Context, clinicID int64, date time.Time) ([]domain.SessionWithDoctor, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.id, s.clinic_id, s.doctor_id, s.session_date, s.starts_at, s.ends_at,
		        s.capacity, s.delay_min, s.status, s.version, s.avg_consult_sec, d.name
		 FROM sessions s
		 JOIN doctors d ON d.id = s.doctor_id
		 WHERE s.clinic_id = $1 AND s.session_date = $2
		 ORDER BY s.starts_at`, clinicID, date)
	if err != nil {
		return nil, fmt.Errorf("sessions for date: %w", err)
	}
	defer rows.Close()

	sessions := []domain.SessionWithDoctor{}
	for rows.Next() {
		var x domain.SessionWithDoctor
		if err := rows.Scan(&x.ID, &x.ClinicID, &x.DoctorID, &x.SessionDate,
			&x.StartsAt, &x.EndsAt, &x.Capacity, &x.DelayMin, &x.Status,
			&x.Version, &x.AvgConsultSec, &x.DoctorName); err != nil {
			return nil, fmt.Errorf("sessions for date: %w", err)
		}
		sessions = append(sessions, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sessions for date: %w", err)
	}
	return sessions, nil
}

func (s *Store) DoctorsForClinic(ctx context.Context, clinicID int64) ([]domain.Doctor, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, clinic_id, name FROM doctors WHERE clinic_id = $1 ORDER BY name", clinicID)
	if err != nil {
		return nil, fmt.Errorf("doctors for clinic: %w", err)
	}
	defer rows.Close()

	doctors := []domain.Doctor{}
	for rows.Next() {
		var d domain.Doctor
		if err := rows.Scan(&d.ID, &d.ClinicID, &d.Name); err != nil {
			return nil, fmt.Errorf("doctors for clinic: %w", err)
		}
		doctors = append(doctors, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("doctors for clinic: %w", err)
	}
	return doctors, nil
}

func (s *Store) updateSession(ctx context.Context, op, setClause string, args ...any) (domain.Session, error) {
	query := `UPDATE sessions SET ` + setClause + `, version = version + 1
		 WHERE id = $2 AND clinic_id = $3 AND version = $4
		 RETURNING id, clinic_id, doctor_id, session_date, starts_at, ends_at,
		           capacity, delay_min, status, version, avg_consult_sec`

	var x domain.Session
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&x.ID, &x.ClinicID, &x.DoctorID, &x.SessionDate, &x.StartsAt, &x.EndsAt,
		&x.Capacity, &x.DelayMin, &x.Status, &x.Version, &x.AvgConsultSec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrVersionConflict
		}
		return domain.Session{}, fmt.Errorf("%s: %w", op, err)
	}
	return x, nil
}

func (s *Store) SetSessionDelay(ctx context.Context, clinicID, sessionID int64, delayMin, version int) (domain.Session, error) {
	return s.updateSession(ctx, "set session delay", "delay_min = $1",
		delayMin, sessionID, clinicID, version)
}

func (s *Store) CloseSession(ctx context.Context, clinicID, sessionID int64, version int) (domain.Session, error) {
	return s.updateSession(ctx, "close session", "status = $1",
		domain.Closed, sessionID, clinicID, version)
}
