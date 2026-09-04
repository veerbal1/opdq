package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL  string
	Port         string
	CookieSecure bool
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if len(strings.TrimSpace(databaseURL)) == 0 {
		return Config{}, fmt.Errorf("DATABASE_URL is missing")
	}

	port := os.Getenv("PORT")
	if len(strings.TrimSpace(port)) == 0 {
		port = "8080"
	}

	return Config{
		DatabaseURL:  databaseURL,
		Port:         port,
		CookieSecure: os.Getenv("COOKIE_SECURE") == "true",
	}, nil
}
