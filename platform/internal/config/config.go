package config

import (
	"crypto/ecdh"
	"os"
	"time"
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

	// Approval engine: maximum TTL for approval valid_until per spec §8.5.
	// Default: 2160h (90 days). Discovery publishes "P90D" as ISO 8601 duration.
	MaxApprovalTTL time.Duration

	// Platform DIDComm identity (server acts as trusted "via" participant per MSG-03).
	// Set programmatically at startup from derived Ed25519 seed — not loaded from env vars.
	// In production (Phase 4), store in HSM or secure key store.
	PlatformX25519PrivateKey *ecdh.PrivateKey
	PlatformX25519PublicKey  *ecdh.PublicKey
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	maxApprovalTTL := parseMaxApprovalTTL(envOrDefault("MAX_APPROVAL_TTL", "2160h"))
	return &Config{
		Port:           envOrDefault("PORT", "8080"),
		Host:           envOrDefault("HOST", "0.0.0.0"),
		DatabaseURL:    envOrDefault("DATABASE_URL", "postgres://atap:atap@localhost:5432/atap?sslmode=disable"),
		RedisURL:       envOrDefault("REDIS_URL", "redis://localhost:6379"),
		PlatformDomain: envOrDefault("PLATFORM_DOMAIN", "atap.app"),
		MigrationsPath: envOrDefault("MIGRATIONS_PATH", "migrations"),
		MaxApprovalTTL: maxApprovalTTL,
	}
}

// parseMaxApprovalTTL parses a Go duration string for MAX_APPROVAL_TTL.
// Falls back to 90 days (2160h) on parse error.
func parseMaxApprovalTTL(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 2160 * time.Hour // 90 days default
	}
	return d
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
