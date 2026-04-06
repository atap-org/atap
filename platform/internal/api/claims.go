package api

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/web"
)

const claimTTL = 24 * time.Hour

// validClaimScopes is the set of scopes an agent may request in a claim.
var validClaimScopes = map[string]bool{
	"atap:inbox":  true,
	"atap:send":   true,
	"atap:revoke": true,
	"atap:manage": true,
}

// CreateClaim handles POST /v1/claims.
// Authenticated agent creates a claim that produces a short code + URL for a human to open.
func (h *Handler) CreateClaim(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}
	if entity.Type != models.EntityTypeAgent {
		return problem(c, 403, "forbidden", "Forbidden", "only agents can create claims")
	}

	var req models.CreateClaimRequest
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}

	// Validate scopes
	for _, s := range req.Scopes {
		if !validClaimScopes[s] {
			return problem(c, 400, "validation", "Validation Error",
				fmt.Sprintf("invalid scope %q", s))
		}
	}
	// Default scopes if none provided
	if len(req.Scopes) == 0 {
		req.Scopes = []string{"atap:inbox", "atap:send"}
	}

	now := time.Now().UTC()
	claim := &models.Claim{
		ID:          crypto.NewClaimID(),
		Code:        crypto.NewClaimCode(),
		AgentID:     entity.ID,
		AgentName:   req.Name,
		Description: req.Description,
		Scopes:      req.Scopes,
		Status:      models.ClaimStatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(claimTTL),
	}

	// Use entity name as fallback
	if claim.AgentName == "" {
		claim.AgentName = entity.Name
	}

	if err := h.claimStore.CreateClaim(c.Context(), claim); err != nil {
		h.log.Error().Err(err).Str("agent_id", entity.ID).Msg("failed to create claim")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create claim")
	}

	claimURL := fmt.Sprintf("https://%s/claim/%s", h.config.PlatformDomain, claim.Code)

	h.log.Info().
		Str("claim_id", claim.ID).
		Str("code", claim.Code).
		Str("agent_id", entity.ID).
		Msg("claim created")

	return c.Status(201).JSON(models.CreateClaimResponse{
		ID:        claim.ID,
		Code:      claim.Code,
		URL:       claimURL,
		ExpiresAt: claim.ExpiresAt,
	})
}

// ClaimPage handles GET /claim/:code.
// Serves the HTML landing page where a human can authenticate and approve the claim.
func (h *Handler) ClaimPage(c *fiber.Ctx) error {
	code := strings.ToUpper(c.Params("code"))

	claim, err := h.claimStore.GetClaimByCode(c.Context(), code)
	if err != nil {
		h.log.Error().Err(err).Str("code", code).Msg("failed to get claim")
		return c.Status(500).SendString("Something went wrong. Please try again.")
	}
	if claim == nil {
		return c.Status(404).SendString("Claim not found.")
	}
	if claim.Status != models.ClaimStatusPending {
		return c.Status(410).SendString("This claim has already been used or expired.")
	}
	if time.Now().After(claim.ExpiresAt) {
		return c.Status(410).SendString("This claim has expired.")
	}

	// Render the claim page from embedded template
	tmpl, err := template.ParseFS(web.Templates, "templates/claim.html")
	if err != nil {
		h.log.Error().Err(err).Msg("failed to parse claim template")
		return c.Status(500).SendString("Internal server error")
	}

	data := map[string]interface{}{
		"Code":        claim.Code,
		"AgentName":   claim.AgentName,
		"Description": claim.Description,
		"Scopes":      claim.Scopes,
		"ExpiresAt":   claim.ExpiresAt.Format(time.RFC3339),
		"Domain":      h.config.PlatformDomain,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		h.log.Error().Err(err).Msg("failed to render claim template")
		return c.Status(500).SendString("Internal server error")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.Send(buf.Bytes())
}

// ClaimStartAuth handles POST /claim/:code/auth.
// Starts email OTP for the claim flow (no DPoP auth required — this IS the onboarding).
func (h *Handler) ClaimStartAuth(c *fiber.Ctx) error {
	code := strings.ToUpper(c.Params("code"))

	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Email == "" {
		return problem(c, 400, "validation", "Validation Error", "email is required")
	}

	// Verify claim is still valid
	claim, err := h.claimStore.GetClaimByCode(c.Context(), code)
	if err != nil || claim == nil {
		return problem(c, 404, "not-found", "Not Found", "claim not found")
	}
	if claim.Status != models.ClaimStatusPending || time.Now().After(claim.ExpiresAt) {
		return problem(c, 410, "expired", "Gone", "claim is no longer valid")
	}

	ctx := c.Context()

	// Rate limit by claim code (not entity — human doesn't have one yet)
	rateKey := fmt.Sprintf("otp:rate:claim:%s", code)
	count, rateErr := h.redis.Incr(ctx, rateKey).Result()
	if rateErr == nil {
		if count == 1 {
			h.redis.Expire(ctx, rateKey, time.Hour) //nolint:errcheck
		}
		if count > 5 {
			return problem(c, 429, "rate-limit", "Too Many Requests", "too many OTP requests for this claim")
		}
	}

	// Generate and store OTP keyed by claim code + email
	otp, err := generateOTP()
	if err != nil {
		return problem(c, 500, "internal", "Internal Server Error", "failed to generate OTP")
	}
	otpKey := fmt.Sprintf("otp:claim:%s:%s", code, req.Email)
	if err := h.redis.Set(ctx, otpKey, otp, otpTTL).Err(); err != nil {
		return problem(c, 500, "internal", "Internal Server Error", "failed to store OTP")
	}

	// v1.0 stub: log OTP (production would send via email provider)
	h.log.Info().Str("code", code).Str("email", req.Email).
		Str("otp", otp).Msg("CLAIM OTP (stub — not sent)")

	return c.Status(200).JSON(fiber.Map{"message": "OTP sent to email"})
}

// ClaimApprove handles POST /claim/:code/approve.
// Verifies email OTP, creates human entity (server-side custody), redeems claim,
// and binds agent to human.
func (h *Handler) ClaimApprove(c *fiber.Ctx) error {
	code := strings.ToUpper(c.Params("code"))

	var req struct {
		Email string `json:"email"`
		OTP   string `json:"otp"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Email == "" || req.OTP == "" {
		return problem(c, 400, "validation", "Validation Error", "email and otp are required")
	}

	ctx := c.Context()

	// Verify claim is still valid
	claim, err := h.claimStore.GetClaimByCode(ctx, code)
	if err != nil || claim == nil {
		return problem(c, 404, "not-found", "Not Found", "claim not found")
	}
	if claim.Status != models.ClaimStatusPending || time.Now().After(claim.ExpiresAt) {
		return problem(c, 410, "expired", "Gone", "claim is no longer valid")
	}

	// Verify OTP
	otpKey := fmt.Sprintf("otp:claim:%s:%s", code, req.Email)
	stored, otpErr := h.redis.Get(ctx, otpKey).Result()
	if otpErr != nil || stored != req.OTP {
		return problem(c, 400, "invalid-otp", "Invalid OTP", "invalid or expired OTP")
	}
	h.redis.Del(ctx, otpKey) //nolint:errcheck

	// Check if a human entity already exists for this email (via Redis session lookup)
	// For v1: we create a new human entity with server-side custody keys.
	// The email becomes an attestation, not the identity.

	// Generate server-side custody keypair for the human
	pubKey, privKey, err := crypto.GenerateKeyPair()
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate human keypair")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create identity")
	}

	entityID := crypto.DeriveHumanID(pubKey)
	did := crypto.BuildDID(h.config.PlatformDomain, models.EntityTypeHuman, entityID)
	keyID := crypto.NewKeyID("hum")

	// Generate X25519 for DIDComm
	x25519Priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate X25519 keypair for human")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create identity")
	}

	now := time.Now().UTC()
	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              did,
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		X25519PublicKey:  x25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: x25519Priv.Bytes(),
		Name:             req.Email, // Use email as display name for now
		TrustLevel:       models.TrustLevel0,
		Registry:         h.config.PlatformDomain,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// For server-side custody: generate client credentials so the human
	// can authenticate later (via the web UI) without needing the private key directly.
	secret, err := generateClientSecret()
	if err != nil {
		h.log.Error().Err(err).Msg("failed to generate client secret for human")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create identity")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to hash client secret")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create identity")
	}
	entity.ClientSecretHash = string(hash)

	// Persist human entity
	if err := h.entityStore.CreateEntity(ctx, entity); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create human entity")
		return problem(c, 500, "internal", "Internal Server Error", "failed to create identity")
	}

	// Create initial key version
	kv := &models.KeyVersion{
		ID:        newEntityKeyVersionID(),
		EntityID:  entityID,
		PublicKey: pubKey,
		KeyIndex:  1,
		ValidFrom: now,
		CreatedAt: now,
	}
	if err := h.keyVersionStore.CreateKeyVersion(ctx, kv); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to create key version for human")
	}

	// Redeem the claim
	redeemed, err := h.claimStore.RedeemClaim(ctx, code, entityID)
	if err != nil || !redeemed {
		h.log.Error().Err(err).Str("code", code).Msg("failed to redeem claim")
		return problem(c, 409, "conflict", "Conflict", "claim was already redeemed")
	}

	// Bind agent to human: set principal_did on the agent entity
	if err := h.claimStore.SetEntityPrincipalDID(ctx, claim.AgentID, did); err != nil {
		h.log.Error().Err(err).Str("agent_id", claim.AgentID).Msg("failed to bind agent to human")
		// Non-fatal: claim is redeemed, binding can be retried
	}

	// Notify agent via DIDComm inbox
	agentEntity, _ := h.entityStore.GetEntity(ctx, claim.AgentID)
	if agentEntity != nil && h.messageStore != nil {
		payload := fmt.Sprintf(`{"claim_id":"%s","human_did":"%s"}`, claim.ID, did)
		msg := &models.DIDCommMessage{
			ID:           newDIDCommMessageID(),
			RecipientDID: agentEntity.DID,
			SenderDID:    fmt.Sprintf("did:web:%s:server:platform", h.config.PlatformDomain),
			MessageType:  "https://atap.dev/protocols/claim/1.0/redeemed",
			Payload:      []byte(payload),
			State:        "pending",
			CreatedAt:    now,
		}
		if queueErr := h.messageStore.QueueMessage(ctx, msg); queueErr != nil {
			h.log.Warn().Err(queueErr).Str("agent_id", claim.AgentID).
				Msg("failed to notify agent of claim redemption (non-fatal)")
		}
	}

	// Store the human's session token in Redis so the web UI can authenticate
	// them for future visits. The token is a short-lived session (24h).
	sessionToken := fmt.Sprintf("clmsess_%x", mustRandBytes(16))
	sessionKey := fmt.Sprintf("session:claim:%s", sessionToken)
	h.redis.Set(ctx, sessionKey, entityID, 24*time.Hour) //nolint:errcheck

	h.log.Info().
		Str("claim_id", claim.ID).
		Str("code", code).
		Str("human_id", entityID).
		Str("human_did", did).
		Str("agent_id", claim.AgentID).
		Msg("claim redeemed — human entity created, agent bound")

	return c.Status(200).JSON(fiber.Map{
		"status":      "approved",
		"human_did":   did,
		"agent_name":  claim.AgentName,
		"private_key": base64.StdEncoding.EncodeToString(privKey),
	})
}

// mustRandBytes generates n random bytes or panics.
func mustRandBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b) //nolint:errcheck
	return b
}
