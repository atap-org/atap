package config

import (
	"crypto/ecdh"
	"os"
)

// Config holds Phase 2 configuration values.
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

	// Platform DIDComm identity (server acts as trusted "via" participant per MSG-03).
	// Set programmatically at startup from derived Ed25519 seed — not loaded from env vars.
	// In production (Phase 4), store in HSM or secure key store.
	PlatformX25519PrivateKey *ecdh.PrivateKey
	PlatformX25519PublicKey  *ecdh.PublicKey
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
