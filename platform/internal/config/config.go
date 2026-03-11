package config

import (
	"os"
)

// Config holds Phase 1 configuration values.
type Config struct {
	// Server
	Port string
	Host string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Platform identity
	PlatformDomain string

	// Migrations
	MigrationsPath string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:           envOrDefault("PORT", "8080"),
		Host:           envOrDefault("HOST", "0.0.0.0"),
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://atap:atap@localhost:5432/atap?sslmode=disable"),
		RedisURL:       envOrDefault("REDIS_URL", "redis://localhost:6379"),
		PlatformDomain: envOrDefault("PLATFORM_DOMAIN", "atap.app"),
		MigrationsPath: envOrDefault("MIGRATIONS_PATH", "migrations"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
