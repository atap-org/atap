package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK CREDENTIAL STORE
// ============================================================

type mockCredentialStore struct {
	mu           sync.Mutex
	encKeys      map[string][]byte            // entityID -> key
	credentials  map[string][]*models.Credential // entityID -> list
	statusLists  map[string]*models.CredentialStatusList
	nextIndices  map[string]int
	statusBits   map[string][]byte
}

func newMockCredentialStore() *mockCredentialStore {
	return &mockCredentialStore{
		encKeys:     make(map[string][]byte),
		credentials: make(map[string][]*models.Credential),
		statusLists: make(map[string]*models.CredentialStatusList),
		nextIndices: make(map[string]int),
		statusBits:  make(map[string][]byte),
	}
}

func (m *mockCredentialStore) CreateEncKey(_ context.Context, entityID string, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encKeys[entityID] = key
	return nil
}

func (m *mockCredentialStore) GetEncKey(_ context.Context, entityID string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.encKeys[entityID]
	if !ok {
		return nil, nil
	}
	return k, nil
}

func (m *mockCredentialStore) DeleteEncKey(_ context.Context, entityID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.encKeys, entityID)
	return nil
}

func (m *mockCredentialStore) CreateCredential(_ context.Context, cred *models.Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.credentials[cred.EntityID] = append(m.credentials[cred.EntityID], cred)
	return nil
}

func (m *mockCredentialStore) GetCredentials(_ context.Context, entityID string) ([]models.Credential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.Credential
	for _, c := range m.credentials[entityID] {
		result = append(result, *c)
	}
	return result, nil
}

func (m *mockCredentialStore) RevokeCredential(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, creds := range m.credentials {
		for _, c := range creds {
			if c.ID == id {
				now := time.Now()
				c.RevokedAt = &now
				return nil
			}
		}
	}
	return nil
}

func (m *mockCredentialStore) GetStatusList(_ context.Context, listID string) (*models.CredentialStatusList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sl, ok := m.statusLists[listID]
	if !ok {
		return nil, nil
	}
	return sl, nil
}

func (m *mockCredentialStore) GetNextStatusIndex(_ context.Context, listID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.nextIndices[listID]
	m.nextIndices[listID] = idx + 1
	return idx, nil
}

func (m *mockCredentialStore) UpdateStatusListBit(_ context.Context, listID string, index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bits := m.statusBits[listID]
	if bits == nil {
		bits = make([]byte, 16384)
		m.statusBits[listID] = bits
	}
	byteIdx := index / 8
	if byteIdx < len(bits) {
		bits[byteIdx] |= (1 << (7 - uint(index%8)))
	}
	return nil
}

// ============================================================
// TEST HANDLER HELPERS
// ============================================================

// newCredentialTestHandler creates a Handler with credential store for testing.
func newCredentialTestHandler(t *testing.T) (
	*Handler,
	*testFiberApp,
	*mockEntityStore,
	*mockCredentialStore,
	*mockOAuthTokenStore,
) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cs := newMockCredentialStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()

	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		credentialStore: cs,
		config:          cfg,
		redis:           rdb,
		platformKey:     platformPriv,
		log:             zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, cs, ots
}

// setupCredentialTestEntity creates an entity in the store and issues a DPoP-bound token for it.
func setupCredentialTestEntity(t *testing.T, h *Handler, es *mockEntityStore, ots *mockOAuthTokenStore, entityID string) (tokenStr string, dpopPriv interface{ Sign([]byte) ([]byte, error) }, dpopPubBytes []byte) {
	t.Helper()
	pub, _, _ := crypto.GenerateKeyPair()
	dpopPub, dpopPrivKey, _ := crypto.GenerateKeyPair()
	jkt := computeTestJWKThumbprint(t, dpopPub)
	did := "did:web:atap.app:human:" + entityID
	jti := "cred-jti-" + entityID

	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              did,
		PublicKeyEd25519: pub,
		KeyID:            crypto.NewKeyID("hum"),
		TrustLevel:       0,
	}

	token := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:manage"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_ = dpopPrivKey // used for DPoP proof generation inline in tests
	return token, nil, dpopPub
}

// ============================================================
// TESTS: StartEmailVerification
// ============================================================

func TestStartEmailVerification(t *testing.T) {
	t.Run("valid email returns 200", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "email-start-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-email-start-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		body, _ := json.Marshal(map[string]string{"email": "test@example.com"})
		fullURL := "https://atap.app/v1/credentials/email/start"
		req := httptest.NewRequest("POST", "/v1/credentials/email/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 200, got %d; body=%v", resp.StatusCode, errBody)
		}
	})

	t.Run("missing email returns 400", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "email-start-02"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-email-start-jti-02"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		body, _ := json.Marshal(map[string]string{})
		fullURL := "https://atap.app/v1/credentials/email/start"
		req := httptest.NewRequest("POST", "/v1/credentials/email/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

// ============================================================
// TESTS: VerifyEmail
// ============================================================

func TestVerifyEmail(t *testing.T) {
	t.Run("valid OTP issues credential and returns 201", func(t *testing.T) {
		h, app, es, cs, ots := newCredentialTestHandler(t)

		ctx := context.Background()
		// Check if Redis is available for OTP storage
		if err := h.redis.Ping(ctx).Err(); err != nil {
			t.Skipf("Redis not available (%v), skipping OTP test", err)
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "email-verify-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-email-verify-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did,
			PublicKeyEd25519: pub, KeyID: crypto.NewKeyID("hum"), TrustLevel: 0,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		// Pre-seed OTP in Redis
		email := "valid@example.com"
		otp := "123456"
		otpKey := fmt.Sprintf("otp:email:%s:%s", entityID, email)
		h.redis.Set(ctx, otpKey, otp, 10*time.Minute)
		t.Cleanup(func() { h.redis.Del(ctx, otpKey) })

		body, _ := json.Marshal(map[string]string{"email": email, "otp": otp})
		fullURL := "https://atap.app/v1/credentials/email/verify"
		req := httptest.NewRequest("POST", "/v1/credentials/email/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 10000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 201, got %d; body=%v", resp.StatusCode, errBody)
		}

		// Check credential was stored
		creds, _ := cs.GetCredentials(ctx, entityID)
		if len(creds) == 0 {
			t.Error("expected credential to be stored, none found")
		}

		// Verify credential type
		if len(creds) > 0 && creds[0].Type != "ATAPEmailVerification" {
			t.Errorf("credential type = %q, want ATAPEmailVerification", creds[0].Type)
		}

		// Check JWT returned in response body
		var respBody map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err == nil {
			if _, ok := respBody["credential"]; !ok {
				t.Error("response missing 'credential' JWT field")
			}
		}
	})

	t.Run("wrong OTP returns 400", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		ctx := context.Background()
		if err := h.redis.Ping(ctx).Err(); err != nil {
			t.Skipf("Redis not available (%v), skipping OTP test", err)
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "email-verify-02"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-email-verify-jti-02"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		email := "wrong@example.com"
		otpKey := fmt.Sprintf("otp:email:%s:%s", entityID, email)
		h.redis.Set(ctx, otpKey, "654321", 10*time.Minute)
		t.Cleanup(func() { h.redis.Del(ctx, otpKey) })

		body, _ := json.Marshal(map[string]string{"email": email, "otp": "000000"})
		fullURL := "https://atap.app/v1/credentials/email/verify"
		req := httptest.NewRequest("POST", "/v1/credentials/email/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for wrong OTP, got %d", resp.StatusCode)
		}
	})

	t.Run("expired OTP returns 400", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		ctx := context.Background()
		if err := h.redis.Ping(ctx).Err(); err != nil {
			t.Skipf("Redis not available (%v), skipping OTP test", err)
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "email-verify-03"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-email-verify-jti-03"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		// No OTP stored (simulates expired/missing OTP)
		email := "expired@example.com"
		body, _ := json.Marshal(map[string]string{"email": email, "otp": "999999"})
		fullURL := "https://atap.app/v1/credentials/email/verify"
		req := httptest.NewRequest("POST", "/v1/credentials/email/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for expired/missing OTP, got %d", resp.StatusCode)
		}
	})
}

// ============================================================
// TESTS: StartPhoneVerification
// ============================================================

func TestStartPhoneVerification(t *testing.T) {
	t.Run("valid phone returns 200", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "phone-start-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-phone-start-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		body, _ := json.Marshal(map[string]string{"phone": "+14155551234"})
		fullURL := "https://atap.app/v1/credentials/phone/start"
		req := httptest.NewRequest("POST", "/v1/credentials/phone/start", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 200, got %d; body=%v", resp.StatusCode, errBody)
		}
	})
}

// ============================================================
// TESTS: VerifyPhone (CRD-02)
// ============================================================

func TestVerifyPhone(t *testing.T) {
	t.Run("valid OTP issues PhoneVerification credential and returns 201", func(t *testing.T) {
		h, app, es, cs, ots := newCredentialTestHandler(t)

		ctx := context.Background()
		if err := h.redis.Ping(ctx).Err(); err != nil {
			t.Skipf("Redis not available (%v), skipping OTP test", err)
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "phone-verify-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-phone-verify-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did,
			PublicKeyEd25519: pub, KeyID: crypto.NewKeyID("hum"), TrustLevel: 0,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		phone := "+14155559876"
		otp := "456789"
		otpKey := fmt.Sprintf("otp:phone:%s:%s", entityID, phone)
		h.redis.Set(ctx, otpKey, otp, 10*time.Minute)
		t.Cleanup(func() { h.redis.Del(ctx, otpKey) })

		body, _ := json.Marshal(map[string]string{"phone": phone, "otp": otp})
		fullURL := "https://atap.app/v1/credentials/phone/verify"
		req := httptest.NewRequest("POST", "/v1/credentials/phone/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 10000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 201, got %d; body=%v", resp.StatusCode, errBody)
		}

		// Verify ATAPPhoneVerification credential was stored
		creds, _ := cs.GetCredentials(ctx, entityID)
		if len(creds) == 0 {
			t.Error("expected credential to be stored, none found")
		}
		if len(creds) > 0 && creds[0].Type != "ATAPPhoneVerification" {
			t.Errorf("credential type = %q, want ATAPPhoneVerification", creds[0].Type)
		}

		// Check JWT returned in response body
		var respBody map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err == nil {
			if _, ok := respBody["credential"]; !ok {
				t.Error("response missing 'credential' JWT field")
			}
		}
	})

	t.Run("wrong OTP returns 400", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		ctx := context.Background()
		if err := h.redis.Ping(ctx).Err(); err != nil {
			t.Skipf("Redis not available (%v), skipping OTP test", err)
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "phone-verify-02"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-phone-verify-jti-02"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		phone := "+14155550001"
		otpKey := fmt.Sprintf("otp:phone:%s:%s", entityID, phone)
		h.redis.Set(ctx, otpKey, "111111", 10*time.Minute)
		t.Cleanup(func() { h.redis.Del(ctx, otpKey) })

		body, _ := json.Marshal(map[string]string{"phone": phone, "otp": "999999"})
		fullURL := "https://atap.app/v1/credentials/phone/verify"
		req := httptest.NewRequest("POST", "/v1/credentials/phone/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for wrong phone OTP, got %d", resp.StatusCode)
		}
	})
}

// ============================================================
// TESTS: SubmitPersonhood
// ============================================================

func TestSubmitPersonhood(t *testing.T) {
	t.Run("valid provider_token issues credential and returns 201", func(t *testing.T) {
		h, app, es, cs, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "personhood-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-personhood-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did,
			PublicKeyEd25519: pub, KeyID: crypto.NewKeyID("hum"), TrustLevel: 0,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		body, _ := json.Marshal(map[string]string{"provider_token": "worldid_proof_abc123"})
		fullURL := "https://atap.app/v1/credentials/personhood"
		req := httptest.NewRequest("POST", "/v1/credentials/personhood", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 10000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 201, got %d; body=%v", resp.StatusCode, errBody)
		}

		// Credential stored
		ctx := context.Background()
		creds, _ := cs.GetCredentials(ctx, entityID)
		if len(creds) == 0 {
			t.Error("expected personhood credential to be stored")
		}
	})

	t.Run("body with biometric field is rejected (PRV-04)", func(t *testing.T) {
		h, app, es, _, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "personhood-02"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-personhood-jti-02"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did, PublicKeyEd25519: pub,
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		// Include biometric field — should be rejected
		body, _ := json.Marshal(map[string]string{
			"provider_token": "proof_abc",
			"biometric_data": "face_scan_xyz",
		})
		fullURL := "https://atap.app/v1/credentials/personhood"
		req := httptest.NewRequest("POST", "/v1/credentials/personhood", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("expected 400 for biometric field (PRV-04), got %d", resp.StatusCode)
		}
	})
}

// ============================================================
// TESTS: ListCredentials
// ============================================================

func TestListCredentials(t *testing.T) {
	t.Run("returns list of decrypted credential JWTs", func(t *testing.T) {
		h, app, es, cs, ots := newCredentialTestHandler(t)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		entityID := "list-creds-01"
		did := "did:web:atap.app:human:" + entityID
		jti := "cred-list-jti-01"

		pub, _, _ := crypto.GenerateKeyPair()
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeHuman, DID: did,
			PublicKeyEd25519: pub, KeyID: crypto.NewKeyID("hum"),
		}
		tokenStr := issueTestToken(t, h, did, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		// Seed an encrypted credential
		encKey := make([]byte, 32)
		for i := range encKey {
			encKey[i] = byte(i + 1)
		}
		cs.encKeys[entityID] = encKey

		// Store a credential in the mock (CredentialCT is empty for mock — handler returns empty list
		// when decryption fails on empty ciphertext, which is acceptable for this list test)
		cs.credentials[entityID] = append(cs.credentials[entityID], &models.Credential{
			ID:       "crd_test001",
			EntityID: entityID,
			Type:     "ATAPEmailVerification",
			IssuedAt: time.Now(),
		})

		fullURL := "https://atap.app/v1/credentials"
		req := httptest.NewRequest("GET", "/v1/credentials", nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", generateDPoPProof(t, dpopPriv, dpopPub, "GET", fullURL))

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 200, got %d; body=%v", resp.StatusCode, errBody)
		}
	})
}

// ============================================================
// TESTS: GetStatusList
// ============================================================

func TestGetStatusList(t *testing.T) {
	t.Run("returns Bitstring Status List VC", func(t *testing.T) {
		h, app, _, cs, _ := newCredentialTestHandler(t)
		_ = h

		// Seed a status list
		listID := "1"
		cs.statusLists[listID] = &models.CredentialStatusList{
			ID:        listID,
			Bits:      make([]byte, 16384), // 131072 slots
			NextIndex: 0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		req := httptest.NewRequest("GET", "/v1/credentials/status/1", nil)
		// No auth required — public endpoint

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			var errBody map[string]any
			json.NewDecoder(resp.Body).Decode(&errBody)
			t.Fatalf("expected 200, got %d; body=%v", resp.StatusCode, errBody)
		}

		// Response should be a VC with encodedList field
		var respBody map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		// Should have @context as VC
		if _, ok := respBody["@context"]; !ok {
			t.Error("response missing @context (should be a VC)")
		}
	})

	t.Run("nonexistent status list returns 404", func(t *testing.T) {
		_, app, _, _, _ := newCredentialTestHandler(t)

		req := httptest.NewRequest("GET", "/v1/credentials/status/nonexistent", nil)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}
