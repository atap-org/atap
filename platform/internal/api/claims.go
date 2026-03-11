package api

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateClaim handles POST /v1/claims -- authenticated agent creates an invite claim.
func (h *Handler) CreateClaim(c *fiber.Ctx) error {
	entity := c.Locals("entity").(*models.Entity)

	claimID := crypto.NewClaimID()
	code := crypto.GenerateClaimCode()
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	claim := &models.Claim{
		ID:        claimID,
		Code:      code,
		CreatorID: entity.ID,
		Status:    models.ClaimStatusPending,
		CreatedAt: now,
		ExpiresAt: &expiresAt,
	}

	if err := h.claimStore.CreateClaim(c.Context(), claim); err != nil {
		h.log.Error().Err(err).Str("claim_id", claimID).Msg("failed to create claim")
		return problem(c, 500, "store_error", "Failed to create claim", "")
	}

	h.log.Info().
		Str("claim_id", claimID).
		Str("creator_id", entity.ID).
		Msg("claim created")

	return c.Status(201).JSON(models.CreateClaimResponse{
		ID:   claimID,
		Code: code,
		Link: fmt.Sprintf("https://link.atap.app/claim/%s", code),
	})
}

// GetClaim handles GET /v1/claims/:code -- public endpoint to check claim status.
func (h *Handler) GetClaim(c *fiber.Ctx) error {
	code := c.Params("code")

	claim, err := h.claimStore.GetClaimByCode(c.Context(), code)
	if err != nil {
		h.log.Error().Err(err).Str("code", code).Msg("failed to get claim")
		return problem(c, 500, "query_failed", "Failed to get claim", "")
	}
	if claim == nil {
		return problem(c, 404, "not_found", "Claim not found", "")
	}

	return c.JSON(claim)
}
