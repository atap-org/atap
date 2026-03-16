package api

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	// Step 7a: Check if the recipient is the server platform DID.
	// Server-addressed messages (e.g., TypeApprovalRevoke) are decrypted and processed
	// internally rather than delivered to an inbox.
	serverDID := "did:web:" + h.config.PlatformDomain + ":server:platform"
	if recipientDID == serverDID && h.platformX25519Key != nil {
		if handled, handlerErr := h.handleServerAddressedMessage(c, body, senderDID); handled {
			return handlerErr
		}
		// If not handled (unknown type), fall through to passthrough queuing.
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

// handleServerAddressedMessage decrypts and processes a JWE message addressed to the
// server platform DID (did:web:{domain}:server:platform). This is used for internal
// protocol messages that the server acts upon directly, rather than relaying to an entity.
//
// Returns (true, response) if the message was handled, or (false, nil) if it was
// an unknown type that should fall through to passthrough queuing.
func (h *Handler) handleServerAddressedMessage(c *fiber.Ctx, jweBytes []byte, senderDID string) (bool, error) {
	// Look up the sender's X25519 public key for ECDH-1PU decryption.
	// The senderDID was extracted from the JWE's skid field.
	var senderX25519Pub *ecdh.PublicKey
	if senderDID != "" {
		senderEntity, err := h.entityStore.GetEntityByDID(c.Context(), senderDID)
		if err == nil && senderEntity != nil && len(senderEntity.X25519PublicKey) > 0 {
			pub, err := ecdh.X25519().NewPublicKey(senderEntity.X25519PublicKey)
			if err == nil {
				senderX25519Pub = pub
			}
		}
	}

	if senderX25519Pub == nil {
		h.log.Warn().Str("sender_did", senderDID).Msg("server-addressed JWE: sender X25519 key not found, cannot decrypt")
		return false, nil
	}

	// Decrypt the JWE using the platform X25519 private key.
	plaintext, err := didcomm.Decrypt(jweBytes, h.platformX25519Key, senderX25519Pub)
	if err != nil {
		h.log.Warn().Err(err).Str("sender_did", senderDID).Msg("server-addressed JWE: decryption failed")
		return false, nil
	}

	// Parse the decrypted plaintext as a DIDComm PlaintextMessage.
	var msg didcomm.PlaintextMessage
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		h.log.Warn().Err(err).Msg("server-addressed JWE: failed to parse plaintext message")
		return false, nil
	}

	switch msg.Type {
	case didcomm.TypeApprovalRevoke:
		return true, h.processApprovalRevoke(c, &msg, senderDID)
	default:
		h.log.Warn().Str("type", msg.Type).Msg("server-addressed JWE: unhandled message type, falling through to passthrough")
		return false, nil
	}
}

// processApprovalRevoke handles an approval/1.0/revoke DIDComm message directed to the server.
// It stores a revocation entry and optionally forwards to the via DID if present.
func (h *Handler) processApprovalRevoke(c *fiber.Ctx, msg *didcomm.PlaintextMessage, senderDID string) error {
	body := msg.Body

	approvalID, _ := body["approval_id"].(string)
	if approvalID == "" {
		h.log.Warn().Str("sender_did", senderDID).Msg("approval/1.0/revoke missing approval_id")
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
	}

	// approver_did comes from the message body (set by the approver).
	// The server uses the senderDID from the SKID for attribution if body lacks approver_did.
	approverDID, _ := body["approver_did"].(string)
	if approverDID == "" {
		approverDID = senderDID
	}

	revokedAt := time.Now().UTC()

	// Compute expires_at: if valid_until is nil -> revokedAt + 60 minutes, else parse it.
	var expiresAt time.Time
	if validUntilStr, ok := body["valid_until"].(string); ok && validUntilStr != "" {
		parsed, err := time.Parse(time.RFC3339, validUntilStr)
		if err != nil {
			h.log.Warn().Err(err).Str("valid_until", validUntilStr).Msg("approval/1.0/revoke: invalid valid_until format")
			expiresAt = revokedAt.Add(60 * time.Minute)
		} else {
			expiresAt = parsed.UTC()
		}
	} else {
		expiresAt = revokedAt.Add(60 * time.Minute)
	}

	revocationID := "rev_" + ulid.MustNew(ulid.Timestamp(revokedAt), rand.Reader).String()
	rev := &models.Revocation{
		ID:          revocationID,
		ApprovalID:  approvalID,
		ApproverDID: approverDID,
		RevokedAt:   revokedAt,
		ExpiresAt:   expiresAt,
	}

	if err := h.revocationStore.CreateRevocation(c.Context(), rev); err != nil {
		h.log.Error().Err(err).Str("approval_id", approvalID).Msg("failed to store revocation from DIDComm revoke")
		// Non-fatal for the response — client gets 202 Accepted regardless.
	}

	// Forward to via DID if present (three-party case).
	if viaDID, ok := body["via"].(string); ok && viaDID != "" {
		msgID := "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
		forwardPayload, err := json.Marshal(msg)
		if err != nil {
			h.log.Error().Err(err).Msg("approval/1.0/revoke: failed to marshal forward message")
		} else {
			fwdMsg := &models.DIDCommMessage{
				ID:           msgID,
				RecipientDID: viaDID,
				SenderDID:    approverDID,
				MessageType:  didcomm.TypeApprovalRevoke,
				Payload:      forwardPayload,
				State:        "pending",
				CreatedAt:    time.Now().UTC(),
			}
			if err := h.messageStore.QueueMessage(context.Background(), fwdMsg); err != nil {
				h.log.Error().Err(err).Str("via", viaDID).Msg("approval/1.0/revoke: failed to queue forward message")
			}
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"id":     revocationID,
		"status": "queued",
	})
}

// dispatchDIDCommMessage serializes a PlaintextMessage and queues it for each recipient DID.
// This is used internally by the server to dispatch DIDComm messages (e.g., revocation forwards).
func (h *Handler) dispatchDIDCommMessage(msg *didcomm.PlaintextMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("dispatchDIDCommMessage: marshal: %w", err)
	}
	for _, recipientDID := range msg.To {
		msgID := "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
		qmsg := &models.DIDCommMessage{
			ID:           msgID,
			RecipientDID: recipientDID,
			SenderDID:    msg.From,
			MessageType:  msg.Type,
			Payload:      payload,
			State:        "pending",
			CreatedAt:    time.Now().UTC(),
		}
		if err := h.messageStore.QueueMessage(context.Background(), qmsg); err != nil {
			h.log.Error().Err(err).Str("recipient", recipientDID).Msg("dispatchDIDCommMessage: failed to queue")
		}
	}
	return nil
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
