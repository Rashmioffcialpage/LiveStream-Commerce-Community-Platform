package config

import "os"

type Config struct {
	Port                   string
	DatabaseURL            string
	JWTSecret              string
	KafkaBrokers           string
	StreamServiceURL       string
	PaymentServiceURL      string
	SubscriptionPriceCents int
}

func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8083"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://subscription:subscription@localhost:5436/subscription?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-me"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9192"),
		StreamServiceURL:  getEnv("STREAM_SERVICE_URL", "http://localhost:8081"),
		PaymentServiceURL: getEnv("PAYMENT_SERVICE_URL", "http://localhost:8084"),
		// flat price for every channel, on purpose -- tiered pricing is a
		// real feature (Twitch has Tier 1/2/3) but out of scope for
		// proving the subscribe -> charge -> event -> dashboard mechanism
		SubscriptionPriceCents: 499,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
