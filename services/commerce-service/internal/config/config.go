package config

import "os"

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	KafkaBrokers      string
	StreamServiceURL  string
	PaymentServiceURL string
	// 1 coin = 1 cent, on purpose -- a real product would sell coin
	// bundles at a markup; this just needs a deterministic, easy-to-verify
	// conversion for testing the buy -> spend -> earn chain.
	CentsPerCoin int
}

func Load() Config {
	return Config{
		Port:              getEnv("PORT", "8086"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://commerce:commerce@localhost:5438/commerce?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-me"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9192"),
		StreamServiceURL:  getEnv("STREAM_SERVICE_URL", "http://localhost:8081"),
		PaymentServiceURL: getEnv("PAYMENT_SERVICE_URL", "http://localhost:8084"),
		CentsPerCoin:      1,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
