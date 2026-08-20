package config

import "os"

type Config struct {
	Port             string
	DatabaseURL      string
	RedisURL         string
	JWTSecret        string
	KafkaBrokers     string
	StreamServiceURL string
}

func Load() Config {
	return Config{
		Port:             getEnv("PORT", "8082"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://chat:chat@localhost:5435/chat?sslmode=disable"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6382/0"),
		JWTSecret:        getEnv("JWT_SECRET", "dev-secret-change-me"),
		KafkaBrokers:     getEnv("KAFKA_BROKERS", "localhost:9192"),
		StreamServiceURL: getEnv("STREAM_SERVICE_URL", "http://localhost:8081"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
