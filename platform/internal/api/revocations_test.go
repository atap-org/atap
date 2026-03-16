package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK REVOCATION STORE
// ============================================================

type mockRevocationStore struct {
	mu          sync.Mutex
	revocations map[string]*models.Revocation
}

func newMockRevocationStore() *mockRevocationStore {
	return &mockRevocationStore{revocations: make(map[string]*models.Revocation)}
}

func (m *mockRevocationStore) CreateRevocation(_ context.Context, r *models.Revocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check unique constraint on approval_id
	for _, existing := range m.revocations {
		if existing.ApprovalID == r.ApprovalID {
			return fmt.Errorf("revocation for approval_id %q already exists", r.ApprovalID)
		}
	}
	cp := *r
	m.revocations[r.ID] = &cp
	return nil
}

func (m *mockRevocationStore) ListRevocations(_ context.Context, approverDID string) ([]models.Revocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	var results []models.Revocation
	for _, r := range m.revocations {
		if r.ApproverDID == approverDID && r.ExpiresAt.After(now) {
			results = append(results, *r)
		}
	}
	return results, nil
}

func (m *mockRevocationStore) CleanupExpiredRevocations(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// TEST SETUP HELPERS
// ============================================================

// newRevocationTestHandler creates a Handler with revocation store wired for testing.
func newRevocationTestHandler(t *testing.T) (*Handler, *testFiberApp, *mockEntityStore, *mockRevocationStore, *mockOAuthTokenStore) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	rs := newMockRevocationStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()
	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		revocationStore: rs,
		config:          cfg,
		redis:           rdb,
		platformKey:     platformPriv,
		log:             zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, rs, ots
}

// issueRevokeToken issues a DPoP-bound JWT with atap:revoke scope for testing.
func issueRevokeToken(t *testing.T, h *Handler, ots *mockOAuthTokenStore, entityDID string, dpopPub ed25519.PublicKey) (string, string) {
	t.Helper()
	jti := uuid.NewString()
	jkt, err := jwkThumbprint(dpopPub)
	if err != nil {
		t.Fatalf("compute jkt: %v", err)
	}
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:revoke"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityDID,
		TokenType: "access",
		Scope:     []string{"atap:revoke"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	return tokenStr, jti
}

// ============================================================
// TESTS
// ============================================================

// TestSubmitRevocation_Success tests that a valid POST /v1/revocations returns 201.
func TestSubmitRevocation_Success(t *testing.T) {
	h, app, es, _, ots := newRevocationTestHandler(t)

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	pub, _, _ := crypto.GenerateKeyPair()
	entityID := "rev-test-01"
	entityDID := "did:web:atap.app:human:" + entityID
	keyID := crypto.NewKeyID("hum")
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: pub,
		KeyID:            keyID,
	}

	tokenStr, _ := issueRevokeToken(t, h, ots, entityDID, dpopPub)

	body := map[string]any{
		"approval_id": "apr_test01",
		"signature":   "test-signature",
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/revocations"
	req := httptest.NewRequest("POST", "/v1/revocations", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("expected 201, got %d; body=%v", resp.StatusCode, errBody)
	}

	var respBody map[string]any
	json.NewDecoder(resp.Body).Decode(&respBody)

	if respBody["id"] == nil {
		t.Error("response missing id field")
	}
	if respBody["approval_id"] != "apr_test01" {
		t.Errorf("approval_id = %v, want apr_test01", respBody["approval_id"])
	}
	if respBody["approver_did"] != entityDID {
		t.Errorf("approver_did = %v, want %q", respBody["approver_did"], entityDID)
	}
	if respBody["revoked_at"] == nil {
		t.Error("response missing revoked_at")
	}
	if respBody["expires_at"] == nil {
		t.Error("response missing expires_at")
	}
}

// TestSubmitRevocation_WrongScope tests that a token without atap:revoke scope returns 403.
func TestSubmitRevocation_WrongScope(t *testing.T) {
	h, app, es, _, ots := newRevocationTestHandler(t)

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	pub, _, _ := crypto.GenerateKeyPair()
	entityID := "rev-scope-test"
	entityDID := "did:web:atap.app:human:" + entityID
	keyID := crypto.NewKeyID("hum")
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: pub,
		KeyID:            keyID,
	}

	// Issue token with atap:inbox scope (wrong scope for revocations)
	jti := uuid.NewString()
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:inbox"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:inbox"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{
		"approval_id": "apr_scope_test",
		"signature":   "test-signature",
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/revocations"
	req := httptest.NewRequest("POST", "/v1/revocations", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		t.Errorf("expected 403 for wrong scope, got %d", resp.StatusCode)
	}
}

// TestListRevocations_Success tests that GET /v1/revocations?entity=... returns revocations.
func TestListRevocations_Success(t *testing.T) {
	_, app, _, rs, _ := newRevocationTestHandler(t)

	approverDID := "did:web:atap.app:human:list-test-01"
	now := time.Now().UTC()
	rev := &models.Revocation{
		ID:          "rev_list01",
		ApprovalID:  "apr_list01",
		ApproverDID: approverDID,
		RevokedAt:   now,
		ExpiresAt:   now.Add(60 * time.Minute),
	}
	if err := rs.CreateRevocation(nil, rev); err != nil {
		t.Fatalf("setup CreateRevocation: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/revocations?entity="+approverDID, nil)
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

	var respBody map[string]any
	json.NewDecoder(resp.Body).Decode(&respBody)

	if respBody["entity"] != approverDID {
		t.Errorf("entity = %v, want %q", respBody["entity"], approverDID)
	}
	revocations, ok := respBody["revocations"].([]any)
	if !ok {
		t.Fatalf("revocations field is not an array: %T %v", respBody["revocations"], respBody["revocations"])
	}
	if len(revocations) != 1 {
		t.Errorf("expected 1 revocation, got %d", len(revocations))
	}
	if respBody["checked_at"] == nil {
		t.Error("response missing checked_at")
	}
}

// TestListRevocations_MissingEntity tests that GET /v1/revocations without entity param returns 400.
func TestListRevocations_MissingEntity(t *testing.T) {
	_, app, _, _, _ := newRevocationTestHandler(t)

	req := httptest.NewRequest("GET", "/v1/revocations", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for missing entity param, got %d", resp.StatusCode)
	}
}
