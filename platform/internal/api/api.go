package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

type Handler struct {
	store  *store.Store
	redis  *redis.Client
	config *config.Config
	log    zerolog.Logger
}

func NewHandler(s *store.Store, r *redis.Client, cfg *config.Config, log zerolog.Logger) *Handler {
	return &Handler{store: s, redis: r, config: cfg, log: log}
}

// SetupRoutes configures all API routes.
func (h *Handler) SetupRoutes(app *fiber.App) {
	v1 := app.Group("/v1")

	// Health
	v1.Get("/health", h.Health)

	// Registration (no auth)
	v1.Post("/register", h.RegisterAgent)

	// Inbox
	v1.Post("/inbox/:targetId", h.AuthMiddleware(), h.SendSignal)
	v1.Get("/inbox/:entityId", h.AuthMiddleware(), h.PollInbox)
	v1.Get("/inbox/:entityId/stream", h.AuthMiddleware(), h.StreamInbox)

	// Channels
	v1.Post("/entities/:entityId/channels", h.AuthMiddleware(), h.CreateChannel)
	v1.Get("/entities/:entityId/channels", h.AuthMiddleware(), h.ListChannels)
	v1.Delete("/channels/:channelId", h.AuthMiddleware(), h.RevokeChannel)
	v1.Post("/channels/:channelId/signals", h.InboundWebhook) // no auth — validated by channel

	// Entities
	v1.Get("/entities/:entityId", h.GetEntity)

	// Verification endpoint (no auth)
	v1.Post("/verify", h.VerifyDelegation)
}

// ============================================================
// HEALTH
// ============================================================

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

func (h *Handler) RegisterAgent(c *fiber.Ctx) error {
	var req models.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	// Generate keypair
	pubKey, _, err := crypto.GenerateKeyPair()
	if err != nil {
		return problem(c, 500, "key_generation_failed", "Failed to generate keypair", "")
	}

	// Generate IDs
	entityID := crypto.NewEntityID()
	keyID := crypto.NewKeyID(entityID[:8])
	token, tokenHash := crypto.NewToken()

	// Set defaults
	deliveryPref := req.DeliveryPref
	if deliveryPref == "" {
		deliveryPref = models.DeliverySSE
	}

	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		URI:              fmt.Sprintf("agent://%s", entityID),
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             req.Name,
		Description:      req.Description,
		TrustLevel:       models.TrustLevel0,
		DeliveryPref:     deliveryPref,
		WebhookURL:       req.WebhookURL,
		Attestations:     json.RawMessage("{}"),
		TokenHash:        tokenHash,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        time.Now().UTC(),
	}

	if err := h.store.CreateEntity(c.Context(), entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create entity")
		return problem(c, 500, "creation_failed", "Failed to create entity", "")
	}

	baseURL := fmt.Sprintf("https://api.%s/v1", h.config.PlatformDomain)

	h.log.Info().Str("entity_id", entityID).Str("type", "agent").Msg("entity registered")

	return c.Status(201).JSON(models.RegisterResponse{
		URI:       entity.URI,
		ID:        entityID,
		Token:     token,
		PublicKey: crypto.EncodePublicKey(pubKey),
		KeyID:     keyID,
		InboxURL:  fmt.Sprintf("%s/inbox/%s", baseURL, entityID),
		StreamURL: fmt.Sprintf("%s/inbox/%s/stream", baseURL, entityID),
	})
}

// ============================================================
// INBOX
// ============================================================

func (h *Handler) SendSignal(c *fiber.Ctx) error {
	targetID := c.Params("targetId")
	sender := c.Locals("entity").(*models.Entity)

	var req models.SendSignalRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid signal body", err.Error())
	}

	// Verify target exists
	target, err := h.store.GetEntity(c.Context(), targetID)
	if err != nil {
		return problem(c, 500, "internal_error", "Failed to lookup target", "")
	}
	if target == nil {
		return problem(c, 404, "entity_not_found", "Target entity not found", "")
	}

	// Build signal
	now := time.Now().UTC()
	contentType := req.Type
	if contentType == "" {
		contentType = "application/json"
	}

	sig := &models.Signal{
		ID:      crypto.NewSignalID(),
		Version: "1",
		TS:      now.Format(time.RFC3339),
		Route: models.SignalRoute{
			Origin:  sender.URI,
			Target:  target.URI,
			ReplyTo: sender.URI,
			Thread:  req.Thread,
			Ref:     req.Ref,
		},
		Signal: models.SignalBody{
			Type:      contentType,
			Encrypted: false,
			Data:      req.Data,
		},
		CreatedAt: now,
	}

	if req.TTL != nil || req.Priority != nil || len(req.Tags) > 0 {
		sig.Context = &models.SignalContext{
			Source:   "agent",
			TTL:     req.TTL,
			Priority: 1,
			Tags:    req.Tags,
		}
		if req.Priority != nil {
			sig.Context.Priority = *req.Priority
		}
	}

	if req.TTL != nil && *req.TTL > 0 {
		exp := now.Add(time.Duration(*req.TTL) * time.Second)
		sig.ExpiresAt = &exp
	}

	// Save to database
	if err := h.store.SaveSignal(c.Context(), sig); err != nil {
		h.log.Error().Err(err).Str("signal_id", sig.ID).Msg("failed to save signal")
		return problem(c, 500, "save_failed", "Failed to save signal", "")
	}

	// Publish to Redis for real-time SSE delivery
	sigJSON, _ := json.Marshal(sig)
	h.redis.Publish(c.Context(), "inbox:"+targetID, string(sigJSON))

	h.log.Info().
		Str("signal_id", sig.ID).
		Str("from", sender.URI).
		Str("to", target.URI).
		Msg("signal sent")

	return c.Status(202).JSON(fiber.Map{
		"id":     sig.ID,
		"status": "accepted",
	})
}

func (h *Handler) PollInbox(c *fiber.Ctx) error {
	entityID := c.Params("entityId")
	entity := c.Locals("entity").(*models.Entity)

	// Verify ownership
	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "You can only read your own inbox", "")
	}

	afterID := c.Query("after", "")
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}

	signals, err := h.store.GetSignalsForEntity(c.Context(), entity.URI, afterID, limit)
	if err != nil {
		return problem(c, 500, "query_failed", "Failed to query inbox", "")
	}

	cursor := ""
	if len(signals) > 0 {
		cursor = signals[len(signals)-1].ID
	}

	return c.JSON(fiber.Map{
		"signals":  signals,
		"cursor":   cursor,
		"has_more": len(signals) == limit,
	})
}

func (h *Handler) StreamInbox(c *fiber.Ctx) error {
	entityID := c.Params("entityId")
	entity := c.Locals("entity").(*models.Entity)

	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "You can only stream your own inbox", "")
	}

	lastEventID := c.Get("Last-Event-ID", "")

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ctx := c.Context()

	c.Context().SetBodyStreamWriter(func(w *bufioWriter) {
		// 1. Replay missed signals if Last-Event-ID provided
		if lastEventID != "" {
			missed, _ := h.store.GetSignalsForEntity(context.Background(), entity.URI, lastEventID, 100)
			for _, sig := range missed {
				sigJSON, _ := json.Marshal(sig)
				fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", sig.ID, string(sigJSON))
			}
			w.Flush()
		}

		// 2. Subscribe to Redis for live signals
		sub := h.redis.Subscribe(context.Background(), "inbox:"+entityID)
		defer sub.Close()

		ch := sub.Channel()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var sig models.Signal
				json.Unmarshal([]byte(msg.Payload), &sig)
				fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", sig.ID, msg.Payload)
				w.Flush()
			case <-ticker.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				w.Flush()
			case <-ctx.Done():
				return
			}
		}
	})

	return nil
}

// ============================================================
// CHANNELS
// ============================================================

func (h *Handler) CreateChannel(c *fiber.Ctx) error {
	entityID := c.Params("entityId")
	entity := c.Locals("entity").(*models.Entity)

	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "You can only create channels for your own entity", "")
	}

	var req struct {
		Label     string    `json:"label"`
		Tags      []string  `json:"tags"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	channelID := crypto.NewChannelID()
	webhookURL := fmt.Sprintf("https://api.%s/v1/channels/%s/signals", h.config.PlatformDomain, channelID)

	ch := &models.Channel{
		ID:         channelID,
		EntityID:   entityID,
		WebhookURL: webhookURL,
		Label:      req.Label,
		Tags:       req.Tags,
		ExpiresAt:  req.ExpiresAt,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
	}

	if err := h.store.CreateChannel(c.Context(), ch); err != nil {
		return problem(c, 500, "creation_failed", "Failed to create channel", "")
	}

	h.log.Info().Str("channel_id", channelID).Str("entity_id", entityID).Msg("channel created")

	return c.Status(201).JSON(ch)
}

func (h *Handler) ListChannels(c *fiber.Ctx) error {
	entityID := c.Params("entityId")
	entity := c.Locals("entity").(*models.Entity)

	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "You can only list your own channels", "")
	}

	channels, err := h.store.ListChannels(c.Context(), entityID)
	if err != nil {
		return problem(c, 500, "query_failed", "Failed to list channels", "")
	}

	return c.JSON(fiber.Map{"channels": channels})
}

func (h *Handler) RevokeChannel(c *fiber.Ctx) error {
	channelID := c.Params("channelId")

	ch, err := h.store.GetChannel(c.Context(), channelID)
	if err != nil || ch == nil {
		return problem(c, 404, "not_found", "Channel not found", "")
	}

	entity := c.Locals("entity").(*models.Entity)
	if entity.ID != ch.EntityID {
		return problem(c, 403, "forbidden", "You can only revoke your own channels", "")
	}

	if err := h.store.RevokeChannel(c.Context(), channelID); err != nil {
		return problem(c, 500, "revoke_failed", "Failed to revoke channel", "")
	}

	return c.JSON(fiber.Map{"status": "revoked"})
}

func (h *Handler) InboundWebhook(c *fiber.Ctx) error {
	channelID := c.Params("channelId")

	ch, err := h.store.GetChannel(c.Context(), channelID)
	if err != nil || ch == nil {
		return problem(c, 404, "not_found", "Channel not found", "")
	}
	if !ch.Active {
		return problem(c, 410, "channel_revoked", "This channel has been revoked", "")
	}
	if ch.ExpiresAt != nil && time.Now().After(*ch.ExpiresAt) {
		return problem(c, 410, "channel_expired", "This channel has expired", "")
	}

	// Get the entity that owns this channel
	entity, _ := h.store.GetEntity(c.Context(), ch.EntityID)
	if entity == nil {
		return problem(c, 404, "entity_not_found", "Channel owner not found", "")
	}

	// Parse incoming payload (accept any JSON)
	var payload json.RawMessage
	if err := c.BodyParser(&payload); err != nil {
		return problem(c, 400, "invalid_payload", "Request body must be valid JSON", "")
	}

	// Wrap in ATAP signal
	now := time.Now().UTC()
	sig := &models.Signal{
		ID:      crypto.NewSignalID(),
		Version: "1",
		TS:      now.Format(time.RFC3339),
		Route: models.SignalRoute{
			Origin:  "external",
			Target:  entity.URI,
			Channel: channelID,
		},
		Signal: models.SignalBody{
			Type:      "application/json",
			Encrypted: false,
			Data:      payload,
		},
		Context: &models.SignalContext{
			Source: "webhook",
		},
		CreatedAt: now,
	}

	if err := h.store.SaveSignal(c.Context(), sig); err != nil {
		return problem(c, 500, "save_failed", "Failed to process webhook", "")
	}

	// Publish for real-time delivery
	sigJSON, _ := json.Marshal(sig)
	h.redis.Publish(c.Context(), "inbox:"+ch.EntityID, string(sigJSON))

	// Update channel stats
	h.store.IncrementChannelCount(c.Context(), channelID)

	h.log.Info().
		Str("signal_id", sig.ID).
		Str("channel_id", channelID).
		Str("entity_id", ch.EntityID).
		Msg("inbound webhook processed")

	return c.Status(202).JSON(fiber.Map{"signal_id": sig.ID})
}

// ============================================================
// ENTITIES
// ============================================================

func (h *Handler) GetEntity(c *fiber.Ctx) error {
	entityID := c.Params("entityId")

	entity, err := h.store.GetEntity(c.Context(), entityID)
	if err != nil {
		return problem(c, 500, "query_failed", "Failed to get entity", "")
	}
	if entity == nil {
		return problem(c, 404, "not_found", "Entity not found", "")
	}

	// Add base64 public key to response
	entity.PublicKeyBase64 = base64.StdEncoding.EncodeToString(entity.PublicKeyEd25519)

	return c.JSON(entity)
}

// ============================================================
// VERIFY (stub for Phase 2)
// ============================================================

func (h *Handler) VerifyDelegation(c *fiber.Ctx) error {
	return problem(c, 501, "not_implemented", "Delegation verification will be available in Phase 2", "")
}

// ============================================================
// AUTH MIDDLEWARE
// ============================================================

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

// bufioWriter wraps the Fiber stream writer interface
type bufioWriter = interface {
	Write(p []byte) (n int, err error)
	Flush() error
}
