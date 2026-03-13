package api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// validEntityTypes is the set of recognized entity types.
var validEntityTypes = map[string]bool{
	models.EntityTypeAgent:   true,
	models.EntityTypeMachine: true,
	models.EntityTypeHuman:   true,
	models.EntityTypeOrg:     true,
}

// CreateEntity handles POST /v1/entities.
// Accepts a public key and entity type, creates a did:web DID, and returns entity info.
// For agent/machine types, a client_secret is generated and returned once (never stored in plaintext).
func (h *Handler) CreateEntity(c *fiber.Ctx) error {
	var req models.CreateEntityRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}

	// Validate entity type
	if !validEntityTypes[req.Type] {
		return problem(c, 400, "validation", "Validation Error",
			fmt.Sprintf("invalid entity type %q: must be one of agent, machine, human, org", req.Type))
	}

	// Validate public key is present
	if req.PublicKey == "" {
		return problem(c, 400, "validation", "Validation Error", "public_key is required")
	}

	// Decode the public key from base64
	pubKey, err := crypto.DecodePublicKey(req.PublicKey)
	if err != nil {
		return problem(c, 400, "validation", "Validation Error",
			fmt.Sprintf("invalid public_key: %v", err))
	}

	// Agent requires a principal_did
	if req.Type == models.EntityTypeAgent && req.PrincipalDID == "" {
		return problem(c, 400, "validation", "Validation Error",
			"principal_did is required for agent entities")
	}

	// Derive entity ID
	var entityID string
	if req.Type == models.EntityTypeHuman {
		// Human ID is derived from the public key hash (deterministic)
		entityID = crypto.DeriveHumanID(pubKey)
	} else {
		entityID = crypto.NewEntityID()
	}

	// Build the DID
	did := crypto.BuildDID(h.config.PlatformDomain, req.Type, entityID)

	// Generate key ID
	keyID := crypto.NewKeyID(req.Type[:3])

	// Generate X25519 keypair for DIDComm key agreement
	x25519Priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate X25519 keypair")
		return problem(c, 500, "internal", "Internal Server Error", "failed to generate X25519 keypair")
	}

	now := time.Now().UTC()

	entity := &models.Entity{
		ID:               entityID,
		Type:             req.Type,
		DID:              did,
		PrincipalDID:     req.PrincipalDID,
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		X25519PublicKey:  x25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: x25519Priv.Bytes(),
		Name:             req.Name,
		TrustLevel:       models.TrustLevel0,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// For agent and machine types: generate client credentials
	var clientSecret string
	if req.Type == models.EntityTypeAgent || req.Type == models.EntityTypeMachine {
		secret, err := generateClientSecret()
		if err != nil {
			h.log.Error().Err(err).Msg("failed to generate client secret")
			return problem(c, 500, "internal", "Internal Server Error", "failed to generate credentials")
		}
		clientSecret = secret

		// Hash with bcrypt for storage (never store plaintext)
		hash, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to hash client secret")
			return problem(c, 500, "internal", "Internal Server Error", "failed to secure credentials")
		}
		entity.ClientSecretHash = string(hash)
	}

	// Persist entity
	if err := h.entityStore.CreateEntity(c.Context(), entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create entity")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create entity")
	}

	// Create initial key version (key_index=1, no valid_until)
	kv := &models.KeyVersion{
		ID:        newEntityKeyVersionID(),
		EntityID:  entityID,
		PublicKey: pubKey,
		KeyIndex:  1,
		ValidFrom: now,
		CreatedAt: now,
	}
	if err := h.keyVersionStore.CreateKeyVersion(c.Context(), kv); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create initial key version")
		// Don't fail the whole request -- entity was created. Log and continue.
	}

	resp := models.CreateEntityResponse{
		ID:           entityID,
		DID:          did,
		Type:         req.Type,
		Name:         req.Name,
		ClientSecret: clientSecret, // empty for human/org; populated for agent/machine
	}

	return c.Status(201).JSON(resp)
}

// GetEntity handles GET /v1/entities/{id}.
// Returns the public view of an entity including its DID. No auth required.
func (h *Handler) GetEntity(c *fiber.Ctx) error {
	id := c.Params("entityId")
	entity, err := h.entityStore.GetEntity(c.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", id).Msg("failed to get entity")
		return problem(c, 500, "internal", "Internal Server Error", "failed to retrieve entity")
	}
	if entity == nil {
		return problem(c, 404, "not-found", "Not Found", fmt.Sprintf("entity %q not found", id))
	}

	return c.JSON(entity)
}

// DeleteEntity handles DELETE /v1/entities/{id}.
// Removes the entity and all associated data (cascade). Auth will be enforced in Plan 04.
func (h *Handler) DeleteEntity(c *fiber.Ctx) error {
	id := c.Params("entityId")

	// Verify entity exists before deleting
	entity, err := h.entityStore.GetEntity(c.Context(), id)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", id).Msg("failed to check entity existence")
		return problem(c, 500, "internal", "Internal Server Error", "failed to delete entity")
	}
	if entity == nil {
		return problem(c, 404, "not-found", "Not Found", fmt.Sprintf("entity %q not found", id))
	}

	if err := h.entityStore.DeleteEntity(c.Context(), id); err != nil {
		h.log.Error().Err(err).Str("entity_id", id).Msg("failed to delete entity")
		return problem(c, 500, "internal", "Internal Server Error", "failed to delete entity")
	}

	return c.SendStatus(204)
}

// RotateKey handles POST /v1/entities/{id}/keys/rotate.
// Replaces the active key with a new public key, keeping history for DID Document inclusion.
// Auth will be enforced in Plan 04.
func (h *Handler) RotateKey(c *fiber.Ctx) error {
	id := c.Params("entityId")

	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.PublicKey == "" {
		return problem(c, 400, "validation", "Validation Error", "public_key is required")
	}

	newPubKey, err := crypto.DecodePublicKey(req.PublicKey)
	if err != nil {
		return problem(c, 400, "validation", "Validation Error",
			fmt.Sprintf("invalid public_key: %v", err))
	}

	// Verify entity exists
	entity, err := h.entityStore.GetEntity(c.Context(), id)
	if err != nil {
		return problem(c, 500, "internal", "Internal Server Error", "failed to retrieve entity")
	}
	if entity == nil {
		return problem(c, 404, "not-found", "Not Found", fmt.Sprintf("entity %q not found", id))
	}

	newKV, err := h.keyVersionStore.RotateKey(c.Context(), id, newPubKey)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", id).Msg("failed to rotate key")
		return problem(c, 500, "internal", "Internal Server Error", "failed to rotate key")
	}

	return c.JSON(newKV)
}

// generateClientSecret creates a secure 32-byte random secret encoded as base64url.
// This is the "atap_" prefixed token per CLAUDE.md token convention.
func generateClientSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate client secret: %w", err)
	}
	return "atap_" + base64.URLEncoding.EncodeToString(b), nil
}

// newEntityKeyVersionID generates a new key version ID for initial entity registration.
func newEntityKeyVersionID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("kv_%x", b)
}
