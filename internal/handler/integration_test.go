package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/veerbal/opdq/internal/handler"
	"github.com/veerbal/opdq/internal/store"
)

var (
	testPool      *pgxpool.Pool
	testAdminPool *pgxpool.Pool
	testMux       *http.ServeMux
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:18.6-alpine",
		postgres.WithDatabase("opd_db"),
		postgres.WithUsername("opd"),
		postgres.WithPassword("12345678"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)))
	if err != nil {
		panic(err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(err)
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		panic(err)
	}

	db, err := goose.OpenDBWithDriver("postgres", connStr)
	if err != nil {
		panic(err)
	}
	if _, err := db.Exec("CREATE ROLE opd_app LOGIN PASSWORD 'app87654321'"); err != nil {
		panic(err)
	}
	if err := goose.Up(db, "../../migrations"); err != nil {
		panic(err)
	}
	db.Close()

	appConnStr := fmt.Sprintf("postgres://opd_app:app87654321@%s:%s/opd_db?sslmode=disable", host, port.Port())
	testPool, err = pgxpool.New(ctx, appConnStr)
	if err != nil {
		panic(err)
	}

	testAdminPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		panic(err)
	}

	h := handler.NewHandler(store.NewStore(testPool))
	testMux = h.Routes()

	code := m.Run()

	testPool.Close()
	testAdminPool.Close()
	pgContainer.Terminate(ctx)

	os.Exit(code)
}

func resetDB(t *testing.T) {
	_, err := testAdminPool.Exec(context.Background(),
		"TRUNCATE clinics, doctors, sessions, appointments, appointment_events, staff_users, auth_sessions RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatal(err)
	}
}
