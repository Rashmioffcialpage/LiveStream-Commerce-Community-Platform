package config

import "os"

type Config struct {
	Port         string
	DatabaseURL  string
	RedisURL     string
	JWTSecret    string
	KafkaBrokers string

	// S3-compatible object storage for stream recordings. Points at MinIO
	// locally; in production this is just AWS S3 with S3Endpoint unset
	// (the SDK defaults to the real AWS endpoints) -- see internal/storage.
	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8081"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://stream:stream@localhost:5434/stream?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6381/0"),
		// must match auth-service's JWT_SECRET -- stream-service verifies
		// tokens it never issues, it doesn't mint its own.
		JWTSecret: getEnv("JWT_SECRET", "dev-secret-change-me"),

		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9192"),

		S3Endpoint:       os.Getenv("S3_ENDPOINT"), // empty = real AWS S3
		S3PublicEndpoint: getEnv("S3_PUBLIC_ENDPOINT", "http://localhost:9002"),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:      getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:         getEnv("S3_BUCKET", "recordings"),
		S3Region:         getEnv("S3_REGION", "us-east-1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
