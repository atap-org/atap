package api

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"errors"
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

// KeyVersionStore defines the data access methods for key version management.
type KeyVersionStore interface {
	CreateKeyVersion(ctx context.Context, kv *models.KeyVersion) error
	GetActiveKeyVersion(ctx context.Context, entityID string) (*models.KeyVersion, error)
	GetKeyVersions(ctx context.Context, entityID string) ([]models.KeyVersion, error)
	RotateKey(ctx context.Context, entityID string, newPubKey []byte) (*models.KeyVersion, error)
}

// MessageStore defines the data access methods for DIDComm offline message delivery.
type MessageStore interface {
	QueueMessage(ctx context.Context, msg *models.DIDCommMessage) error
	GetPendingMessages(ctx context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error)
	MarkDelivered(ctx context.Context, messageIDs []string) error
	CleanupExpiredMessages(ctx context.Context) (int64, error)
}

// ApprovalStore defines the data access methods for the approval engine.
type ApprovalStore interface {
	CreateApproval(ctx context.Context, a *models.Approval) error
	GetApproval(ctx context.Context, id string) (*models.Approval, error)
	UpdateApprovalState(ctx context.Context, id, state string, respondedAt *time.Time) error
	ConsumeApproval(ctx context.Context, id string) (bool, error)
	ListApprovals(ctx context.Context, entityDID string, limit, offset int) ([]models.Approval, error)
	RevokeWithChildren(ctx context.Context, parentID string) error
	CleanupExpiredApprovals(ctx context.Context) (int64, error)
}

// OAuthTokenStore defines the data access methods for OAuth 2.1 tokens and auth codes.
type OAuthTokenStore interface {
	CreateOAuthToken(ctx context.Context, token *models.OAuthToken) error
	GetOAuthToken(ctx context.Context, tokenID string) (*models.OAuthToken, error)
	RevokeOAuthToken(ctx context.Context, tokenID string) error
	CreateAuthCode(ctx context.Context, code *models.OAuthAuthCode) error
	RedeemAuthCode(ctx context.Context, code string) (*models.OAuthAuthCode, error)
	CleanupExpiredTokens(ctx context.Context) (int64, error)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	entityStore       EntityStore
	keyVersionStore   KeyVersionStore
	oauthTokenStore   OAuthTokenStore
	messageStore      MessageStore
	approvalStore     ApprovalStore
	config            *config.Config
	redis             *redis.Client
	platformKey       ed25519.PrivateKey
	platformX25519Key *ecdh.PrivateKey
	log               zerolog.Logger
}

// NewHandler creates a new Handler with all dependencies.
func NewHandler(
	es EntityStore,
	kvs KeyVersionStore,
	ots OAuthTokenStore,
	ms MessageStore,
	as ApprovalStore,
	rdb *redis.Client,
	platformKey ed25519.PrivateKey,
	platformX25519Key *ecdh.PrivateKey,
	cfg *config.Config,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		entityStore:       es,
		keyVersionStore:   kvs,
		oauthTokenStore:   ots,
		messageStore:      ms,
		approvalStore:     as,
		config:            cfg,
		redis:             rdb,
		platformKey:       platformKey,
		platformX25519Key: platformX25519Key,
		log:               log,
	}
}

// SetupRoutes configures all API routes.
func (h *Handler) SetupRoutes(app *fiber.App) {
	// Discovery (outside /v1/ per ATAP spec)
	app.Get("/.well-known/atap.json", h.Discovery)

	// Server DID Document (did:web:{domain}:server:platform)
	app.Get("/server/platform/did.json", h.ResolveServerDID)

	// DID Document resolution per did:web spec (outside /v1/, before v1 group)
	app.Get("/:type/:id/did.json", h.ResolveDID)

	v1 := app.Group("/v1")

	// Health
	v1.Get("/health", h.Health)

	// Entity registration (public — no auth required)
	v1.Post("/entities", h.CreateEntity)
	v1.Get("/entities/:entityId", h.GetEntity)

	// OAuth 2.1 endpoints (public — no auth on these, DPoP is handled inline)
	v1.Post("/oauth/token", h.Token)
	v1.Get("/oauth/authorize", h.Authorize)

	// DIDComm endpoint (public — no OAuth/DPoP, DIDComm is self-authenticating via ECDH-1PU)
	// TODO Phase 4: add IP-based rate limiting to prevent abuse.
	v1.Post("/didcomm", h.HandleDIDComm)

	// Authenticated routes — require DPoP-bound access token
	auth := v1.Group("", h.DPoPAuthMiddleware())
	auth.Delete("/entities/:entityId", h.RequireScope("atap:manage"), h.DeleteEntity)
	auth.Post("/entities/:entityId/keys/rotate", h.RequireScope("atap:manage"), h.RotateKey)

	// DIDComm inbox (authenticated — requires DPoP + atap:inbox scope)
	auth.Get("/didcomm/inbox", h.RequireScope("atap:inbox"), h.HandleInbox)

	// Approvals (authenticated — require DPoP + atap:approve scope)
	auth.Post("/approvals", h.RequireScope("atap:approve"), h.CreateApproval)
	auth.Post("/approvals/:approvalId/respond", h.RequireScope("atap:approve"), h.RespondApproval)
	auth.Get("/approvals", h.RequireScope("atap:approve"), h.ListApprovals)
	auth.Get("/approvals/:approvalId", h.RequireScope("atap:approve"), h.GetApproval)
	auth.Delete("/approvals/:approvalId", h.RequireScope("atap:approve"), h.RevokeApproval)

	// Approval status check (public per spec — verifiers need this)
	v1.Get("/approvals/:approvalId/status", h.GetApprovalStatus)
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

const mimeApplicationProblemJSON = "application/problem+json"

// problem writes an RFC 7807 Problem Details response.
// Content-Type is set to application/problem+json per RFC 7807 Section 3.
func problem(c *fiber.Ctx, status int, errType, title, detail string) error {
	return c.Status(status).JSON(models.ProblemDetail{
		Type:     fmt.Sprintf("https://atap.dev/errors/%s", errType),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: c.Path(),
	}, mimeApplicationProblemJSON)
}

// GlobalErrorHandler is the Fiber global error handler that produces RFC 7807 responses.
// Register via fiber.Config{ErrorHandler: api.GlobalErrorHandler}.
func GlobalErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	errType := "internal"
	title := "Internal Server Error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		switch code {
		case fiber.StatusNotFound:
			errType = "not-found"
			title = "Not Found"
		case fiber.StatusBadRequest:
			errType = "bad-request"
			title = "Bad Request"
		case fiber.StatusUnauthorized:
			errType = "unauthorized"
			title = "Unauthorized"
		case fiber.StatusForbidden:
			errType = "forbidden"
			title = "Forbidden"
		case fiber.StatusUnprocessableEntity:
			errType = "validation"
			title = "Unprocessable Entity"
		}
	}

	return c.Status(code).JSON(models.ProblemDetail{
		Type:     fmt.Sprintf("https://atap.dev/errors/%s", errType),
		Title:    title,
		Status:   code,
		Detail:   err.Error(),
		Instance: c.Path(),
	}, mimeApplicationProblemJSON)
}
