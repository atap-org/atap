package api

import (
	"crypto/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// POST /v1/revocations
// ============================================================

// SubmitRevocation handles POST /v1/revocations.
// Requires authenticated entity with atap:revoke scope.
// The approver_did is taken from the authenticated entity's DID to prevent spoofing.
func (h *Handler) SubmitRevocation(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	var req struct {
		ApprovalID string     `json:"approval_id"`
		ValidUntil *time.Time `json:"valid_until,omitempty"`
		Signature  string     `json:"signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, fiber.StatusBadRequest, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.ApprovalID == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "approval_id is required")
	}
	if req.Signature == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "signature is required")
	}

	revokedAt := time.Now().UTC()

	// Compute expires_at: if valid_until is nil -> revokedAt + 60 minutes, else -> *valid_until
	var expiresAt time.Time
	if req.ValidUntil == nil {
		expiresAt = revokedAt.Add(60 * time.Minute)
	} else {
		expiresAt = req.ValidUntil.UTC()
	}

	// Generate revocation ID
	revocationID := "rev_" + ulid.MustNew(ulid.Timestamp(revokedAt), rand.Reader).String()

	rev := &models.Revocation{
		ID:          revocationID,
		ApprovalID:  req.ApprovalID,
		ApproverDID: entity.DID,
		RevokedAt:   revokedAt,
		ExpiresAt:   expiresAt,
	}

	if err := h.revocationStore.CreateRevocation(c.Context(), rev); err != nil {
		h.log.Error().Err(err).Str("approval_id", req.ApprovalID).Msg("failed to create revocation")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to create revocation")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           rev.ID,
		"approval_id":  rev.ApprovalID,
		"approver_did": rev.ApproverDID,
		"revoked_at":   rev.RevokedAt,
		"expires_at":   rev.ExpiresAt,
	})
}

// ============================================================
// GET /v1/revocations
// ============================================================

// ListRevocations handles GET /v1/revocations.
// Public endpoint — returns active (non-expired) revocations for the given approver DID.
func (h *Handler) ListRevocations(c *fiber.Ctx) error {
	entityDID := c.Query("entity")
	if entityDID == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error",
			"entity query parameter is required")
	}

	revocations, err := h.revocationStore.ListRevocations(c.Context(), entityDID)
	if err != nil {
		h.log.Error().Err(err).Str("entity_did", entityDID).Msg("failed to list revocations")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to list revocations")
	}

	// Return empty slice (not null) when no revocations found
	if revocations == nil {
		revocations = []models.Revocation{}
	}

	return c.JSON(fiber.Map{
		"entity":      entityDID,
		"revocations": revocations,
		"checked_at":  time.Now().UTC().Format(time.RFC3339),
	})
}
