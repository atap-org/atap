package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/api"
	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
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

	// Request logging
	app.Use(func(c *fiber.Ctx) error {
		start := zerolog.TimestampFunc()
		err := c.Next()
		log.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", zerolog.TimestampFunc().Sub(start)).
			Msg("request")
		return err
	})

	// Routes
	handler := api.NewHandler(db, db, db, rdb, platformPriv, cfg, log)
	handler.SetupRoutes(app)

	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/v1/health")
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

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
