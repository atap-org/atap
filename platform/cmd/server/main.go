package main

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/hkdf"

	"github.com/atap-dev/atap/platform/internal/api"
	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

func main() {
	// Logger
	log := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "atap-platform").
		Logger()

	// Config
	cfg := config.Load()

	// Database
	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()
	log.Info().Msg("connected to PostgreSQL")

	// Run migrations
	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}
	log.Info().Msg("migrations applied")

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse Redis URL")
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer rdb.Close()
	log.Info().Msg("connected to Redis")

	// Generate platform signing key (used for OAuth token signing)
	platformPub, platformPriv, err := crypto.GenerateKeyPair()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to generate platform signing key")
	}
	log.Info().Str("public_key", crypto.EncodePublicKey(platformPub)).Msg("platform signing key generated")

	// Derive server X25519 key from platform Ed25519 seed for stability across restarts.
	// In production (Phase 4), this should be stored in a HSM or secure key store.
	hkdfReader := hkdf.New(sha256.New, platformPriv.Seed(), []byte("atap-platform-x25519"), nil)
	platformX25519Priv, err := ecdh.X25519().GenerateKey(hkdfReader)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to derive platform X25519 key")
	}
	cfg.PlatformX25519PrivateKey = platformX25519Priv
	cfg.PlatformX25519PublicKey = platformX25519Priv.PublicKey()
	log.Info().
		Str("server_did", fmt.Sprintf("did:web:%s:server:platform", cfg.PlatformDomain)).
		Msg("server DIDComm identity ready")

	// Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "ATAP Platform",
		ServerHeader: "ATAP",
		BodyLimit:    128 * 1024, // 128KB
		ErrorHandler: api.GlobalErrorHandler,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, DPoP",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// Request logging (skip health checks to reduce noise)
	app.Use(func(c *fiber.Ctx) error {
		start := zerolog.TimestampFunc()
		err := c.Next()
		if c.Path() == "/v1/health" {
			return err
		}

		evt := log.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", zerolog.TimestampFunc().Sub(start)).
			Str("ip", c.IP()).
			Str("user_agent", c.Get("User-Agent"))

		// Add entity context from auth middleware (set on authenticated routes)
		if entity, ok := c.Locals("entity").(*models.Entity); ok && entity != nil {
			evt = evt.
				Str("entity_id", entity.ID).
				Str("entity_type", entity.Type).
				Str("did", entity.DID)
		}

		evt.Msg("request")
		return err
	})

	// Routes
	handler := api.NewHandler(db, db, db, db, db, db, db, db, db, rdb, platformPriv, platformX25519Priv, cfg, log)

	// Rate limit config: load from DB and start background refresh (API-07)
	rateLimitCtx, rateLimitCancel := context.WithCancel(context.Background())
	defer rateLimitCancel()
	rateLimitCfg := api.StartRateLimitConfigRefresh(rateLimitCtx, db, log)
	handler.SetRateLimitConfig(rateLimitCfg)

	handler.SetupRoutes(app)

	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/v1/health")
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Revocation expiry cleanup (removes expired revocation entries every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := db.CleanupExpiredRevocations(context.Background())
				if err != nil {
					log.Error().Err(err).Msg("revocation cleanup failed")
				} else if n > 0 {
					log.Info().Int64("removed", n).Msg("cleaned up expired revocations")
				}
			case <-quit:
				return
			}
		}
	}()

	go func() {
		<-quit
		log.Info().Msg("shutting down...")
		app.Shutdown()
	}()

	// Start
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Info().Str("addr", addr).Msg("ATAP platform starting")

	if err := app.Listen(addr); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}

// runMigrations applies database migrations from the given path.
func runMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
