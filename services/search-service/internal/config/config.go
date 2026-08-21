package config

import "os"

type Config struct {
	Port           string
	OpenSearchURL  string
	KafkaBrokers   string
	AuthServiceURL string
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8088"),
		OpenSearchURL:  getEnv("OPENSEARCH_URL", "http://localhost:9201"),
		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9192"),
		AuthServiceURL: getEnv("AUTH_SERVICE_URL", "http://localhost:8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
