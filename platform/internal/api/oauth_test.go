package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK OAUTH TOKEN STORE
// ============================================================

type mockOAuthTokenStore struct {
	tokens map[string]*models.OAuthToken
	codes  map[string]*models.OAuthAuthCode
}

func newMockOAuthTokenStore() *mockOAuthTokenStore {
	return &mockOAuthTokenStore{
		tokens: make(map[string]*models.OAuthToken),
		codes:  make(map[string]*models.OAuthAuthCode),
	}
}

func (m *mockOAuthTokenStore) CreateOAuthToken(_ context.Context, token *models.OAuthToken) error {
	m.tokens[token.ID] = token
	return nil
}

func (m *mockOAuthTokenStore) GetOAuthToken(_ context.Context, tokenID string) (*models.OAuthToken, error) {
	t, ok := m.tokens[tokenID]
	if !ok {
		return nil, nil
	}
	if t.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	if t.RevokedAt != nil {
		return nil, nil
	}
	return t, nil
}

func (m *mockOAuthTokenStore) RevokeOAuthToken(_ context.Context, tokenID string) error {
	if t, ok := m.tokens[tokenID]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (m *mockOAuthTokenStore) CreateAuthCode(_ context.Context, code *models.OAuthAuthCode) error {
	m.codes[code.Code] = code
	return nil
}

func (m *mockOAuthTokenStore) RedeemAuthCode(_ context.Context, code string) (*models.OAuthAuthCode, error) {
	c, ok := m.codes[code]
	if !ok {
		return nil, nil
	}
	if c.UsedAt != nil {
		return nil, nil
	}
	if c.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	now := time.Now()
	c.UsedAt = &now
	return c, nil
}

func (m *mockOAuthTokenStore) CleanupExpiredTokens(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// TEST HELPERS
// ============================================================

// newTestHandlerFull creates a Handler with all fields (including oauth store and redis).
func newTestHandlerFull(es EntityStore, kvs KeyVersionStore, ots OAuthTokenStore, cfg *config.Config) (*Handler, *testFiberApp) {
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()
	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		config:          cfg,
		redis:           rdb,
		platformKey:     platformPriv,
	}
	app := newTestFiberAppFromHandler(h)
	return h, app
}

// generateDPoPProof creates a DPoP proof JWT signed by the given Ed25519 private key.
func generateDPoPProof(t *testing.T, privKey ed25519.PrivateKey, pubKey ed25519.PublicKey, method, rawURL string) string {
	t.Helper()

	// Build JWK for the public key (OKP Ed25519)
	jwk := jose.JSONWebKey{Key: pubKey, Algorithm: string(jose.EdDSA)}
	jwkBytes, err := jwk.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal JWK: %v", err)
	}
	var jwkMap map[string]interface{}
	if err := json.Unmarshal(jwkBytes, &jwkMap); err != nil {
		t.Fatalf("unmarshal JWK map: %v", err)
	}

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: privKey},
		(&jose.SignerOptions{}).WithHeader("typ", "dpop+jwt").WithHeader("jwk", jwkMap),
	)
	if err != nil {
		t.Fatalf("create DPoP signer: %v", err)
	}

	claims := map[string]interface{}{
		"jti": uuid.NewString(),
		"htm": method,
		"htu": rawURL,
		"iat": time.Now().Unix(),
	}

	token, err := josejwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign DPoP proof: %v", err)
	}
	return token
}

// pkceS256Challenge computes the PKCE S256 challenge from a verifier string.
func pkceS256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// computeTestJWKThumbprint computes the JWK SHA-256 thumbprint for a test Ed25519 public key.
func computeTestJWKThumbprint(t *testing.T, pubKey ed25519.PublicKey) string {
	t.Helper()
	thumb, err := jwkThumbprint(pubKey)
	if err != nil {
		t.Fatalf("compute JWK thumbprint: %v", err)
	}
	return thumb
}

// issueTestToken issues a JWT access token using the handler's platform key (for test setup).
func issueTestToken(t *testing.T, h *Handler, entityDID, jti, jkt string, scopes []string, ttl time.Duration) string {
	t.Helper()
	tokenStr, err := issueJWT(h.platformKey, h.config.PlatformDomain, entityDID, jti, jkt, scopes, ttl)
	if err != nil {
		t.Fatalf("issue test JWT: %v", err)
	}
	return tokenStr
}

// ============================================================
// CLIENT CREDENTIALS GRANT TESTS
// ============================================================

func TestClientCredentials(t *testing.T) {
	t.Run("valid client credentials returns DPoP access token", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "cc-valid-001"
		secret := "atap_valid_secret_12345"
		secretHash, _ := bcryptHashPassword(secret)
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: entityPub,
			ClientSecretHash: secretHash,
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:agent:"+entityID)
		form.Set("client_secret", secret)

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var body map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&body)
			t.Fatalf("status = %d, want 200; body = %v", resp.StatusCode, body)
		}

		var tokenResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if tokenResp["access_token"] == nil {
			t.Error("missing access_token")
		}
		if tokenResp["token_type"] != "DPoP" {
			t.Errorf("token_type = %q, want DPoP", tokenResp["token_type"])
		}
		if ei, ok := tokenResp["expires_in"].(float64); !ok || int(ei) != 3600 {
			t.Errorf("expires_in = %v, want 3600", tokenResp["expires_in"])
		}
		if tokenResp["scope"] == nil {
			t.Error("missing scope")
		}
		if tokenResp["refresh_token"] == nil {
			t.Error("missing refresh_token")
		}
	})

	t.Run("invalid client_secret returns 401", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "cc-bad-secret-001"
		correctSecret := "atap_correct_secret"
		secretHash, _ := bcryptHashPassword(correctSecret)
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: entityPub,
			ClientSecretHash: secretHash,
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:agent:"+entityID)
		form.Set("client_secret", "atap_wrong_secret")

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (wrong secret)", resp.StatusCode)
		}
	})

	t.Run("nonexistent client_id returns 401", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:agent:nonexistent")
		form.Set("client_secret", "atap_any_secret")

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (nonexistent client)", resp.StatusCode)
		}
	})

	t.Run("human entity rejected with 400 for client_credentials", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "cc-human-001"
		secret := "atap_human_secret"
		secretHash, _ := bcryptHashPassword(secret)
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeHuman,
			DID:              "did:web:atap.app:human:" + entityID,
			PublicKeyEd25519: entityPub,
			ClientSecretHash: secretHash,
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:human:"+entityID)
		form.Set("client_secret", secret)

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (human cannot use client_credentials)", resp.StatusCode)
		}
	})

	t.Run("missing DPoP header returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:agent:any")
		form.Set("client_secret", "atap_secret")

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// No DPoP header

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (missing DPoP)", resp.StatusCode)
		}
	})

	t.Run("invalid scope rejected with 400", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "cc-scope-001"
		secret := "atap_scope_secret"
		secretHash, _ := bcryptHashPassword(secret)
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: entityPub,
			ClientSecretHash: secretHash,
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "did:web:atap.app:agent:"+entityID)
		form.Set("client_secret", secret)
		form.Set("scope", "invalid:scope")

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (invalid scope)", resp.StatusCode)
		}
	})
}

func TestClientCredentials_TokenContainsClaims(t *testing.T) {
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	h, app := newTestHandlerFull(es, kvs, ots, cfg)

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	entityPub, _, _ := crypto.GenerateKeyPair()
	entityID := "claims-agent-001"
	secret := "atap_claims_secret"
	secretHash, _ := bcryptHashPassword(secret)
	entityDID := "did:web:atap.app:agent:" + entityID
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		ClientSecretHash: secretHash,
	}

	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", entityDID)
	form.Set("client_secret", secret)
	form.Set("scope", "atap:inbox atap:send")

	req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DPoP", dpopProof)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("status = %d, want 200; body = %v", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Parse and verify JWT claims
	parsedToken, err := josejwt.ParseSigned(tokenResp.AccessToken, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		t.Fatalf("parse access token JWT: %v", err)
	}

	platformPub := h.platformKey.Public().(ed25519.PublicKey)
	var claims map[string]interface{}
	if err := parsedToken.Claims(platformPub, &claims); err != nil {
		t.Fatalf("verify JWT signature: %v", err)
	}

	if claims["sub"] != entityDID {
		t.Errorf("sub = %q, want %q", claims["sub"], entityDID)
	}
	if claims["iss"] != cfg.PlatformDomain {
		t.Errorf("iss = %q, want %q", claims["iss"], cfg.PlatformDomain)
	}
	if claims["jti"] == nil {
		t.Error("jti claim missing")
	}
	cnf, ok := claims["cnf"].(map[string]interface{})
	if !ok {
		t.Fatal("cnf claim missing or wrong type")
	}
	if cnf["jkt"] == nil {
		t.Error("cnf.jkt missing")
	}

	expectedJKT := computeTestJWKThumbprint(t, dpopPub)
	if cnf["jkt"] != expectedJKT {
		t.Errorf("cnf.jkt = %q, want %q", cnf["jkt"], expectedJKT)
	}

	scope, _ := claims["scope"].(string)
	if !strings.Contains(scope, "atap:inbox") || !strings.Contains(scope, "atap:send") {
		t.Errorf("scope = %q, should contain atap:inbox and atap:send", scope)
	}

	// Verify token stored in store
	jti := claims["jti"].(string)
	storedToken, _ := ots.GetOAuthToken(context.Background(), jti)
	if storedToken == nil {
		t.Error("access token not stored in oauth store")
	}
	if storedToken != nil && storedToken.DPoPJKT != expectedJKT {
		t.Errorf("stored DPoPJKT = %q, want %q", storedToken.DPoPJKT, expectedJKT)
	}
}

// ============================================================
// AUTHORIZATION CODE GRANT TESTS
// ============================================================

func TestAuthCode_Authorize(t *testing.T) {
	t.Run("valid authorize redirects with code", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "auth-human-001"
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeHuman,
			DID:              "did:web:atap.app:human:" + entityID,
			PublicKeyEd25519: entityPub,
		}

		params := url.Values{}
		params.Set("response_type", "code")
		params.Set("client_id", "did:web:atap.app:human:"+entityID)
		params.Set("redirect_uri", "atap://callback")
		params.Set("scope", "atap:approve")
		params.Set("code_challenge", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM")
		params.Set("code_challenge_method", "S256")
		params.Set("state", "xyz123")

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/oauth/authorize")
		req := httptest.NewRequest("GET", "/v1/oauth/authorize?"+params.Encode(), nil)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 302 {
			var body map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&body)
			t.Fatalf("status = %d, want 302; body = %v", resp.StatusCode, body)
		}

		loc := resp.Header.Get("Location")
		if !strings.HasPrefix(loc, "atap://callback") {
			t.Errorf("Location = %q, want atap://callback prefix", loc)
		}
		if !strings.Contains(loc, "code=") {
			t.Error("Location should contain code= param")
		}
		if !strings.Contains(loc, "state=xyz123") {
			t.Error("Location should contain state=xyz123")
		}
	})

	t.Run("plain code_challenge_method returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "auth-human-002"
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeHuman,
			DID:              "did:web:atap.app:human:" + entityID,
			PublicKeyEd25519: entityPub,
		}

		params := url.Values{}
		params.Set("response_type", "code")
		params.Set("client_id", "did:web:atap.app:human:"+entityID)
		params.Set("redirect_uri", "atap://callback")
		params.Set("code_challenge", "verifier")
		params.Set("code_challenge_method", "plain")

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/oauth/authorize")
		req := httptest.NewRequest("GET", "/v1/oauth/authorize?"+params.Encode(), nil)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (plain rejected)", resp.StatusCode)
		}
	})

	t.Run("missing code_challenge returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerFull(es, kvs, ots, cfg)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "auth-human-003"
		es.entities[entityID] = &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeHuman,
			DID:              "did:web:atap.app:human:" + entityID,
			PublicKeyEd25519: entityPub,
		}

		params := url.Values{}
		params.Set("response_type", "code")
		params.Set("client_id", "did:web:atap.app:human:"+entityID)
		params.Set("redirect_uri", "atap://callback")
		params.Set("code_challenge_method", "S256")
		// no code_challenge

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/oauth/authorize")
		req := httptest.NewRequest("GET", "/v1/oauth/authorize?"+params.Encode(), nil)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (missing code_challenge)", resp.StatusCode)
		}
	})
}

func TestAuthCode_TokenExchange(t *testing.T) {
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app := newTestHandlerFull(es, kvs, ots, cfg)

	entityPub, _, _ := crypto.GenerateKeyPair()
	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	entityID := "exchange-human-001"
	entityDID := "did:web:atap.app:human:" + entityID
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
	}

	jkt := computeTestJWKThumbprint(t, dpopPub)
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	codeChallenge := pkceS256Challenge(codeVerifier)

	// Pre-load auth code
	authCode := &models.OAuthAuthCode{
		Code:          "exchange-code-001",
		EntityID:      entityID,
		RedirectURI:   "atap://callback",
		Scope:         []string{"atap:approve", "atap:manage"},
		CodeChallenge: codeChallenge,
		DPoPJKT:       jkt,
		ExpiresAt:     time.Now().UTC().Add(10 * time.Minute),
		CreatedAt:     time.Now().UTC(),
	}
	ots.codes["exchange-code-001"] = authCode

	t.Run("valid auth code exchange returns token", func(t *testing.T) {
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", "exchange-code-001")
		form.Set("redirect_uri", "atap://callback")
		form.Set("code_verifier", codeVerifier)

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var body map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&body)
			t.Fatalf("status = %d, want 200; body = %v", resp.StatusCode, body)
		}
		var tokenResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenResp)
		if tokenResp["access_token"] == nil {
			t.Error("missing access_token")
		}
		if tokenResp["token_type"] != "DPoP" {
			t.Errorf("token_type = %q, want DPoP", tokenResp["token_type"])
		}
		if tokenResp["refresh_token"] == nil {
			t.Error("missing refresh_token")
		}
	})

	t.Run("invalid code_verifier returns 400", func(t *testing.T) {
		// Create fresh code for this sub-test
		freshCode := &models.OAuthAuthCode{
			Code:          "exchange-code-002",
			EntityID:      entityID,
			RedirectURI:   "atap://callback",
			Scope:         []string{"atap:approve"},
			CodeChallenge: codeChallenge,
			DPoPJKT:       jkt,
			ExpiresAt:     time.Now().UTC().Add(10 * time.Minute),
			CreatedAt:     time.Now().UTC(),
		}
		ots.codes["exchange-code-002"] = freshCode

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", "exchange-code-002")
		form.Set("redirect_uri", "atap://callback")
		form.Set("code_verifier", "wrong_verifier")

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (wrong verifier)", resp.StatusCode)
		}
	})

	t.Run("second redemption returns 400 (single-use)", func(t *testing.T) {
		// Code from first subtest is already used
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/oauth/token")
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", "exchange-code-001") // already used
		form.Set("redirect_uri", "atap://callback")
		form.Set("code_verifier", codeVerifier)

		req := httptest.NewRequest("POST", "/v1/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 (code already used)", resp.StatusCode)
		}
	})
}

// ============================================================
// DPOP MIDDLEWARE TESTS
// ============================================================

func TestDPoP_Middleware(t *testing.T) {
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	h, app := newTestHandlerFull(es, kvs, ots, cfg)

	entityPub, _, _ := crypto.GenerateKeyPair()
	entityID := "dpop-agent-001"
	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		DID:              "did:web:atap.app:agent:" + entityID,
		PublicKeyEd25519: entityPub,
	}
	es.entities[entityID] = entity

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jkt := computeTestJWKThumbprint(t, dpopPub)

	jti := uuid.NewString()
	tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:manage"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	t.Run("valid DPoP auth passes on protected endpoint", func(t *testing.T) {
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		// Should not be 401/403 — DPoP check passes
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			t.Errorf("status = %d, DPoP should have passed auth", resp.StatusCode)
		}
	})

	t.Run("Authorization Bearer rejected with 401", func(t *testing.T) {
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr) // wrong scheme
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (Bearer rejected)", resp.StatusCode)
		}
	})

	t.Run("missing DPoP header returns 401", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		// No DPoP header

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (missing DPoP)", resp.StatusCode)
		}
	})

	t.Run("method mismatch in DPoP proof returns 401", func(t *testing.T) {
		wrongMethodProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", wrongMethodProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (method mismatch)", resp.StatusCode)
		}
	})

	t.Run("URL mismatch in DPoP proof returns 401", func(t *testing.T) {
		wrongURLProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/wrongid")
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", wrongURLProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (URL mismatch)", resp.StatusCode)
		}
	})

	t.Run("JWK thumbprint mismatch returns 401", func(t *testing.T) {
		// Different DPoP key pair
		otherPub, otherPriv, _ := crypto.GenerateKeyPair()
		differentKeyProof := generateDPoPProof(t, otherPriv, otherPub, "DELETE", "https://atap.app/v1/entities/"+entityID)

		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", differentKeyProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401 (JKT mismatch)", resp.StatusCode)
		}
	})
}

func TestDPoP_RevokedToken(t *testing.T) {
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	h, app := newTestHandlerFull(es, kvs, ots, cfg)

	entityPub, _, _ := crypto.GenerateKeyPair()
	entityID := "dpop-revoked-001"
	entity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		DID:              "did:web:atap.app:agent:" + entityID,
		PublicKeyEd25519: entityPub,
	}
	es.entities[entityID] = entity

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jkt := computeTestJWKThumbprint(t, dpopPub)
	jti := uuid.NewString()
	tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:manage"}, time.Hour)

	revokedAt := time.Now()
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:manage"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		RevokedAt: &revokedAt,
	}

	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
	req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
	req.Header.Set("Authorization", "DPoP "+tokenStr)
	req.Header.Set("DPoP", dpopProof)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401 (revoked token)", resp.StatusCode)
	}
}

// ============================================================
// SCOPE ENFORCEMENT TESTS
// ============================================================

func TestScope_Enforcement(t *testing.T) {
	// Use separate handlers/stores for each sub-test to avoid state leaking between tests
	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()

	t.Run("token with atap:manage allows DELETE", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "scope-manage-001"
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: entityPub,
		}
		es.entities[entityID] = entity
		jkt := computeTestJWKThumbprint(t, dpopPub)

		jti := uuid.NewString()
		tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID:        jti,
			EntityID:  entityID,
			TokenType: "access",
			Scope:     []string{"atap:manage"},
			DPoPJKT:   jkt,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 403 {
			t.Error("got 403 but atap:manage scope is present")
		}
	})

	t.Run("token without atap:manage returns 403 on DELETE", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "scope-inbox-001"
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: entityPub,
		}
		es.entities[entityID] = entity
		jkt := computeTestJWKThumbprint(t, dpopPub)

		jti := uuid.NewString()
		// Token only has atap:inbox, not atap:manage
		tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:inbox"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID:        jti,
			EntityID:  entityID,
			TokenType: "access",
			Scope:     []string{"atap:inbox"},
			DPoPJKT:   jkt,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("status = %d, want 403 (missing atap:manage)", resp.StatusCode)
		}
	})
}

// Suppress unused import warnings
var _ = bytes.NewBuffer
var _ = http.StatusOK
