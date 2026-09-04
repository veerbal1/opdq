package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerbal/opdq/internal/auth"
	"github.com/veerbal/opdq/internal/config"
	"github.com/veerbal/opdq/internal/domain"
	"github.com/veerbal/opdq/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	email := os.Getenv("SEED_EMAIL")
	password := os.Getenv("SEED_PASSWORD")
	clinicName := os.Getenv("SEED_CLINIC_NAME")
	doctorName := os.Getenv("SEED_DOCTOR_NAME")
	adminName := os.Getenv("SEED_ADMIN_NAME")
	if adminName == "" {
		adminName = "Admin"
	}

	if email == "" || password == "" || clinicName == "" || doctorName == "" {
		return errors.New("SEED_EMAIL, SEED_PASSWORD, SEED_CLINIC_NAME and SEED_DOCTOR_NAME are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	var existingID int64
	err = pool.QueryRow(ctx, "SELECT id FROM staff_users WHERE email = $1", email).Scan(&existingID)
	if err == nil {
		slog.Info("already seeded, nothing to do", "email", email, "user_id", existingID)
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check existing user: %w", err)
	}

	s := store.NewStore(pool)

	clinic, err := s.CreateClinic(ctx, clinicName)
	if err != nil {
		return err
	}

	doctor, err := s.CreateDoctor(ctx, doctorName, clinic.ID)
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	user, err := s.CreateStaffUser(ctx, clinic.ID, adminName, email, hash, domain.RoleAdmin)
	if err != nil {
		return err
	}

	slog.Info("seeded",
		"clinic_id", clinic.ID, "doctor_id", doctor.ID,
		"user_id", user.ID, "email", email)
	return nil
}
