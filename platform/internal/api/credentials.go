package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/atap-dev/atap/platform/internal/credential"
	"github.com/atap-dev/atap/platform/internal/models"
)

const (
	otpTTL        = 10 * time.Minute
	otpRateLimit  = 3
	otpRateWindow = time.Hour
	defaultListID = "1"
)

// generateOTP creates a 6-digit numeric OTP string.
func generateOTP() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	// Take a 6-digit number from the random bytes
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

// generateEncKey creates a 32-byte AES-256-GCM key.
func generateEncKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate enc key: %w", err)
	}
	return key, nil
}

// ensureEncKey ensures the entity has an encryption key, creating one if needed.
// Returns the key bytes.
func (h *Handler) ensureEncKey(ctx context.Context, entityID string) ([]byte, error) {
	key, err := h.credentialStore.GetEncKey(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get enc key: %w", err)
	}
	if key != nil {
		return key, nil
	}
	// No key yet — create one
	key, err = generateEncKey()
	if err != nil {
		return nil, err
	}
	if err := h.credentialStore.CreateEncKey(ctx, entityID, key); err != nil {
		return nil, fmt.Errorf("create enc key: %w", err)
	}
	return key, nil
}

// checkOTPRateLimit checks if the entity has sent too many OTPs in the last hour.
// Returns an error if rate limit exceeded. Skips limit if Redis is unavailable (best-effort).
func (h *Handler) checkOTPRateLimit(ctx context.Context, entityID string) error {
	rateKey := fmt.Sprintf("otp:rate:%s", entityID)
	count, err := h.redis.Incr(ctx, rateKey).Result()
	if err != nil {
		// Redis unavailable — skip rate limiting (best-effort)
		return nil
	}
	if count == 1 {
		// First request in window — set expiry
		h.redis.Expire(ctx, rateKey, otpRateWindow) //nolint:errcheck
	}
	if count > int64(otpRateLimit) {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

// storeOTP generates an OTP, stores it in Redis with 10-min TTL, and returns the OTP for logging/sending.
func (h *Handler) storeOTP(ctx context.Context, keyPrefix, entityID, contact string) (string, error) {
	otp, err := generateOTP()
	if err != nil {
		return "", err
	}
	otpKey := fmt.Sprintf("%s:%s:%s", keyPrefix, entityID, contact)
	if err := h.redis.Set(ctx, otpKey, otp, otpTTL).Err(); err != nil {
		return "", fmt.Errorf("store otp: %w", err)
	}
	return otp, nil
}

// verifyOTP retrieves and validates an OTP from Redis.
// Deletes the OTP on successful match (one-time use).
func (h *Handler) verifyOTP(ctx context.Context, keyPrefix, entityID, contact, providedOTP string) error {
	otpKey := fmt.Sprintf("%s:%s:%s", keyPrefix, entityID, contact)
	stored, err := h.redis.Get(ctx, otpKey).Result()
	if err == redis.Nil {
		return fmt.Errorf("otp expired or not found")
	}
	if err != nil {
		return fmt.Errorf("get otp: %w", err)
	}
	if stored != providedOTP {
		return fmt.Errorf("invalid otp")
	}
	// Delete OTP after use (one-time use)
	h.redis.Del(ctx, otpKey) //nolint:errcheck
	return nil
}

// platformIssuerDID returns the platform's issuer DID.
func (h *Handler) platformIssuerDID() string {
	return fmt.Sprintf("did:web:%s:server:platform", h.config.PlatformDomain)
}

// issueAndStoreCredential issues a VC JWT, encrypts it, and stores it in the credential table.
// Returns the plaintext JWT.
func (h *Handler) issueAndStoreCredential(
	ctx context.Context,
	entityID, entityDID, credType string,
	issueFunc func(entityDID, issuerDID, keyID string, statusIndex int, statusListID string) (string, error),
) (string, error) {
	// Ensure enc key exists
	encKey, err := h.ensureEncKey(ctx, entityID)
	if err != nil {
		return "", fmt.Errorf("ensure enc key: %w", err)
	}

	// Get next status index (falls back to 0 if status list not seeded)
	statusIndex, err := h.credentialStore.GetNextStatusIndex(ctx, defaultListID)
	if err != nil {
		// Status list not created yet — use index 0
		statusIndex = 0
	}

	// Issue the VC JWT
	issuerDID := h.platformIssuerDID()
	serverKeyID := fmt.Sprintf("did:web:%s:server:platform#key-1", h.config.PlatformDomain)
	jwtStr, err := issueFunc(entityDID, issuerDID, serverKeyID, statusIndex, defaultListID)
	if err != nil {
		return "", fmt.Errorf("issue credential: %w", err)
	}

	// Encrypt the JWT
	ct, err := credential.EncryptCredential(encKey, []byte(jwtStr))
	if err != nil {
		return "", fmt.Errorf("encrypt credential: %w", err)
	}

	// Persist credential
	cred := &models.Credential{
		ID:           newCredentialID(),
		EntityID:     entityID,
		Type:         credType,
		StatusIndex:  statusIndex,
		StatusListID: defaultListID,
		CredentialCT: ct,
		IssuedAt:     time.Now().UTC(),
	}
	if err := h.credentialStore.CreateCredential(ctx, cred); err != nil {
		return "", fmt.Errorf("store credential: %w", err)
	}

	return jwtStr, nil
}

// newCredentialID generates a new credential ID.
func newCredentialID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("crd_%x", b)
}

// requireEntity extracts the authenticated entity from the Fiber context.
// Returns nil and writes a 401 problem response if the entity is not present.
func requireEntity(c *fiber.Ctx) *models.Entity {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return nil
	}
	return entity
}

// StartEmailVerification handles POST /v1/credentials/email/start.
// Generates a 6-digit OTP, stores it in Redis with 10-min TTL, returns 200.
func (h *Handler) StartEmailVerification(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Email == "" {
		return problem(c, 400, "validation", "Validation Error", "email is required")
	}

	ctx := c.Context()

	// Rate limit: max 3 OTPs per hour per entity
	if err := h.checkOTPRateLimit(ctx, entity.ID); err != nil {
		return problem(c, 429, "rate-limit", "Too Many Requests", "OTP rate limit exceeded; try again later")
	}

	otp, err := h.storeOTP(ctx, "otp:email", entity.ID, req.Email)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to store email OTP")
		return problem(c, 500, "internal", "Internal Server Error", "failed to initiate email verification")
	}

	// v1.0 stub: log OTP (production would send via email provider)
	h.log.Info().Str("entity_id", entity.ID).Str("email", req.Email).
		Str("otp", otp).Msg("EMAIL OTP (stub — not sent)")

	return c.Status(200).JSON(fiber.Map{"message": "OTP sent to email"})
}

// VerifyEmail handles POST /v1/credentials/email/verify.
// Validates OTP, issues ATAPEmailVerification VC, returns 201 with credential JWT.
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

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

	if err := h.verifyOTP(ctx, "otp:email", entity.ID, req.Email, req.OTP); err != nil {
		return problem(c, 400, "invalid-otp", "Invalid OTP", err.Error())
	}

	email := req.Email
	platformPub := ed25519.PublicKey(h.platformKey.Public().(ed25519.PublicKey))
	jwtStr, err := h.issueAndStoreCredential(ctx, entity.ID, entity.DID, "ATAPEmailVerification",
		func(entityDID, issuerDID, keyID string, statusIndex int, statusListID string) (string, error) {
			return credential.IssueEmailVC(entityDID, email, issuerDID, keyID,
				platformPub, h.platformKey, statusIndex, statusListID)
		},
	)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to issue email credential")
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue credential")
	}

	return c.Status(201).JSON(fiber.Map{"credential": jwtStr})
}

// StartPhoneVerification handles POST /v1/credentials/phone/start.
// Generates a 6-digit OTP, stores it in Redis with 10-min TTL, returns 200.
// v1.0 stub: logs OTP instead of sending SMS.
func (h *Handler) StartPhoneVerification(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

	var req struct {
		Phone string `json:"phone"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Phone == "" {
		return problem(c, 400, "validation", "Validation Error", "phone is required")
	}

	ctx := c.Context()

	if err := h.checkOTPRateLimit(ctx, entity.ID); err != nil {
		return problem(c, 429, "rate-limit", "Too Many Requests", "OTP rate limit exceeded; try again later")
	}

	otp, err := h.storeOTP(ctx, "otp:phone", entity.ID, req.Phone)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to store phone OTP")
		return problem(c, 500, "internal", "Internal Server Error", "failed to initiate phone verification")
	}

	// v1.0 stub: log OTP (production would send via SMS provider)
	h.log.Info().Str("entity_id", entity.ID).Str("phone", req.Phone).
		Str("otp", otp).Msg("PHONE OTP (stub — not sent via SMS)")

	return c.Status(200).JSON(fiber.Map{"message": "OTP sent to phone"})
}

// VerifyPhone handles POST /v1/credentials/phone/verify.
// Validates OTP, issues ATAPPhoneVerification VC, returns 201 with credential JWT (CRD-02).
func (h *Handler) VerifyPhone(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

	var req struct {
		Phone string `json:"phone"`
		OTP   string `json:"otp"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.Phone == "" || req.OTP == "" {
		return problem(c, 400, "validation", "Validation Error", "phone and otp are required")
	}

	ctx := c.Context()

	if err := h.verifyOTP(ctx, "otp:phone", entity.ID, req.Phone, req.OTP); err != nil {
		return problem(c, 400, "invalid-otp", "Invalid OTP", err.Error())
	}

	phone := req.Phone
	platformPub := ed25519.PublicKey(h.platformKey.Public().(ed25519.PublicKey))
	jwtStr, err := h.issueAndStoreCredential(ctx, entity.ID, entity.DID, "ATAPPhoneVerification",
		func(entityDID, issuerDID, keyID string, statusIndex int, statusListID string) (string, error) {
			return credential.IssuePhoneVC(entityDID, phone, issuerDID, keyID,
				platformPub, h.platformKey, statusIndex, statusListID)
		},
	)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to issue phone credential")
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue credential")
	}

	return c.Status(201).JSON(fiber.Map{"credential": jwtStr})
}

// SubmitPersonhood handles POST /v1/credentials/personhood.
// Accepts {provider_token}, rejects any body containing "biometric" fields (PRV-04).
func (h *Handler) SubmitPersonhood(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

	// Parse body as raw map to detect biometric fields (PRV-04)
	var rawBody map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &rawBody); err != nil {
		return problem(c, 400, "bad-request", "Bad Request", "invalid JSON body")
	}

	// PRV-04: reject any key containing "biometric"
	for key := range rawBody {
		if strings.Contains(strings.ToLower(key), "biometric") {
			return problem(c, 400, "privacy-violation", "Privacy Violation",
				"biometric data is not accepted (PRV-04)")
		}
	}

	// Extract provider_token (optional for v1.0 assertion-based personhood)
	if raw, ok := rawBody["provider_token"]; ok {
		var providerToken string
		if err := json.Unmarshal(raw, &providerToken); err != nil || providerToken == "" {
			return problem(c, 400, "validation", "Validation Error", "provider_token must be a non-empty string")
		}
	}

	ctx := c.Context()

	platformPub := ed25519.PublicKey(h.platformKey.Public().(ed25519.PublicKey))
	jwtStr, err := h.issueAndStoreCredential(ctx, entity.ID, entity.DID, "ATAPPersonhood",
		func(entityDID, issuerDID, keyID string, statusIndex int, statusListID string) (string, error) {
			return credential.IssuePersonhoodVC(entityDID, issuerDID, keyID,
				platformPub, h.platformKey, statusIndex, statusListID)
		},
	)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to issue personhood credential")
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue credential")
	}

	return c.Status(201).JSON(fiber.Map{"credential": jwtStr})
}

// ListCredentials handles GET /v1/credentials.
// Returns the authenticated entity's credentials with decrypted VC JWTs.
func (h *Handler) ListCredentials(c *fiber.Ctx) error {
	entity := requireEntity(c)
	if entity == nil {
		return problem(c, 401, "unauthorized", "Unauthorized", "authentication required")
	}

	ctx := c.Context()

	encKey, err := h.credentialStore.GetEncKey(ctx, entity.ID)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to get enc key")
		return problem(c, 500, "internal", "Internal Server Error", "failed to list credentials")
	}

	creds, err := h.credentialStore.GetCredentials(ctx, entity.ID)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entity.ID).Msg("failed to get credentials")
		return problem(c, 500, "internal", "Internal Server Error", "failed to list credentials")
	}

	type credentialResponse struct {
		ID        string     `json:"id"`
		Type      string     `json:"type"`
		JWT       string     `json:"credential,omitempty"`
		IssuedAt  time.Time  `json:"issued_at"`
		RevokedAt *time.Time `json:"revoked_at,omitempty"`
	}

	result := make([]credentialResponse, 0, len(creds))
	for _, cred := range creds {
		resp := credentialResponse{
			ID:        cred.ID,
			Type:      cred.Type,
			IssuedAt:  cred.IssuedAt,
			RevokedAt: cred.RevokedAt,
		}

		// Decrypt the credential JWT if enc key and ciphertext are present
		if encKey != nil && len(cred.CredentialCT) > 0 {
			plaintext, decErr := credential.DecryptCredential(encKey, cred.CredentialCT)
			if decErr != nil {
				h.log.Warn().Err(decErr).Str("credential_id", cred.ID).
					Msg("failed to decrypt credential (crypto-shredded?)")
				// Skip unreadable credentials rather than failing the whole request
				continue
			}
			resp.JWT = string(plaintext)
		}

		result = append(result, resp)
	}

	return c.JSON(result)
}

// GetStatusList handles GET /v1/credentials/status/:listId.
// Returns a Bitstring Status List VC (public endpoint, no auth required).
func (h *Handler) GetStatusList(c *fiber.Ctx) error {
	listID := c.Params("listId")
	ctx := c.Context()

	sl, err := h.credentialStore.GetStatusList(ctx, listID)
	if err != nil {
		h.log.Error().Err(err).Str("list_id", listID).Msg("failed to get status list")
		return problem(c, 500, "internal", "Internal Server Error", "failed to retrieve status list")
	}
	if sl == nil {
		return problem(c, 404, "not-found", "Not Found",
			fmt.Sprintf("status list %q not found", listID))
	}

	// Encode the bitstring per W3C Bitstring Status List v1.0
	encoded, err := credential.EncodeStatusList(sl.Bits)
	if err != nil {
		h.log.Error().Err(err).Str("list_id", listID).Msg("failed to encode status list")
		return problem(c, 500, "internal", "Internal Server Error", "failed to encode status list")
	}

	statusListURL := fmt.Sprintf("https://%s/v1/credentials/status/%s", h.config.PlatformDomain, listID)
	issuerDID := h.platformIssuerDID()

	// Build a minimal Bitstring Status List VC (W3C Bitstring Status List v1.0 §5)
	vc := fiber.Map{
		"@context": []string{
			"https://www.w3.org/ns/credentials/v2",
			"https://www.w3.org/ns/credentials/status/bitstring-status-list/v1",
		},
		"id":        statusListURL,
		"type":      []string{"VerifiableCredential", "BitstringStatusListCredential"},
		"issuer":    issuerDID,
		"validFrom": sl.CreatedAt.UTC().Format(time.RFC3339),
		"credentialSubject": fiber.Map{
			"id":            statusListURL + "#list",
			"type":          "BitstringStatusList",
			"statusPurpose": "revocation",
			"encodedList":   encoded,
		},
	}

	return c.Status(200).JSON(vc, "application/vc+ld+json")
}
