package config

import "os"

type Config struct {
	Port                   string
	DatabaseURL            string
	RedisURL               string
	JWTSecret              string
	KafkaBrokers           string
	AuthServiceURL         string
	SubscriptionServiceURL string
}

func Load() Config {
	return Config{
		Port:                   getEnv("PORT", "8087"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://notification:notification@localhost:5439/notification?sslmode=disable"),
		RedisURL:               getEnv("REDIS_URL", "redis://localhost:6383/0"),
		JWTSecret:              getEnv("JWT_SECRET", "dev-secret-change-me"),
		KafkaBrokers:           getEnv("KAFKA_BROKERS", "localhost:9192"),
		AuthServiceURL:         getEnv("AUTH_SERVICE_URL", "http://localhost:8080"),
		SubscriptionServiceURL: getEnv("SUBSCRIPTION_SERVICE_URL", "http://localhost:8083"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
