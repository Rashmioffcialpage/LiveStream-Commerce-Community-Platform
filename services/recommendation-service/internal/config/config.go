package config

import "os"

type Config struct {
	Port             string
	RedisURL         string
	KafkaBrokers     string
	StreamServiceURL string
	JWTSecret        string
}

func Load() Config {
	return Config{
		Port:             getEnv("PORT", "8089"),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6384/0"),
		KafkaBrokers:     getEnv("KAFKA_BROKERS", "localhost:9192"),
		StreamServiceURL: getEnv("STREAM_SERVICE_URL", "http://localhost:8081"),
		JWTSecret:        getEnv("JWT_SECRET", "dev-secret-change-me"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
