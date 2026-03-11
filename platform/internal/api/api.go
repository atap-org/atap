package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// EntityStore defines the data access methods required by the API layer.
type EntityStore interface {
	CreateEntity(ctx context.Context, e *models.Entity) error
	GetEntity(ctx context.Context, id string) (*models.Entity, error)
	GetEntityByTokenHash(ctx context.Context, hash []byte) (*models.Entity, error)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store  EntityStore
	config *config.Config
	log    zerolog.Logger
}

// NewHandler creates a new Handler with all dependencies.
func NewHandler(s EntityStore, cfg *config.Config, log zerolog.Logger) *Handler {
	return &Handler{store: s, config: cfg, log: log}
}

// SetupRoutes configures all API routes.
func (h *Handler) SetupRoutes(app *fiber.App) {
	v1 := app.Group("/v1")

	// Health
	v1.Get("/health", h.Health)

	// Registration (no auth)
	v1.Post("/register", h.RegisterAgent)

	// Entities (no auth)
	v1.Get("/entities/:entityId", h.GetEntity)

	// Authenticated endpoints
	auth := v1.Group("", h.AuthMiddleware())
	auth.Get("/me", h.GetMe)
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
// REGISTRATION
// ============================================================

// RegisterAgent creates a new agent entity with generated keypair and token.
func (h *Handler) RegisterAgent(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	// Generate keypair
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		return problem(c, 500, "key_generation_failed", "Failed to generate keypair", "")
	}

	// Generate IDs
	entityID := crypto.NewEntityID()
	keyID := crypto.NewKeyID(entityID[:8])
	token, tokenHash := crypto.NewToken()

	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		URI:              fmt.Sprintf("agent://%s", entityID),
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             req.Name,
		TrustLevel:       models.TrustLevel0,
		TokenHash:        tokenHash,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        time.Now().UTC(),
	}

	if err := h.store.CreateEntity(c.Context(), entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create entity")
		return problem(c, 500, "creation_failed", "Failed to create entity", "")
	}

	h.log.Info().Str("entity_id", entityID).Str("type", "agent").Msg("entity registered")

	return c.Status(201).JSON(models.RegisterResponse{
		URI:        entity.URI,
		ID:         entityID,
		Token:      token,
		PublicKey:  crypto.EncodePublicKey(pubKey),
		PrivateKey: crypto.EncodePrivateKey(privKey),
		KeyID:      keyID,
	})
}

// ============================================================
// ENTITIES
// ============================================================

// GetEntity returns the public view of an entity.
func (h *Handler) GetEntity(c *fiber.Ctx) error {
	entityID := c.Params("entityId")

	entity, err := h.store.GetEntity(c.Context(), entityID)
	if err != nil {
		return problem(c, 500, "query_failed", "Failed to get entity", "")
	}
	if entity == nil {
		return problem(c, 404, "not_found", "Entity not found", "")
	}

	return c.JSON(models.EntityLookupResponse{
		ID:         entity.ID,
		Type:       entity.Type,
		URI:        entity.URI,
		PublicKey:  crypto.EncodePublicKey(entity.PublicKeyEd25519),
		KeyID:      entity.KeyID,
		Name:       entity.Name,
		TrustLevel: entity.TrustLevel,
		Registry:   entity.Registry,
		CreatedAt:  entity.CreatedAt,
	})
}

// GetMe returns the authenticated entity's public info.
func (h *Handler) GetMe(c *fiber.Ctx) error {
	entity := c.Locals("entity").(*models.Entity)

	return c.JSON(models.EntityLookupResponse{
		ID:         entity.ID,
		Type:       entity.Type,
		URI:        entity.URI,
		PublicKey:  crypto.EncodePublicKey(entity.PublicKeyEd25519),
		KeyID:      entity.KeyID,
		Name:       entity.Name,
		TrustLevel: entity.TrustLevel,
		Registry:   entity.Registry,
		CreatedAt:  entity.CreatedAt,
	})
}

// ============================================================
// AUTH MIDDLEWARE
// ============================================================

// AuthMiddleware validates Bearer tokens via SHA-256 hash lookup.
func (h *Handler) AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return problem(c, 401, "unauthorized", "Missing Authorization header", "")
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			return problem(c, 401, "unauthorized", "Invalid Authorization format, use Bearer", "")
		}

		hash := crypto.HashToken(token)
		entity, err := h.store.GetEntityByTokenHash(c.Context(), hash)
		if err != nil {
			return problem(c, 500, "auth_error", "Authentication failed", "")
		}
		if entity == nil {
			return problem(c, 401, "unauthorized", "Invalid token", "")
		}

		c.Locals("entity", entity)
		return c.Next()
	}
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
