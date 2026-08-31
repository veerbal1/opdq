package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veerbal/opdq/internal/config"
)

func run() error {
	cfg, err := config.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}

	err = pool.Ping(ctx)
	if err != nil {
		return err
	}
	slog.Info("connected to database")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Ok")
	})

	server := &http.Server{
		Handler:           mux,
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
	}

	go func() {
		server.ListenAndServe()
	}()

	return nil
}

func main() {
	err := run()
	if err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}
