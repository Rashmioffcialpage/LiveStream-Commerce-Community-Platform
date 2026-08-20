package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8081"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://stream:stream@localhost:5434/stream?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6381/0"),
		// must match auth-service's JWT_SECRET -- stream-service verifies
		// tokens it never issues, it doesn't mint its own.
		JWTSecret: getEnv("JWT_SECRET", "dev-secret-change-me"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
