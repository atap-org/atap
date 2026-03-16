package api

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/url"
	"strings"
	"time"

	dpop "github.com/AxisCommunications/go-dpop"
	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/gofiber/fiber/v2"
)

// dpopNonceTTL is the Redis TTL for DPoP jti nonce cache entries.
// Must be larger than the DPoP proof time window (60s) to prevent replay attacks.
const dpopNonceTTL = 5 * time.Minute

// ============================================================
// DPOP AUTH MIDDLEWARE
// ============================================================

// DPoPAuthMiddleware validates the DPoP proof and Authorization: DPoP token on each request.
// Sets c.Locals("entity") and c.Locals("scopes") on success.
//
// Validation sequence (per RESEARCH.md Pattern 3):
//  1. Extract and parse DPoP proof JWT
//  2. Verify htm (method) and htu (URL) match
//  3. Check jti nonce not seen before (replay prevention via Redis)
//  4. Extract Authorization: DPoP header (reject Bearer)
//  5. Parse and verify JWT access token (signed by platform key)
//  6. Verify token cnf.jkt matches DPoP proof key thumbprint
//  7. Look up token in store (verify not revoked)
//  8. Look up entity, set in c.Locals
func (h *Handler) DPoPAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Step 1: Extract DPoP proof header
		dpopHeader := c.Get("DPoP")
		if dpopHeader == "" {
			return problem(c, 401, "invalid_dpop_proof", "Missing DPoP Proof",
				"DPoP header is required for authenticated requests")
		}

		// Step 2: Parse and validate DPoP proof (htm + htu + timing)
		requestURL := fmt.Sprintf("https://%s%s", h.config.PlatformDomain, c.Path())
		u, err := url.Parse(requestURL)
		if err != nil {
			return problem(c, 401, "invalid_dpop_proof", "Invalid DPoP Proof",
				"failed to parse request URL")
		}

		window := 60 * time.Second
		proof, err := dpop.Parse(dpopHeader, dpop.HTTPVerb(c.Method()), u, dpop.ParseOptions{
			TimeWindow: &window,
		})
		if err != nil {
			return problem(c, 401, "invalid_dpop_proof", "Invalid DPoP Proof",
				fmt.Sprintf("DPoP proof validation failed: %v", err))
		}

		// Step 3: Check jti nonce replay prevention via Redis
		claims, ok := proof.Claims.(*dpop.ProofTokenClaims)
		if !ok || claims == nil || claims.ID == "" {
			return problem(c, 401, "invalid_dpop_proof", "Invalid DPoP Proof",
				"DPoP proof missing jti claim")
		}
		jti := claims.ID
		nonceKey := "dpop:nonce:" + jti

		// Check if nonce already seen
		exists, err := h.redis.Exists(context.Background(), nonceKey).Result()
		if err == nil && exists > 0 {
			return problem(c, 401, "dpop_proof_replay", "DPoP Proof Replay Detected",
				"DPoP proof jti has already been used")
		}
		// Store nonce with TTL (best-effort: if Redis is down, skip replay check)
		if err == nil {
			h.redis.Set(context.Background(), nonceKey, "1", dpopNonceTTL)
		}

		// Step 4: Extract and validate Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return problem(c, 401, "invalid_token", "Missing Authorization",
				"Authorization header is required")
		}

		// Reject Bearer scheme — must use DPoP
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return problem(c, 401, "invalid_token_type", "Invalid Token Type",
				"Use DPoP token type, not Bearer. Authorization header must start with 'DPoP '")
		}

		if !strings.HasPrefix(authHeader, "DPoP ") {
			return problem(c, 401, "invalid_token", "Invalid Authorization",
				"Authorization header must use DPoP scheme: 'DPoP <token>'")
		}

		tokenValue := strings.TrimPrefix(authHeader, "DPoP ")
		if tokenValue == "" {
			return problem(c, 401, "invalid_token", "Missing Token",
				"no token value after DPoP scheme")
		}

		// Step 5: Parse and verify JWT access token
		platformPub := h.platformKey.Public().(ed25519.PublicKey)
		parsedToken, err := josejwt.ParseSigned(tokenValue, []jose.SignatureAlgorithm{jose.EdDSA})
		if err != nil {
			return problem(c, 401, "invalid_token", "Invalid Token",
				"token is not a valid JWT")
		}

		var tokenClaims struct {
			josejwt.Claims
			CNF struct {
				JKT string `json:"jkt"`
			} `json:"cnf"`
			Scope string `json:"scope"`
		}
		if err := parsedToken.Claims(platformPub, &tokenClaims); err != nil {
			return problem(c, 401, "invalid_token", "Invalid Token Signature",
				"token signature verification failed")
		}

		// Step 6: Verify token not expired (Claims.ValidateWithLeeway checks exp)
		if err := tokenClaims.Claims.ValidateWithLeeway(josejwt.Expected{
			Time: time.Now(),
		}, 0); err != nil {
			return problem(c, 401, "invalid_token", "Token Expired",
				"access token has expired")
		}

		// Step 7: Look up token in store (verify not revoked)
		jtiToken := tokenClaims.Claims.ID
		storedToken, err := h.oauthTokenStore.GetOAuthToken(c.Context(), jtiToken)
		if err != nil {
			h.log.Error().Err(err).Str("jti", jtiToken).Msg("failed to look up token")
			return problem(c, 500, "internal", "Internal Server Error", "failed to validate token")
		}
		if storedToken == nil {
			return problem(c, 401, "invalid_token", "Token Not Found",
				"access token not found or has been revoked")
		}

		// Step 8: Verify cnf.jkt matches DPoP proof key
		if tokenClaims.CNF.JKT == "" {
			return problem(c, 401, "invalid_token", "Token Missing cnf.jkt",
				"access token is not DPoP-bound")
		}
		proofJKT := proof.PublicKey()
		if tokenClaims.CNF.JKT != proofJKT {
			return problem(c, 401, "dpop_binding_mismatch", "DPoP Binding Mismatch",
				"DPoP proof key does not match the token's cnf.jkt binding")
		}

		// Step 9: Look up entity by DID from sub claim
		entity, err := h.entityStore.GetEntityByDID(c.Context(), tokenClaims.Claims.Subject)
		if err != nil {
			h.log.Error().Err(err).Str("sub", tokenClaims.Claims.Subject).Msg("failed to look up entity")
			return problem(c, 500, "internal", "Internal Server Error", "failed to look up entity")
		}
		if entity == nil {
			return problem(c, 401, "invalid_token", "Entity Not Found",
				"entity associated with token not found")
		}

		// Set entity and scopes in context locals
		c.Locals("entity", entity)
		scopes := strings.Fields(tokenClaims.Scope)
		c.Locals("scopes", scopes)

		return c.Next()
	}
}

// RequireScope returns middleware that checks if the authenticated token has the required scope.
// Must be used after DPoPAuthMiddleware.
func (h *Handler) RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		scopes, ok := c.Locals("scopes").([]string)
		if !ok {
			return problem(c, 403, "insufficient_scope", "Insufficient Scope",
				"no scopes found in token")
		}

		for _, s := range scopes {
			if s == scope {
				return c.Next()
			}
		}

		return problem(c, 403, "insufficient_scope", "Insufficient Scope",
			fmt.Sprintf("token does not have required scope %q", scope))
	}
}
