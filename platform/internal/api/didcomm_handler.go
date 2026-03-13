package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/atap-dev/atap/platform/internal/didcomm"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// POST /v1/didcomm
// ============================================================

// HandleDIDComm handles incoming DIDComm v2.1 encrypted messages.
//
// This endpoint is PUBLIC — no OAuth/DPoP auth is required.
// DIDComm messages are self-authenticating via ECDH-1PU key agreement.
//
// The server acts as a mediator:
//  1. Validates Content-Type is application/didcomm-encrypted+json
//  2. Extracts recipient KID from JWE recipients[0].header.kid
//  3. Validates the recipient belongs to this server's domain
//  4. Looks up the entity by DID
//  5. Queues the message for offline pickup
//  6. Publishes to Redis for live SSE notification (best-effort)
func (h *Handler) HandleDIDComm(c *fiber.Ctx) error {
	// Step 1: Validate Content-Type.
	ct := c.Get("Content-Type")
	// Strip parameters (e.g., "; charset=utf-8") before comparing.
	ctBase := strings.Split(ct, ";")[0]
	ctBase = strings.TrimSpace(ctBase)
	if ctBase != didcomm.ContentTypeEncrypted {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(models.ProblemDetail{
			Type:     "https://atap.dev/errors/unsupported-media-type",
			Title:    "Unsupported Media Type",
			Status:   fiber.StatusUnsupportedMediaType,
			Detail:   "Content-Type must be " + didcomm.ContentTypeEncrypted,
			Instance: c.Path(),
		}, mimeApplicationProblemJSON)
	}

	// Step 2: Read raw JWE body.
	body := c.Body()
	if len(body) == 0 {
		return problem(c, fiber.StatusBadRequest, "empty-body", "Empty Body",
			"request body must contain a DIDComm JWE envelope")
	}

	// Step 3: Extract recipient KID from JWE.
	recipientKID, err := didcomm.ExtractRecipientKID(body)
	if err != nil {
		return problem(c, fiber.StatusBadRequest, "invalid-jwe", "Invalid JWE",
			"could not extract recipient KID: "+err.Error())
	}

	// Step 4: Validate recipient domain (prevent forwarding to foreign DIDs).
	if !didcomm.ValidateRecipientDomain(recipientKID, h.config.PlatformDomain) {
		return problem(c, fiber.StatusBadRequest, "invalid-recipient", "Invalid Recipient",
			"recipient DID does not belong to this server")
	}

	// Step 5: Extract DID from KID (strip fragment #key-x25519-1).
	recipientDID := recipientKID
	if idx := strings.Index(recipientDID, "#"); idx >= 0 {
		recipientDID = recipientDID[:idx]
	}

	// Step 6: Look up entity by DID.
	entity, err := h.entityStore.GetEntityByDID(c.Context(), recipientDID)
	if err != nil {
		h.log.Error().Err(err).Str("did", recipientDID).Msg("failed to look up recipient entity")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to look up recipient entity")
	}
	if entity == nil {
		return problem(c, fiber.StatusBadRequest, "unknown-recipient", "Unknown Recipient",
			"no entity registered with DID "+recipientDID)
	}

	// Step 7: Optionally extract sender KID for logging (non-fatal if missing).
	senderDID := ""
	if senderKID, err := didcomm.ExtractSenderKID(body); err == nil {
		// Strip fragment to get the DID.
		if idx := strings.Index(senderKID, "#"); idx >= 0 {
			senderDID = senderKID[:idx]
		} else {
			senderDID = senderKID
		}
	}

	// Step 8: Generate message ID.
	msgID := "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()

	// Step 9: Queue message for offline delivery.
	msg := &models.DIDCommMessage{
		ID:           msgID,
		RecipientDID: recipientDID,
		SenderDID:    senderDID,
		Payload:      body,
		State:        "pending",
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.messageStore.QueueMessage(c.Context(), msg); err != nil {
		h.log.Error().Err(err).Str("msg_id", msgID).Msg("failed to queue message")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to queue message for delivery")
	}

	// Step 10: Publish to Redis for live notification (best-effort — skip if unavailable).
	if h.redis != nil {
		redisKey := "inbox:" + entity.ID
		if pubErr := h.redis.Publish(context.Background(), redisKey, msgID).Err(); pubErr != nil {
			// Non-fatal: the message is already queued in DB, Redis is for live SSE only.
			h.log.Warn().Err(pubErr).Str("key", redisKey).Msg("Redis publish failed (non-fatal)")
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"id":     msgID,
		"status": "queued",
	})
}

// ============================================================
// GET /v1/didcomm/inbox
// ============================================================

// HandleInbox returns pending DIDComm messages for the authenticated entity.
//
// This endpoint REQUIRES DPoP auth with atap:inbox scope (set by DPoPAuthMiddleware).
//
// Behavior:
//  1. Get authenticated entity DID from context
//  2. Parse optional ?limit=N query param (default 50, max 100)
//  3. Retrieve pending messages from queue
//  4. Mark all returned messages as delivered
//  5. Return messages with payload base64-encoded
func (h *Handler) HandleInbox(c *fiber.Ctx) error {
	// Step 1: Get authenticated entity from context (set by DPoPAuthMiddleware).
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	// Step 2: Parse limit parameter.
	limit := c.QueryInt("limit", 50)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Step 3: Get pending messages.
	msgs, err := h.messageStore.GetPendingMessages(c.Context(), entity.DID, limit)
	if err != nil {
		h.log.Error().Err(err).Str("entity_did", entity.DID).Msg("failed to get pending messages")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to retrieve messages")
	}

	// Step 4: Mark all returned messages as delivered.
	if len(msgs) > 0 {
		ids := make([]string, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
		}
		if err := h.messageStore.MarkDelivered(c.Context(), ids); err != nil {
			// Non-fatal: messages were already returned to the client.
			// Log and continue — idempotent pickup is acceptable.
			h.log.Warn().Err(err).Strs("ids", ids).Msg("failed to mark messages as delivered (non-fatal)")
		}
	}

	// Step 5: Build response — payload is base64-encoded for safe JSON transport.
	type messageResponse struct {
		ID          string    `json:"id"`
		SenderDID   string    `json:"sender_did,omitempty"`
		MessageType string    `json:"message_type,omitempty"`
		Payload     string    `json:"payload"` // base64-encoded JWE bytes
		CreatedAt   time.Time `json:"created_at"`
	}

	respMsgs := make([]messageResponse, len(msgs))
	for i, m := range msgs {
		respMsgs[i] = messageResponse{
			ID:          m.ID,
			SenderDID:   m.SenderDID,
			MessageType: m.MessageType,
			Payload:     base64.StdEncoding.EncodeToString(m.Payload),
			CreatedAt:   m.CreatedAt,
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"messages": respMsgs,
		"count":    len(respMsgs),
	})
}
