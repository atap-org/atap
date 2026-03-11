package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/api"
	"github.com/atap-dev/atap/platform/internal/config"
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

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse Redis URL")
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(rdb.Options().Context).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer rdb.Close()
	log.Info().Msg("connected to Redis")

	// Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "ATAP Platform",
		ServerHeader: "ATAP",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"type":   "https://atap.dev/errors/internal",
				"title":  "Internal Server Error",
				"status": code,
				"detail": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, Last-Event-ID",
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
	handler := api.NewHandler(db, rdb, cfg, log)
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
