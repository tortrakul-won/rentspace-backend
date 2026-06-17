package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	CORSOrigins    []string
	GotenbergURL   string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:         getEnv("SERVER_PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		CORSOrigins:  parseCORSOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		GotenbergURL: getEnv("GOTENBERG_URL", "http://localhost:3000"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCORSOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(o); s != "" {
			origins = append(origins, s)
		}
	}
	return origins
}
