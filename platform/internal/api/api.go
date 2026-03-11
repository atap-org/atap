package api

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

// EntityStore defines the data access methods required by the API layer.
type EntityStore interface {
	CreateEntity(ctx context.Context, e *models.Entity) error
	GetEntity(ctx context.Context, id string) (*models.Entity, error)
	GetEntityByKeyID(ctx context.Context, keyID string) (*models.Entity, error)
}

// SignalStore defines signal data access methods.
type SignalStore interface {
	SaveSignal(ctx context.Context, s *models.Signal) error
	GetSignal(ctx context.Context, id string) (*models.Signal, error)
	GetInbox(ctx context.Context, entityID string, after string, limit int) ([]*models.Signal, bool, error)
	GetSignalsAfter(ctx context.Context, entityID string, afterID string) ([]*models.Signal, error)
}

// WebhookStore defines webhook delivery data access methods.
type WebhookStore interface {
	UpdateSignalDeliveryStatus(ctx context.Context, signalID, status string) error
	SaveDeliveryAttempt(ctx context.Context, a *models.DeliveryAttempt) error
	GetPendingRetries(ctx context.Context, now time.Time) ([]*models.DeliveryAttempt, error)
	CleanupDeliveryAttempts(ctx context.Context, olderThan time.Time) (int64, error)
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	entityStore EntityStore
	signalStore SignalStore
	config      *config.Config
	redis       *redis.Client
	platformKey ed25519.PrivateKey
	log         zerolog.Logger
}

// NewHandler creates a new Handler with all dependencies.
func NewHandler(es EntityStore, ss SignalStore, rdb *redis.Client, platformKey ed25519.PrivateKey, cfg *config.Config, log zerolog.Logger) *Handler {
	return &Handler{
		entityStore: es,
		signalStore: ss,
		config:      cfg,
		redis:       rdb,
		platformKey: platformKey,
		log:         log,
	}
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

	// Signal sending and inbox (auth required)
	auth.Post("/inbox/:entityId", h.SendSignal)
	auth.Get("/inbox/:entityId", h.GetInbox)
	auth.Get("/inbox/:entityId/stream", h.InboxStream)
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

// RegisterAgent creates a new agent entity with generated keypair.
// No bearer token is generated -- agents authenticate via Ed25519 signed requests.
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

	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		URI:              fmt.Sprintf("agent://%s", entityID),
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             req.Name,
		TrustLevel:       models.TrustLevel0,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        time.Now().UTC(),
	}

	if err := h.entityStore.CreateEntity(c.Context(), entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create entity")
		return problem(c, 500, "creation_failed", "Failed to create entity", "")
	}

	h.log.Info().Str("entity_id", entityID).Str("type", "agent").Msg("entity registered")

	return c.Status(201).JSON(models.RegisterResponse{
		URI:        entity.URI,
		ID:         entityID,
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

	entity, err := h.entityStore.GetEntity(c.Context(), entityID)
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
// SIGNALS
// ============================================================

// SendSignal handles POST /v1/inbox/:entityId — sends a signal to an entity's inbox.
// The write-then-notify pattern ensures PostgreSQL persistence before Redis publish.
func (h *Handler) SendSignal(c *fiber.Ctx) error {
	sender := c.Locals("entity").(*models.Entity)
	targetID := c.Params("entityId")

	// Check payload size
	if len(c.Body()) > models.MaxSignalPayload {
		return problem(c, 413, "payload_too_large",
			"Signal payload exceeds maximum size",
			fmt.Sprintf("Maximum payload size is %d bytes", models.MaxSignalPayload))
	}

	// Parse request
	var req models.SendSignalRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	// Validate signal type is present
	if req.Signal.Type == "" {
		return problem(c, 400, "invalid_request", "Signal type is required", "")
	}

	// Verify target entity exists
	target, err := h.entityStore.GetEntity(c.Context(), targetID)
	if err != nil {
		return problem(c, 500, "query_failed", "Failed to look up target entity", "")
	}
	if target == nil {
		return problem(c, 404, "not_found", "Target entity not found", "")
	}

	// Verify sender's signature over route + signal
	if req.Trust.Signature == "" || req.Trust.SignerKeyID == "" {
		return problem(c, 400, "invalid_request", "Trust signature and signer_key_id are required", "")
	}

	// Verify the signer_key_id matches the authenticated entity
	if req.Trust.SignerKeyID != sender.KeyID {
		return problem(c, 403, "forbidden", "Signer key ID does not match authenticated entity", "")
	}

	// Build signable payload and verify signature
	signablePayload, err := crypto.SignablePayload(req.Route, req.Signal)
	if err != nil {
		return problem(c, 400, "invalid_request", "Failed to build signable payload", err.Error())
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(req.Trust.Signature)
	if err != nil {
		return problem(c, 400, "invalid_request", "Invalid signature encoding", err.Error())
	}

	if !crypto.Verify(sender.PublicKeyEd25519, signablePayload, sigBytes) {
		return problem(c, 400, "invalid_signature", "Signal signature verification failed", "")
	}

	// Build the signal
	now := time.Now().UTC()
	sig := &models.Signal{
		ID:      crypto.NewSignalID(),
		Version: "1",
		TS:      now,
		Route: models.SignalRoute{
			Origin:  sender.URI,
			Target:  target.URI,
			ReplyTo: req.Route.ReplyTo,
			Channel: req.Route.Channel,
			Thread:  req.Route.Thread,
			Ref:     req.Route.Ref,
		},
		Trust: models.SignalTrust{
			Level:      sender.TrustLevel,
			Signer:     sender.URI,
			SignerKeyID: sender.KeyID,
			Signature:  req.Trust.Signature,
		},
		Signal: req.Signal,
		Context: models.SignalContext{
			Source:      models.SignalSourceAgent,
			Idempotency: req.Context.Idempotency,
			Tags:        req.Context.Tags,
			TTL:         req.Context.TTL,
			Priority:    req.Context.Priority,
		},
		TargetEntityID: targetID,
		DeliveryStatus: models.DeliveryPending,
		CreatedAt:      now,
	}

	// Set default priority if not specified
	if sig.Context.Priority == "" {
		sig.Context.Priority = models.PriorityNormal
	}

	// Write to PostgreSQL first (write-then-notify pattern)
	if err := h.signalStore.SaveSignal(c.Context(), sig); err != nil {
		if err == store.ErrDuplicateSignal {
			return problem(c, 409, "duplicate_signal", "Signal with this idempotency key already exists", "")
		}
		h.log.Error().Err(err).Str("signal_id", sig.ID).Msg("failed to save signal")
		return problem(c, 500, "save_failed", "Failed to save signal", "")
	}

	// Then publish to Redis for real-time delivery (SSE)
	if h.redis != nil {
		sigJSON, err := json.Marshal(sig)
		if err != nil {
			h.log.Error().Err(err).Str("signal_id", sig.ID).Msg("failed to marshal signal for Redis")
		} else {
			if err := h.redis.Publish(c.Context(), "inbox:"+targetID, string(sigJSON)).Err(); err != nil {
				h.log.Error().Err(err).Str("signal_id", sig.ID).Msg("failed to publish signal to Redis")
			}
		}
	}

	h.log.Info().
		Str("signal_id", sig.ID).
		Str("from", sender.ID).
		Str("to", targetID).
		Str("type", sig.Signal.Type).
		Msg("signal sent")

	return c.Status(202).JSON(sig)
}

// GetInbox handles GET /v1/inbox/:entityId — returns paginated inbox signals.
func (h *Handler) GetInbox(c *fiber.Ctx) error {
	entity := c.Locals("entity").(*models.Entity)
	entityID := c.Params("entityId")

	// Entity can only read their own inbox
	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "Cannot read another entity's inbox", "")
	}

	// Parse pagination params
	after := c.Query("after", "")
	limitStr := c.Query("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	signals, hasMore, err := h.signalStore.GetInbox(c.Context(), entityID, after, limit)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to get inbox")
		return problem(c, 500, "query_failed", "Failed to get inbox", "")
	}

	resp := models.InboxResponse{
		Signals: signals,
		HasMore: hasMore,
	}
	if resp.Signals == nil {
		resp.Signals = []*models.Signal{}
	}
	if hasMore && len(signals) > 0 {
		resp.Cursor = signals[len(signals)-1].ID
	}

	return c.JSON(resp)
}

// InboxStream handles GET /v1/inbox/:entityId/stream — SSE stream for real-time signal delivery.
// Subscribes to Redis first, then replays missed signals from PostgreSQL (no replay gap).
// Sends 30-second heartbeat comments to keep connections alive.
func (h *Handler) InboxStream(c *fiber.Ctx) error {
	entity := c.Locals("entity").(*models.Entity)
	entityID := c.Params("entityId")

	// Entity can only stream their own inbox
	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "Cannot stream another entity's inbox", "")
	}

	lastEventID := c.Get("Last-Event-ID", "")

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// Subscribe to Redis FIRST (before replay) to avoid replay gap
		ctx := context.Background()
		sub := h.redis.Subscribe(ctx, "inbox:"+entityID)
		defer sub.Close()
		ch := sub.Channel()

		// Replay missed signals from PostgreSQL if reconnecting
		if lastEventID != "" {
			signals, err := h.signalStore.GetSignalsAfter(ctx, entityID, lastEventID)
			if err != nil {
				h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to replay signals")
			} else {
				for _, sig := range signals {
					data, err := json.Marshal(sig)
					if err != nil {
						continue
					}
					fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", sig.ID, string(data))
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}

		// Heartbeat ticker: 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				// Extract signal ID from the JSON for the SSE id field
				var sig models.Signal
				if err := json.Unmarshal([]byte(msg.Payload), &sig); err != nil {
					h.log.Error().Err(err).Msg("failed to unmarshal Redis signal")
					continue
				}
				fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", sig.ID, msg.Payload)
				if err := w.Flush(); err != nil {
					return // client disconnected
				}

			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				if err := w.Flush(); err != nil {
					return // client disconnected
				}
			}
		}
	}))

	return nil
}

// ============================================================
// AUTH MIDDLEWARE
// ============================================================

// AuthMiddleware validates Ed25519 signed requests.
// Expects Authorization header: Signature keyId="key_...",algorithm="ed25519",headers="(request-target) x-atap-timestamp",signature="base64..."
// And X-Atap-Timestamp header with RFC3339 timestamp.
func (h *Handler) AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if auth == "" {
			return problem(c, 401, "unauthorized", "Missing Authorization header", "")
		}

		// Parse keyId from the Signature header
		keyID, _, err := crypto.ParseSignatureHeader(auth)
		if err != nil {
			return problem(c, 401, "unauthorized", "Invalid Authorization format, expected Signature scheme", "")
		}

		// Get timestamp header
		timestamp := c.Get("X-Atap-Timestamp")
		if timestamp == "" {
			return problem(c, 401, "unauthorized", "Missing X-Atap-Timestamp header", "")
		}

		// Look up entity by keyID
		entity, err := h.entityStore.GetEntityByKeyID(c.Context(), keyID)
		if err != nil {
			return problem(c, 500, "auth_error", "Authentication failed", "")
		}
		if entity == nil {
			return problem(c, 401, "unauthorized", "Unknown key", "")
		}

		// Verify the signature
		if err := crypto.VerifyRequest(entity.PublicKeyEd25519, auth, c.Method(), c.Path(), timestamp); err != nil {
			return problem(c, 401, "unauthorized", "Invalid signature", "")
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
