package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

// RegisterHuman handles POST /v1/register/human -- public endpoint for human registration via claim.
func (h *Handler) RegisterHuman(c *fiber.Ctx) error {
	var req models.HumanRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	// Validate required fields
	if req.PublicKey == "" {
		return problem(c, 400, "invalid_request", "Public key is required", "")
	}
	if req.ClaimCode == "" {
		return problem(c, 400, "invalid_request", "Claim code is required", "")
	}

	// Validate claim exists and is pending
	claim, err := h.claimStore.GetClaimByCode(c.Context(), req.ClaimCode)
	if err != nil {
		h.log.Error().Err(err).Str("code", req.ClaimCode).Msg("failed to get claim")
		return problem(c, 500, "query_failed", "Failed to validate claim", "")
	}
	if claim == nil {
		return problem(c, 404, "not_found", "Claim not found", "")
	}
	if claim.Status != models.ClaimStatusPending {
		return problem(c, 409, "claim_not_available", "Claim has already been redeemed or expired", "")
	}

	// Decode public key
	pubKey, err := crypto.DecodePublicKey(req.PublicKey)
	if err != nil {
		return problem(c, 400, "invalid_request", "Invalid public key", err.Error())
	}

	// Derive human ID from public key
	humanID := crypto.DeriveHumanID(pubKey)
	keyID := crypto.NewKeyID(humanID[:8])
	now := time.Now().UTC()

	entity := &models.Entity{
		ID:               humanID,
		Type:             models.EntityTypeHuman,
		URI:              fmt.Sprintf("human://%s", humanID),
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		TrustLevel:       models.TrustLevel0,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        now,
	}

	if err := h.entityStore.CreateEntity(c.Context(), entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", humanID).Msg("failed to create human entity")
		return problem(c, 500, "creation_failed", "Failed to create entity", "")
	}

	// Redeem the claim
	if err := h.claimStore.RedeemClaim(c.Context(), req.ClaimCode, entity.ID); err != nil {
		if errors.Is(err, store.ErrClaimNotAvailable) {
			return problem(c, 409, "claim_already_redeemed", "Claim has already been redeemed", "")
		}
		h.log.Error().Err(err).Str("code", req.ClaimCode).Msg("failed to redeem claim")
		return problem(c, 500, "store_error", "Failed to redeem claim", "")
	}

	// Create delegation: human (delegator) delegates to agent (delegate = claim creator)
	delegation := &models.Delegation{
		ID:          crypto.NewDelegationID(),
		DelegatorID: entity.ID,
		DelegateID:  claim.CreatorID,
		Scope:       json.RawMessage(`{"permissions":["*"]}`),
		CreatedAt:   now,
	}

	if err := h.delegationStore.CreateDelegation(c.Context(), delegation); err != nil {
		h.log.Error().Err(err).Str("delegation_id", delegation.ID).Msg("failed to create delegation")
		return problem(c, 500, "store_error", "Failed to create delegation", "")
	}

	h.log.Info().
		Str("entity_id", humanID).
		Str("claim_code", req.ClaimCode).
		Str("delegation_id", delegation.ID).
		Msg("human registered via claim")

	return c.Status(201).JSON(models.HumanRegisterResponse{
		Entity: *entity,
		KeyID:  keyID,
	})
}
