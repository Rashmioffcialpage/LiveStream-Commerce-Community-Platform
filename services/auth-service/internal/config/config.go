package config

import (
	"os"
	"time"
)

type Config struct {
	Port                string
	DatabaseURL          string
	JWTSecret            string
	AccessTokenTTL       time.Duration
	RefreshTokenTTL      time.Duration
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURL    string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://auth:auth@localhost:5433/auth?sslmode=disable"),
		JWTSecret:          getEnv("JWT_SECRET", "dev-secret-change-me"),
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    30 * 24 * time.Hour,
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/oauth/google/callback"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
