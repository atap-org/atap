package api

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/models"
)

// EntityStore defines the data access methods required by the API layer.
type EntityStore interface {
	CreateEntity(ctx context.Context, e *models.Entity) error
	GetEntity(ctx context.Context, id string) (*models.Entity, error)
	GetEntityByKeyID(ctx context.Context, keyID string) (*models.Entity, error)
	GetEntityByDID(ctx context.Context, did string) (*models.Entity, error)
	DeleteEntity(ctx context.Context, id string) error
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	entityStore EntityStore
	config      *config.Config
	redis       *redis.Client
	platformKey ed25519.PrivateKey
	log         zerolog.Logger
}

// NewHandler creates a new Handler with all dependencies.
func NewHandler(
	es EntityStore,
	rdb *redis.Client,
	platformKey ed25519.PrivateKey,
	cfg *config.Config,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		entityStore: es,
		config:      cfg,
		redis:       rdb,
		platformKey: platformKey,
		log:         log,
	}
}

// SetupRoutes configures all API routes.
// Additional routes will be added in Plans 02-04.
func (h *Handler) SetupRoutes(app *fiber.App) {
	v1 := app.Group("/v1")

	// Health
	v1.Get("/health", h.Health)
}

// ============================================================
// HEALTH
// ============================================================

// Health returns the platform health status.
func (h *Handler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":   "ok",
		"protocol": "ATAP",
		"version":  "0.1",
		"time":     time.Now().UTC().Format(time.RFC3339),
	})
}

// ============================================================
// ERROR HELPERS
// ============================================================

func problem(c *fiber.Ctx, status int, errType, title, detail string) error {
	return c.Status(status).JSON(models.ProblemDetail{
		Type:     fmt.Sprintf("https://atap.dev/errors/%s", errType),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Path(),
	})
}
