package api

import (
	gocrypto "crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	dpop "github.com/AxisCommunications/go-dpop"
	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/atap-dev/atap/platform/internal/models"
)

// cryptoSHA256 is the crypto.Hash value for SHA-256, used for JWK thumbprints.
const cryptoSHA256 = gocrypto.SHA256

// validScopes is the set of recognized ATAP token scopes.
var validScopes = map[string]bool{
	"atap:inbox":   true,
	"atap:send":    true,
	"atap:approve": true,
	"atap:manage":  true,
}

// allScopes is the default scope list when no scope is requested.
var allScopes = []string{"atap:inbox", "atap:send", "atap:approve", "atap:manage"}

// ============================================================
// TOKEN ENDPOINT
// ============================================================

// Token handles POST /v1/oauth/token.
// Supports grant_type=client_credentials (agents/machines) and grant_type=authorization_code (humans).
func (h *Handler) Token(c *fiber.Ctx) error {
	// Require DPoP header before anything else
	dpopHeader := c.Get("DPoP")
	if dpopHeader == "" {
		return problem(c, 400, "invalid_dpop_proof", "Missing DPoP Proof", "DPoP header is required at the token endpoint")
	}

	grantType := c.FormValue("grant_type")
	switch grantType {
	case "client_credentials":
		return h.handleClientCredentials(c, dpopHeader)
	case "authorization_code":
		return h.handleAuthorizationCode(c, dpopHeader)
	default:
		return problem(c, 400, "unsupported_grant_type", "Unsupported Grant Type",
			fmt.Sprintf("grant_type %q is not supported; use client_credentials or authorization_code", grantType))
	}
}

// handleClientCredentials implements the OAuth 2.1 Client Credentials grant (AUTH-02).
func (h *Handler) handleClientCredentials(c *fiber.Ctx, dpopHeader string) error {
	clientID := c.FormValue("client_id")
	clientSecret := c.FormValue("client_secret")

	if clientID == "" {
		return problem(c, 401, "invalid_client", "Invalid Client", "client_id is required")
	}

	// Parse and validate DPoP proof at token endpoint (registers the key)
	tokenURL := fmt.Sprintf("https://%s/v1/oauth/token", h.config.PlatformDomain)
	proof, err := parseDPoPProofAtTokenEndpoint(dpopHeader, tokenURL)
	if err != nil {
		return problem(c, 400, "invalid_dpop_proof", "Invalid DPoP Proof", err.Error())
	}

	// Look up entity by DID
	entity, err := h.entityStore.GetEntityByDID(c.Context(), clientID)
	if err != nil {
		h.log.Error().Err(err).Str("client_id", clientID).Msg("failed to look up entity by DID")
		return problem(c, 500, "internal", "Internal Server Error", "failed to process request")
	}
	if entity == nil {
		return problem(c, 401, "invalid_client", "Invalid Client", "client not found")
	}

	// Human entities cannot use client_credentials (must use authorization_code)
	if entity.Type == models.EntityTypeHuman || entity.Type == models.EntityTypeOrg {
		return problem(c, 400, "invalid_grant", "Invalid Grant",
			"human and org entities must use authorization_code grant; client_credentials is for agent and machine entities only")
	}

	// Verify client_secret
	if entity.ClientSecretHash == "" {
		return problem(c, 401, "invalid_client", "Invalid Client", "client has no credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(entity.ClientSecretHash), []byte(clientSecret)); err != nil {
		return problem(c, 401, "invalid_client", "Invalid Client", "invalid client_secret")
	}

	// Parse and validate requested scopes
	scopes, err := parseScopes(c.FormValue("scope"))
	if err != nil {
		return problem(c, 400, "invalid_scope", "Invalid Scope", err.Error())
	}

	// Compute JWK thumbprint from DPoP proof's public key
	jkt := proof.PublicKey()

	// Issue access token
	accessJTI := uuid.NewString()
	accessToken, err := issueJWT(h.platformKey, h.config.PlatformDomain, entity.DID, accessJTI, jkt, scopes, 1*time.Hour)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to issue access token")
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue token")
	}

	// Store access token metadata
	now := time.Now().UTC()
	accessTokenRecord := &models.OAuthToken{
		ID:        accessJTI,
		EntityID:  entity.ID,
		TokenType: "access",
		Scope:     scopes,
		DPoPJKT:   jkt,
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}
	if err := h.oauthTokenStore.CreateOAuthToken(c.Context(), accessTokenRecord); err != nil {
		h.log.Error().Err(err).Msg("failed to store access token")
		return problem(c, 500, "internal", "Internal Server Error", "failed to store token")
	}

	// Issue refresh token
	refreshJTI := uuid.NewString()
	refreshToken, err := issueJWT(h.platformKey, h.config.PlatformDomain, entity.DID, refreshJTI, jkt, scopes, 90*24*time.Hour)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to issue refresh token")
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue refresh token")
	}

	refreshTokenRecord := &models.OAuthToken{
		ID:        refreshJTI,
		EntityID:  entity.ID,
		TokenType: "refresh",
		Scope:     scopes,
		DPoPJKT:   jkt,
		ExpiresAt: now.Add(90 * 24 * time.Hour),
		CreatedAt: now,
	}
	if err := h.oauthTokenStore.CreateOAuthToken(c.Context(), refreshTokenRecord); err != nil {
		h.log.Warn().Err(err).Msg("failed to store refresh token")
		// Non-fatal: access token was issued
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"token_type":    "DPoP",
		"expires_in":    3600,
		"scope":         strings.Join(scopes, " "),
		"refresh_token": refreshToken,
	})
}

// handleAuthorizationCode implements the OAuth 2.1 Authorization Code exchange (AUTH-03).
func (h *Handler) handleAuthorizationCode(c *fiber.Ctx, dpopHeader string) error {
	code := c.FormValue("code")
	redirectURI := c.FormValue("redirect_uri")
	codeVerifier := c.FormValue("code_verifier")

	if code == "" || codeVerifier == "" {
		return problem(c, 400, "invalid_request", "Invalid Request", "code and code_verifier are required")
	}

	// Parse DPoP proof
	tokenURL := fmt.Sprintf("https://%s/v1/oauth/token", h.config.PlatformDomain)
	proof, err := parseDPoPProofAtTokenEndpoint(dpopHeader, tokenURL)
	if err != nil {
		return problem(c, 400, "invalid_dpop_proof", "Invalid DPoP Proof", err.Error())
	}

	// Atomically redeem the authorization code
	authCode, err := h.oauthTokenStore.RedeemAuthCode(c.Context(), code)
	if err != nil {
		h.log.Error().Err(err).Str("code", code).Msg("failed to redeem auth code")
		return problem(c, 500, "internal", "Internal Server Error", "failed to process auth code")
	}
	if authCode == nil {
		return problem(c, 400, "invalid_grant", "Invalid Grant", "authorization code is invalid, expired, or already used")
	}

	// Verify redirect_uri matches
	if redirectURI != authCode.RedirectURI {
		return problem(c, 400, "invalid_grant", "Invalid Grant", "redirect_uri does not match authorization request")
	}

	// Verify PKCE S256 challenge
	if !verifyPKCE(authCode.CodeChallenge, codeVerifier) {
		return problem(c, 400, "invalid_grant", "Invalid Grant", "code_verifier does not match code_challenge")
	}

	// Verify DPoP binding: JKT must match the one stored at authorization time
	jkt := proof.PublicKey()
	if jkt != authCode.DPoPJKT {
		return problem(c, 400, "invalid_grant", "Invalid Grant", "DPoP key does not match authorization request")
	}

	// Look up entity
	entity, err := h.entityStore.GetEntity(c.Context(), authCode.EntityID)
	if err != nil || entity == nil {
		return problem(c, 400, "invalid_grant", "Invalid Grant", "entity not found")
	}

	// Issue access token
	now := time.Now().UTC()
	accessJTI := uuid.NewString()
	accessToken, err := issueJWT(h.platformKey, h.config.PlatformDomain, entity.DID, accessJTI, jkt, authCode.Scope, 1*time.Hour)
	if err != nil {
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue token")
	}

	if err := h.oauthTokenStore.CreateOAuthToken(c.Context(), &models.OAuthToken{
		ID:        accessJTI,
		EntityID:  entity.ID,
		TokenType: "access",
		Scope:     authCode.Scope,
		DPoPJKT:   jkt,
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}); err != nil {
		h.log.Error().Err(err).Msg("failed to store access token")
		return problem(c, 500, "internal", "Internal Server Error", "failed to store token")
	}

	// Issue refresh token
	refreshJTI := uuid.NewString()
	refreshToken, err := issueJWT(h.platformKey, h.config.PlatformDomain, entity.DID, refreshJTI, jkt, authCode.Scope, 90*24*time.Hour)
	if err != nil {
		return problem(c, 500, "internal", "Internal Server Error", "failed to issue refresh token")
	}

	if err := h.oauthTokenStore.CreateOAuthToken(c.Context(), &models.OAuthToken{
		ID:        refreshJTI,
		EntityID:  entity.ID,
		TokenType: "refresh",
		Scope:     authCode.Scope,
		DPoPJKT:   jkt,
		ExpiresAt: now.Add(90 * 24 * time.Hour),
		CreatedAt: now,
	}); err != nil {
		h.log.Warn().Err(err).Msg("failed to store refresh token")
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"token_type":    "DPoP",
		"expires_in":    3600,
		"scope":         strings.Join(authCode.Scope, " "),
		"refresh_token": refreshToken,
	})
}

// ============================================================
// AUTHORIZE ENDPOINT
// ============================================================

// Authorize handles GET /v1/oauth/authorize.
// Implements the Authorization Code + PKCE flow for human entities (AUTH-03).
func (h *Handler) Authorize(c *fiber.Ctx) error {
	// Parse query parameters
	responseType := c.Query("response_type")
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	state := c.Query("state")

	// Validate response_type
	if responseType != "code" {
		return problem(c, 400, "unsupported_response_type", "Unsupported Response Type",
			"response_type must be 'code'")
	}

	// Validate PKCE S256 (plain rejected per Pitfall 6)
	if codeChallengeMethod != "S256" {
		return problem(c, 400, "invalid_request", "Invalid Request",
			"code_challenge_method must be S256; plain is not supported")
	}

	// Validate code_challenge is present
	if codeChallenge == "" {
		return problem(c, 400, "invalid_request", "Invalid Request",
			"code_challenge is required")
	}

	// Require DPoP header to bind the authorization request to a key
	dpopHeader := c.Get("DPoP")
	if dpopHeader == "" {
		return problem(c, 400, "invalid_dpop_proof", "Missing DPoP Proof",
			"DPoP header is required at the authorization endpoint")
	}

	// Parse DPoP proof for GET request
	authorizeURL := fmt.Sprintf("https://%s/v1/oauth/authorize", h.config.PlatformDomain)
	proof, err := parseDPoPProofForMethod(dpopHeader, "GET", authorizeURL)
	if err != nil {
		return problem(c, 400, "invalid_dpop_proof", "Invalid DPoP Proof", err.Error())
	}

	// Look up entity by DID
	entity, err := h.entityStore.GetEntityByDID(c.Context(), clientID)
	if err != nil {
		h.log.Error().Err(err).Str("client_id", clientID).Msg("failed to look up entity")
		return problem(c, 500, "internal", "Internal Server Error", "failed to process request")
	}
	if entity == nil {
		return problem(c, 400, "invalid_client", "Invalid Client", "client not found")
	}

	// Only human and org entities use Authorization Code flow
	if entity.Type != models.EntityTypeHuman && entity.Type != models.EntityTypeOrg {
		return problem(c, 400, "unauthorized_client", "Unauthorized Client",
			"only human and org entities may use authorization_code grant")
	}

	// Parse and validate scopes
	scopes, err := parseScopes(scope)
	if err != nil {
		return problem(c, 400, "invalid_scope", "Invalid Scope", err.Error())
	}

	// Extract JKT from DPoP proof for binding
	jkt := proof.PublicKey()

	// Generate authorization code
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		h.log.Error().Err(err).Msg("failed to generate auth code")
		return problem(c, 500, "internal", "Internal Server Error", "failed to generate authorization code")
	}
	authCode := base64.RawURLEncoding.EncodeToString(codeBytes)

	now := time.Now().UTC()
	authCodeRecord := &models.OAuthAuthCode{
		Code:          authCode,
		EntityID:      entity.ID,
		RedirectURI:   redirectURI,
		Scope:         scopes,
		CodeChallenge: codeChallenge,
		DPoPJKT:       jkt,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedAt:     now,
	}
	if err := h.oauthTokenStore.CreateAuthCode(c.Context(), authCodeRecord); err != nil {
		h.log.Error().Err(err).Msg("failed to store auth code")
		return problem(c, 500, "internal", "Internal Server Error", "failed to store authorization code")
	}

	// Build redirect URL
	redirectTarget := redirectURI + "?code=" + authCode
	if state != "" {
		redirectTarget += "&state=" + state
	}

	return c.Redirect(redirectTarget, 302)
}

// ============================================================
// JWT HELPERS
// ============================================================

// issueJWT signs and returns a JWT access or refresh token.
// Claims include: sub (entity DID), iss (platform domain), iat, exp, jti, cnf.jkt, scope.
func issueJWT(
	platformPrivKey ed25519.PrivateKey,
	issuer, sub, jti, jkt string,
	scopes []string,
	ttl time.Duration,
) (string, error) {
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: platformPrivKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", fmt.Errorf("create JWT signer: %w", err)
	}

	now := time.Now()
	standardClaims := josejwt.Claims{
		Subject:  sub,
		Issuer:   issuer,
		IssuedAt: josejwt.NewNumericDate(now),
		Expiry:   josejwt.NewNumericDate(now.Add(ttl)),
		ID:       jti,
	}

	dpopClaims := struct {
		CNF struct {
			JKT string `json:"jkt"`
		} `json:"cnf"`
		Scope string `json:"scope"`
	}{}
	dpopClaims.CNF.JKT = jkt
	dpopClaims.Scope = strings.Join(scopes, " ")

	return josejwt.Signed(sig).Claims(standardClaims).Claims(dpopClaims).Serialize()
}

// jwkThumbprint computes the RFC 7638 JWK thumbprint for an Ed25519 public key.
// Returns a base64url-encoded SHA-256 hash of the canonical JWK representation.
func jwkThumbprint(pub ed25519.PublicKey) (string, error) {
	jwk := jose.JSONWebKey{Key: pub, Algorithm: string(jose.EdDSA)}
	tb, err := jwk.Thumbprint(cryptoSHA256)
	if err != nil {
		return "", fmt.Errorf("compute JWK thumbprint: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(tb), nil
}

// parseDPoPProofAtTokenEndpoint parses a DPoP proof JWT for use at the token or authorize endpoint.
// At the token/authorize endpoint, the DPoP public key registers itself — we don't compare to a stored key.
func parseDPoPProofAtTokenEndpoint(proofHeader, rawURL string) (*dpop.Proof, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse token URL: %w", err)
	}
	window := 60 * time.Second
	proof, err := dpop.Parse(proofHeader, dpop.POST, u, dpop.ParseOptions{
		TimeWindow: &window,
	})
	if err != nil {
		return nil, fmt.Errorf("parse DPoP proof: %w", err)
	}
	return proof, nil
}

// parseDPoPProofForMethod parses a DPoP proof JWT for a specific HTTP method and URL.
func parseDPoPProofForMethod(proofHeader, method, rawURL string) (*dpop.Proof, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	window := 60 * time.Second
	proof, err := dpop.Parse(proofHeader, dpop.HTTPVerb(method), u, dpop.ParseOptions{
		TimeWindow: &window,
	})
	if err != nil {
		return nil, fmt.Errorf("parse DPoP proof: %w", err)
	}
	return proof, nil
}

// parseScopes parses and validates the space-separated scope string.
// Returns all scopes if the input is empty.
func parseScopes(scopeStr string) ([]string, error) {
	if scopeStr == "" {
		return allScopes, nil
	}
	parts := strings.Fields(scopeStr)
	for _, s := range parts {
		if !validScopes[s] {
			return nil, fmt.Errorf("invalid scope %q: must be one of atap:inbox, atap:send, atap:approve, atap:manage", s)
		}
	}
	return parts, nil
}

// verifyPKCE checks that SHA-256(codeVerifier) encoded as base64url equals storedChallenge.
func verifyPKCE(storedChallenge, codeVerifier string) bool {
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return storedChallenge == computed
}

// bcryptHashPassword creates a bcrypt hash of the given password.
func bcryptHashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}
