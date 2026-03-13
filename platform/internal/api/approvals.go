package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/didcomm"
	"github.com/atap-dev/atap/platform/internal/models"
)

// serverKeyID returns the platform server's signing key ID.
// Format: did:web:{domain}:server:platform#key-ed25519-0
func (h *Handler) serverKeyID() string {
	return fmt.Sprintf("did:web:%s:server:platform#key-ed25519-0", h.config.PlatformDomain)
}

// serverDID returns the platform server's DID.
func (h *Handler) serverDID() string {
	return fmt.Sprintf("did:web:%s:server:platform", h.config.PlatformDomain)
}

// dispatchDIDCommMessage queues a plaintext DIDComm message for delivery.
// Serializes the message to JSON and stores it in the message queue for each recipient.
func (h *Handler) dispatchDIDCommMessage(msg *didcomm.PlaintextMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		h.log.Error().Err(err).Str("msg_id", msg.ID).Msg("failed to marshal DIDComm message for dispatch")
		return
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
			h.log.Error().Err(err).Str("recipient", recipientDID).Msg("failed to queue DIDComm message")
		}
	}
}

// dispatchApprovalMessage dispatches a DIDComm approval lifecycle message.
// If ctx is nil (background context), uses context.Background().
func (h *Handler) dispatchApprovalMessage(msgType, from string, recipients []string, apr *models.Approval, extraBody map[string]any) {
	body := map[string]any{
		"approval_id": apr.ID,
		"status":      apr.State,
	}
	for k, v := range extraBody {
		body[k] = v
	}
	msg := didcomm.NewMessage(msgType, from, recipients, body)
	msg.Attachments = []didcomm.Attachment{
		{
			ID:        "approval",
			MediaType: "application/json",
			Data: didcomm.AttachmentData{
				JSON: apr,
			},
		},
	}
	h.dispatchDIDCommMessage(msg)
}

// ============================================================
// POST /v1/approvals
// ============================================================

// CreateApproval handles POST /v1/approvals.
// Creates a two-party or three-party approval document and begins the approval flow.
func (h *Handler) CreateApproval(c *fiber.Ctx) error {
	// Get authenticated entity
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	var req struct {
		To            string            `json:"to"`
		Via           string            `json:"via,omitempty"`
		Subject       models.ApprovalSubject `json:"subject"`
		TemplateURL   string            `json:"template_url,omitempty"`
		Parent        string            `json:"parent,omitempty"`
		ValidUntil    *time.Time        `json:"valid_until,omitempty"`
		FromSignature string            `json:"from_signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, fiber.StatusBadRequest, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.To == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "to is required")
	}
	if req.FromSignature == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "from_signature is required")
	}

	ctx := c.Context()

	// Validate to entity exists
	toEntity, err := h.entityStore.GetEntityByDID(ctx, req.To)
	if err != nil {
		h.log.Error().Err(err).Str("to_did", req.To).Msg("failed to look up to entity")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to validate to entity")
	}
	if toEntity == nil {
		return problem(c, fiber.StatusNotFound, "not-found", "Not Found",
			fmt.Sprintf("entity with DID %q not found", req.To))
	}

	// Validate parent if specified
	if req.Parent != "" {
		parentApr, err := h.approvalStore.GetApproval(ctx, req.Parent)
		if err != nil {
			return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to validate parent approval")
		}
		if parentApr == nil {
			return problem(c, fiber.StatusBadRequest, "validation", "Validation Error",
				fmt.Sprintf("parent approval %q not found", req.Parent))
		}
		if approval.IsTerminalState(parentApr.State) {
			// For three-party flow: terminated parent triggers via rejection
			if req.Via != "" {
				// Fall through to via rejection below
			} else {
				return problem(c, fiber.StatusBadRequest, "validation", "Validation Error",
					fmt.Sprintf("parent approval %q is in terminal state %q", req.Parent, parentApr.State))
			}
		}
	}

	// Build approval document
	now := time.Now().UTC()
	apr := &models.Approval{
		AtapApproval: "1",
		ID:           crypto.NewApprovalID(),
		CreatedAt:    now,
		From:         entity.DID,
		To:           req.To,
		Via:          req.Via,
		Parent:       req.Parent,
		Subject:      req.Subject,
		TemplateURL:  req.TemplateURL,
		Signatures:   map[string]string{},
		State:        models.ApprovalStateRequested,
		UpdatedAt:    now,
	}

	// Clamp valid_until if provided
	if req.ValidUntil != nil {
		apr.ValidUntil = approval.ClampValidUntil(req.ValidUntil, h.config.MaxApprovalTTL)
	}

	// Verify from signature (APR-12)
	fromKeyID := fmt.Sprintf("%s#%s", entity.DID, entity.KeyID)
	fromPubKey := ed25519.PublicKey(entity.PublicKeyEd25519)
	if err := approval.VerifyApprovalSignature(apr, req.FromSignature, fromKeyID, fromPubKey); err != nil {
		return problem(c, fiber.StatusBadRequest, "invalid-signature", "Invalid Signature",
			fmt.Sprintf("from_signature verification failed: %v", err))
	}
	apr.Signatures["from"] = req.FromSignature

	// Three-party flow
	if req.Via != "" {
		// Verify via matches server DID
		serverDID := h.serverDID()
		if req.Via != serverDID {
			return problem(c, fiber.StatusBadRequest, "validation", "Validation Error",
				fmt.Sprintf("via must be the server DID %q in Phase 3", serverDID))
		}

		// Via validation gate (APR-08): check for policy violations
		if rejectionReason := h.validateViaApproval(apr, req.Parent); rejectionReason != "" {
			apr.State = models.ApprovalStateRejected
			apr.UpdatedAt = time.Now().UTC()
			if storeErr := h.approvalStore.CreateApproval(ctx, apr); storeErr != nil {
				h.log.Error().Err(storeErr).Msg("failed to store rejected approval")
			}
			// Dispatch TypeApprovalRejected to from
			rejMsg := didcomm.NewMessage(didcomm.TypeApprovalRejected, serverDID, []string{entity.DID},
				map[string]any{
					"approval_id": apr.ID,
					"reason":      rejectionReason,
					"detail":      viaRejectionDetail(rejectionReason),
				})
			h.dispatchDIDCommMessage(rejMsg)
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"type":        "https://atap.dev/errors/via-rejected",
				"title":       "Approval Rejected by Via",
				"status":      422,
				"detail":      viaRejectionDetail(rejectionReason),
				"reason":      rejectionReason,
				"approval_id": apr.ID,
				"instance":    c.Path(),
			}, mimeApplicationProblemJSON)
		}

		// Server co-signs as via
		serverKeyID := h.serverKeyID()
		viaSig, err := approval.SignApproval(apr, h.platformKey, serverKeyID)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to sign approval as via")
			return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to co-sign approval")
		}
		apr.Signatures["via"] = viaSig
	}

	// Persist
	if err := h.approvalStore.CreateApproval(ctx, apr); err != nil {
		h.log.Error().Err(err).Str("approval_id", apr.ID).Msg("failed to create approval")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to create approval")
	}

	// Dispatch DIDComm message
	msgFrom := entity.DID
	if req.Via != "" {
		msgFrom = h.serverDID()
	}
	h.dispatchApprovalMessage(didcomm.TypeApprovalRequest, msgFrom, []string{req.To}, apr, nil)

	return c.Status(fiber.StatusCreated).JSON(approvalResponse(apr))
}

// validateViaApproval checks server policy for three-party flow.
// Returns a rejection reason code or "" if valid.
func (h *Handler) validateViaApproval(apr *models.Approval, parentID string) string {
	// Check subject type is set
	if apr.Subject.Type == "" {
		return "unsupported_subject_type"
	}
	// Check parent is not in terminal state (already checked before calling, but re-check for safety)
	if parentID != "" {
		parentApr, err := h.approvalStore.GetApproval(context.Background(), parentID)
		if err == nil && parentApr != nil && approval.IsTerminalState(parentApr.State) {
			return "parent_terminated"
		}
	}
	return ""
}

// viaRejectionDetail returns a human-readable explanation for a via rejection reason.
func viaRejectionDetail(reason string) string {
	switch reason {
	case "unsupported_subject_type":
		return "The subject type is not supported by this server"
	case "invalid_template":
		return "The template URL could not be fetched or verified"
	case "parent_terminated":
		return "The parent approval is in a terminal state and cannot be extended"
	case "policy_violation":
		return "The approval request violates server policy"
	default:
		return "The approval request was rejected by the via system"
	}
}

// ============================================================
// POST /v1/approvals/{id}/respond
// ============================================================

// RespondApproval handles POST /v1/approvals/{id}/respond.
// The to entity approves or declines the approval request.
func (h *Handler) RespondApproval(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	approvalID := c.Params("approvalId")

	var req struct {
		Status    string `json:"status"`
		Signature string `json:"signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, fiber.StatusBadRequest, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Status != models.ApprovalStateApproved && req.Status != models.ApprovalStateDeclined {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error",
			"status must be 'approved' or 'declined'")
	}
	if req.Signature == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "signature is required")
	}

	ctx := c.Context()

	apr, err := h.approvalStore.GetApproval(ctx, approvalID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to retrieve approval")
	}
	if apr == nil {
		return problem(c, fiber.StatusNotFound, "not-found", "Not Found",
			fmt.Sprintf("approval %q not found", approvalID))
	}

	// Verify authenticated entity is the to party
	if entity.DID != apr.To {
		return problem(c, fiber.StatusForbidden, "forbidden", "Forbidden",
			"only the to entity may respond to this approval")
	}

	// Validate state transition
	if err := approval.ValidateTransition(apr.State, req.Status); err != nil {
		return problem(c, fiber.StatusConflict, "invalid-transition", "Invalid State Transition",
			err.Error())
	}

	// Verify to signature (APR-12)
	toKeyID := fmt.Sprintf("%s#%s", entity.DID, entity.KeyID)
	toPubKey := ed25519.PublicKey(entity.PublicKeyEd25519)
	if err := approval.VerifyApprovalSignature(apr, req.Signature, toKeyID, toPubKey); err != nil {
		return problem(c, fiber.StatusBadRequest, "invalid-signature", "Invalid Signature",
			fmt.Sprintf("signature verification failed: %v", err))
	}

	// Update signatures (not persisted to document but kept in response)
	apr.Signatures["to"] = req.Signature

	// Update state in store
	now := time.Now().UTC()
	if err := h.approvalStore.UpdateApprovalState(ctx, approvalID, req.Status, &now); err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to update approval state")
	}
	apr.State = req.Status
	apr.RespondedAt = &now

	// Dispatch DIDComm response to from (and via if present)
	recipients := []string{apr.From}
	if apr.Via != "" {
		recipients = append(recipients, apr.Via)
	}
	h.dispatchApprovalMessage(didcomm.TypeApprovalResponse, entity.DID, recipients, apr, map[string]any{
		"response_status": req.Status,
	})

	return c.JSON(approvalResponse(apr))
}

// ============================================================
// GET /v1/approvals/{id}
// ============================================================

// GetApproval handles GET /v1/approvals/{id}.
// Returns the full approval document. Requires the entity to be a party.
func (h *Handler) GetApproval(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	approvalID := c.Params("approvalId")
	apr, err := h.approvalStore.GetApproval(c.Context(), approvalID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to retrieve approval")
	}
	if apr == nil {
		return problem(c, fiber.StatusNotFound, "not-found", "Not Found",
			fmt.Sprintf("approval %q not found", approvalID))
	}

	// Verify authenticated entity is a party
	if entity.DID != apr.From && entity.DID != apr.To && entity.DID != apr.Via {
		return problem(c, fiber.StatusForbidden, "forbidden", "Forbidden",
			"you are not a party to this approval")
	}

	return c.JSON(approvalResponse(apr))
}

// ============================================================
// GET /v1/approvals/{id}/status
// ============================================================

// GetApprovalStatus handles GET /v1/approvals/{id}/status.
// Public endpoint — atomically consumes one-time approvals on first check (APR-09).
func (h *Handler) GetApprovalStatus(c *fiber.Ctx) error {
	approvalID := c.Params("approvalId")
	apr, err := h.approvalStore.GetApproval(c.Context(), approvalID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to retrieve approval")
	}
	if apr == nil {
		return problem(c, fiber.StatusNotFound, "not-found", "Not Found",
			fmt.Sprintf("approval %q not found", approvalID))
	}

	checkedAt := time.Now().UTC().Format(time.RFC3339)

	// Check if already consumed
	if apr.State == models.ApprovalStateConsumed {
		return c.JSON(fiber.Map{
			"approval_id": approvalID,
			"valid":       false,
			"checked_at":  checkedAt,
			"reason":      "consumed",
		})
	}

	// One-time approval: state=="approved" and valid_until==nil
	if apr.State == models.ApprovalStateApproved && apr.ValidUntil == nil {
		consumed, err := h.approvalStore.ConsumeApproval(c.Context(), approvalID)
		if err != nil {
			return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to consume approval")
		}
		if consumed {
			return c.JSON(fiber.Map{
				"approval_id": approvalID,
				"valid":       true,
				"checked_at":  checkedAt,
				"consumed":    true,
			})
		}
		// Race: another request consumed it just before us
		return c.JSON(fiber.Map{
			"approval_id": approvalID,
			"valid":       false,
			"checked_at":  checkedAt,
			"reason":      "consumed",
		})
	}

	// Persistent approval check
	if apr.State != models.ApprovalStateApproved {
		return c.JSON(fiber.Map{
			"approval_id": approvalID,
			"valid":       false,
			"checked_at":  checkedAt,
			"reason":      apr.State,
		})
	}

	// Check expiry
	if apr.ValidUntil != nil && time.Now().After(*apr.ValidUntil) {
		return c.JSON(fiber.Map{
			"approval_id": approvalID,
			"valid":       false,
			"checked_at":  checkedAt,
			"reason":      "expired",
		})
	}

	// Check parent validity if applicable
	if apr.Parent != "" {
		parentApr, err := h.approvalStore.GetApproval(c.Context(), apr.Parent)
		if err != nil || parentApr == nil || parentApr.State != models.ApprovalStateApproved {
			return c.JSON(fiber.Map{
				"approval_id": approvalID,
				"valid":       false,
				"checked_at":  checkedAt,
				"reason":      "parent_invalid",
			})
		}
		// Check parent expiry
		if parentApr.ValidUntil != nil && time.Now().After(*parentApr.ValidUntil) {
			return c.JSON(fiber.Map{
				"approval_id": approvalID,
				"valid":       false,
				"checked_at":  checkedAt,
				"reason":      "parent_expired",
			})
		}
	}

	return c.JSON(fiber.Map{
		"approval_id": approvalID,
		"valid":       true,
		"checked_at":  checkedAt,
	})
}

// ============================================================
// GET /v1/approvals
// ============================================================

// ListApprovals handles GET /v1/approvals.
// Lists approvals for the authenticated entity with pagination.
func (h *Handler) ListApprovals(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	limit := c.QueryInt("limit", 20)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	approvals, err := h.approvalStore.ListApprovals(c.Context(), entity.DID, limit, offset)
	if err != nil {
		h.log.Error().Err(err).Str("entity_did", entity.DID).Msg("failed to list approvals")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to list approvals")
	}

	result := make([]map[string]any, len(approvals))
	for i := range approvals {
		result[i] = approvalResponse(&approvals[i])
	}

	return c.JSON(fiber.Map{
		"approvals": result,
		"count":     len(approvals),
	})
}

// ============================================================
// DELETE /v1/approvals/{id}
// ============================================================

// RevokeApproval handles DELETE /v1/approvals/{id}.
// Only the to entity may revoke a persistent approval (with valid_until set).
func (h *Handler) RevokeApproval(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	approvalID := c.Params("approvalId")
	apr, err := h.approvalStore.GetApproval(c.Context(), approvalID)
	if err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to retrieve approval")
	}
	if apr == nil {
		return problem(c, fiber.StatusNotFound, "not-found", "Not Found",
			fmt.Sprintf("approval %q not found", approvalID))
	}

	// Only to entity can revoke
	if entity.DID != apr.To {
		return problem(c, fiber.StatusForbidden, "forbidden", "Forbidden",
			"only the to entity may revoke this approval")
	}

	// Must be approved with valid_until set (persistent)
	if apr.State != models.ApprovalStateApproved {
		return problem(c, fiber.StatusConflict, "invalid-state", "Invalid State",
			fmt.Sprintf("approval must be in 'approved' state to revoke, current state: %q", apr.State))
	}
	if apr.ValidUntil == nil {
		return problem(c, fiber.StatusConflict, "invalid-state", "Invalid State",
			"one-time approvals cannot be explicitly revoked; they are consumed automatically")
	}

	// Cascade revocation to children
	if err := h.approvalStore.RevokeWithChildren(c.Context(), approvalID); err != nil {
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error", "failed to revoke approval")
	}

	// Dispatch DIDComm revoke notification
	recipients := []string{apr.From}
	if apr.Via != "" {
		recipients = append(recipients, apr.Via)
	}
	apr.State = models.ApprovalStateRevoked
	h.dispatchApprovalMessage(didcomm.TypeApprovalRevoke, entity.DID, recipients, apr, nil)

	return c.SendStatus(fiber.StatusNoContent)
}

// ============================================================
// HELPERS
// ============================================================

// approvalResponse builds the API response map for an approval, including server-side state fields.
func approvalResponse(apr *models.Approval) map[string]any {
	resp := map[string]any{
		"atap_approval": apr.AtapApproval,
		"id":            apr.ID,
		"created_at":    apr.CreatedAt,
		"from":          apr.From,
		"to":            apr.To,
		"subject":       apr.Subject,
		"signatures":    apr.Signatures,
		"state":         apr.State,
		"updated_at":    apr.UpdatedAt,
	}
	if apr.Via != "" {
		resp["via"] = apr.Via
	}
	if apr.Parent != "" {
		resp["parent"] = apr.Parent
	}
	if apr.ValidUntil != nil {
		resp["valid_until"] = apr.ValidUntil
	}
	if apr.TemplateURL != "" {
		resp["template_url"] = apr.TemplateURL
	}
	if apr.RespondedAt != nil {
		resp["responded_at"] = apr.RespondedAt
	}
	return resp
}
