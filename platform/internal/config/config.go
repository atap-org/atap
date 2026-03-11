package config

import (
	"os"
)

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

	// SIMRelay
	SIMRelayAPIURL       string
	SIMRelayAPIKey       string
	SIMRelayWebhookSecret string
	SIMRelayNumber       string

	// World ID
	WorldIDAppID        string
	WorldIDRPID         string
	WorldIDRPSigningKey string

	// Firebase (Push)
	FCMCredentialsJSON string
}

func Load() *Config {
	return &Config{
		Port:                  envOrDefault("PORT", "8080"),
		Host:                  envOrDefault("HOST", "0.0.0.0"),
		DatabaseURL:           envOrDefault("DATABASE_URL", "postgres://atap:atap@localhost:5432/atap?sslmode=disable"),
		RedisURL:              envOrDefault("REDIS_URL", "redis://localhost:6379"),
		PlatformDomain:        envOrDefault("PLATFORM_DOMAIN", "atap.app"),
		SIMRelayAPIURL:        os.Getenv("SIMRELAY_API_URL"),
		SIMRelayAPIKey:        os.Getenv("SIMRELAY_API_KEY"),
		SIMRelayWebhookSecret: os.Getenv("SIMRELAY_WEBHOOK_SECRET"),
		SIMRelayNumber:        os.Getenv("SIMRELAY_NUMBER"),
		WorldIDAppID:          os.Getenv("WORLDID_APP_ID"),
		WorldIDRPID:           os.Getenv("WORLDID_RP_ID"),
		WorldIDRPSigningKey:   os.Getenv("WORLDID_RP_SIGNING_KEY"),
		FCMCredentialsJSON:    os.Getenv("FCM_CREDENTIALS_JSON"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
