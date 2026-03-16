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
// MOCK ORG DELEGATE STORE
// ============================================================

type mockOrgDelegateStore struct {
	mu        sync.Mutex
	delegates map[string][]models.Entity // key: orgDID
}

func newMockOrgDelegateStore() *mockOrgDelegateStore {
	return &mockOrgDelegateStore{
		delegates: make(map[string][]models.Entity),
	}
}

func (m *mockOrgDelegateStore) GetOrgDelegates(_ context.Context, orgDID string, limit int) ([]models.Entity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delegates := m.delegates[orgDID]
	if delegates == nil {
		return []models.Entity{}, nil
	}
	if limit > 0 && len(delegates) > limit {
		return delegates[:limit], nil
	}
	return delegates, nil
}

// ============================================================
// THREAD-SAFE MOCK MESSAGE STORE (for fan-out tests)
// ============================================================

// syncMockMessageStore is a thread-safe message store for fan-out tests.
type syncMockMessageStore struct {
	mu       sync.Mutex
	messages map[string]*models.DIDCommMessage
}

func newSyncMockMessageStore() *syncMockMessageStore {
	return &syncMockMessageStore{messages: make(map[string]*models.DIDCommMessage)}
}

func (m *syncMockMessageStore) QueueMessage(_ context.Context, msg *models.DIDCommMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[msg.ID] = msg
	return nil
}

func (m *syncMockMessageStore) GetPendingMessages(_ context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.DIDCommMessage
	for _, msg := range m.messages {
		if msg.RecipientDID == recipientDID && msg.State == "pending" {
			results = append(results, *msg)
		}
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (m *syncMockMessageStore) MarkDelivered(_ context.Context, messageIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, id := range messageIDs {
		if msg, ok := m.messages[id]; ok && msg.State == "pending" {
			msg.State = "delivered"
			msg.DeliveredAt = &now
		}
	}
	return nil
}

func (m *syncMockMessageStore) CleanupExpiredMessages(_ context.Context) (int64, error) {
	return 0, nil
}

// countMessagesTo counts queued messages destined for the given recipientDID.
func (m *syncMockMessageStore) countMessagesTo(recipientDID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, msg := range m.messages {
		if msg.RecipientDID == recipientDID {
			count++
		}
	}
	return count
}

// totalMessages returns the total number of queued messages.
func (m *syncMockMessageStore) totalMessages() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

// ============================================================
// TEST SETUP HELPER
// ============================================================

// newApprovalTestHandler creates a Handler with all stores wired for approval testing.
func newApprovalTestHandler(t *testing.T) (
	*Handler,
	*testFiberApp,
	*mockEntityStore,
	*syncMockMessageStore,
	*mockOrgDelegateStore,
	*mockOAuthTokenStore,
) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	ms := newSyncMockMessageStore()
	ods := newMockOrgDelegateStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()

	h := &Handler{
		entityStore:      es,
		keyVersionStore:  kvs,
		oauthTokenStore:  ots,
		messageStore:     ms,
		orgDelegateStore: ods,
		config:           cfg,
		redis:            rdb,
		platformKey:      platformPriv,
		log:              zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, ms, ods, ots
}

// ============================================================
// TESTS: CreateApproval fan-out
// ============================================================

// TestCreateApproval_OrgFanOut verifies that when the approval target is an org entity,
// DIDComm messages are dispatched to all org delegates.
func TestCreateApproval_OrgFanOut(t *testing.T) {
	h, app, es, ms, ods, ots := newApprovalTestHandler(t)

	// Create the requester (from) entity
	fromPub, _, _ := crypto.GenerateKeyPair()
	fromID := "fanout-from-01"
	fromDID := "did:web:atap.app:agent:" + fromID
	es.entities[fromID] = &models.Entity{
		ID:               fromID,
		Type:             models.EntityTypeAgent,
		DID:              fromDID,
		PublicKeyEd25519: fromPub,
		KeyID:            crypto.NewKeyID("agt"),
	}

	// Create the org (to) entity
	orgID := "fanout-org-01"
	orgDID := "did:web:atap.app:org:" + orgID
	es.entities[orgID] = &models.Entity{
		ID:   orgID,
		Type: models.EntityTypeOrg,
		DID:  orgDID,
	}

	// Register 3 delegates for the org
	delegates := make([]models.Entity, 3)
	for i := 0; i < 3; i++ {
		memberID := fmt.Sprintf("fanout-member-%02d", i+1)
		memberDID := fmt.Sprintf("did:web:atap.app:human:%s", memberID)
		delegates[i] = models.Entity{
			ID:   memberID,
			Type: models.EntityTypeHuman,
			DID:  memberDID,
		}
		es.entities[memberID] = &delegates[i]
	}
	ods.delegates[orgDID] = delegates

	// Issue a DPoP-bound token with atap:approve scope for the requester
	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "fanout-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, fromDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  fromID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{
		"from":    fromDID,
		"to":      orgDID,
		"subject": map[string]any{"type": "com.example.test", "label": "Test", "reversible": false, "payload": map[string]any{}},
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals"
	req := httptest.NewRequest("POST", "/v1/approvals", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("expected 202, got %d; body=%v", resp.StatusCode, errBody)
	}

	// Verify that DIDComm messages were queued for all 3 delegates
	// Give goroutines time to dispatch
	time.Sleep(50 * time.Millisecond)

	delegateMessages := 0
	for _, d := range delegates {
		delegateMessages += ms.countMessagesTo(d.DID)
	}
	if delegateMessages < 3 {
		t.Errorf("expected DIDComm messages for 3 delegates, got %d", delegateMessages)
	}
}

// TestCreateApproval_NonOrgNoFanOut verifies that a normal (non-org) approval target
// does NOT fan out to delegates.
func TestCreateApproval_NonOrgNoFanOut(t *testing.T) {
	h, app, es, ms, _, ots := newApprovalTestHandler(t)

	fromPub, _, _ := crypto.GenerateKeyPair()
	fromID := "nofanout-from-01"
	fromDID := "did:web:atap.app:agent:" + fromID
	es.entities[fromID] = &models.Entity{
		ID:               fromID,
		Type:             models.EntityTypeAgent,
		DID:              fromDID,
		PublicKeyEd25519: fromPub,
		KeyID:            crypto.NewKeyID("agt"),
	}

	toID := "nofanout-to-01"
	toDID := "did:web:atap.app:human:" + toID
	es.entities[toID] = &models.Entity{
		ID:   toID,
		Type: models.EntityTypeHuman,
		DID:  toDID,
	}

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "nofanout-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, fromDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  fromID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{
		"from":    fromDID,
		"to":      toDID,
		"subject": map[string]any{"type": "com.example.test", "label": "Test", "reversible": false, "payload": map[string]any{}},
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals"
	req := httptest.NewRequest("POST", "/v1/approvals", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	time.Sleep(20 * time.Millisecond)
	// Only 1 message should be queued (to toDID directly), no fan-out
	toMessages := ms.countMessagesTo(toDID)
	if toMessages == 0 {
		t.Error("expected 1 message to toDID, got 0")
	}
}

// ============================================================
// TESTS: Per-source rate limiting
// ============================================================

// TestFanOutRateLimit_ExceedsThreshold verifies that a source DID that has already
// sent 10 fan-out requests to the same org gets a 429 response.
// Requires a local Redis instance. Skips if Redis is unavailable.
func TestFanOutRateLimit_ExceedsThreshold(t *testing.T) {
	h, app, es, _, ods, ots := newApprovalTestHandler(t)

	// Check if Redis is available by verifying connection
	ctx := context.Background()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available (%v), skipping rate limit test", err)
	}

	fromPub, _, _ := crypto.GenerateKeyPair()
	fromID := "ratelimit-from-01"
	fromDID := "did:web:atap.app:agent:" + fromID
	es.entities[fromID] = &models.Entity{
		ID:               fromID,
		Type:             models.EntityTypeAgent,
		DID:              fromDID,
		PublicKeyEd25519: fromPub,
		KeyID:            crypto.NewKeyID("agt"),
	}

	orgID := "ratelimit-org-01"
	orgDID := "did:web:atap.app:org:" + orgID
	es.entities[orgID] = &models.Entity{
		ID:   orgID,
		Type: models.EntityTypeOrg,
		DID:  orgDID,
	}
	ods.delegates[orgDID] = []models.Entity{
		{ID: "ratelimit-member-01", Type: models.EntityTypeHuman, DID: "did:web:atap.app:human:ratelimit-member-01"},
	}

	// Pre-seed the Redis counter to simulate 10 fan-outs already done (threshold is 10).
	rateKey := fmt.Sprintf("fanout:rate:%s:%s", fromDID, orgDID)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	// Set counter to 10 (at threshold — next INCR will be 11, exceeding the limit)
	h.redis.Set(ctx, rateKey, 10, time.Hour)

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "ratelimit-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, fromDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  fromID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{
		"from":    fromDID,
		"to":      orgDID,
		"subject": map[string]any{"type": "com.example.test", "label": "Test", "reversible": false, "payload": map[string]any{}},
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals"
	req := httptest.NewRequest("POST", "/v1/approvals", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Errorf("expected 429 Too Many Requests when rate limit exceeded, got %d; body=%v",
			resp.StatusCode, errBody)
	}
}

// TestFanOutRateLimit_BelowThreshold verifies that requests below the rate limit succeed.
// Requires a local Redis instance. Skips if Redis is unavailable.
func TestFanOutRateLimit_BelowThreshold(t *testing.T) {
	h, app, es, _, ods, ots := newApprovalTestHandler(t)

	ctx := context.Background()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available (%v), skipping rate limit test", err)
	}

	fromPub, _, _ := crypto.GenerateKeyPair()
	fromID := "ratelimit-ok-from-01"
	fromDID := "did:web:atap.app:agent:" + fromID
	es.entities[fromID] = &models.Entity{
		ID:               fromID,
		Type:             models.EntityTypeAgent,
		DID:              fromDID,
		PublicKeyEd25519: fromPub,
		KeyID:            crypto.NewKeyID("agt"),
	}

	orgID := "ratelimit-ok-org-01"
	orgDID := "did:web:atap.app:org:" + orgID
	es.entities[orgID] = &models.Entity{
		ID:   orgID,
		Type: models.EntityTypeOrg,
		DID:  orgDID,
	}
	ods.delegates[orgDID] = []models.Entity{
		{ID: "ratelimit-ok-member-01", Type: models.EntityTypeHuman, DID: "did:web:atap.app:human:ratelimit-ok-member-01"},
	}

	// Ensure a clean counter (5 < 10 threshold)
	rateKey := fmt.Sprintf("fanout:rate:%s:%s", fromDID, orgDID)
	h.redis.Del(ctx, rateKey)
	t.Cleanup(func() { h.redis.Del(ctx, rateKey) })
	h.redis.Set(ctx, rateKey, 5, time.Hour)

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "ratelimit-ok-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, fromDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  fromID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{
		"from":    fromDID,
		"to":      orgDID,
		"subject": map[string]any{"type": "com.example.test", "label": "Test", "reversible": false, "payload": map[string]any{}},
	}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals"
	req := httptest.NewRequest("POST", "/v1/approvals", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Errorf("expected 202 for below-threshold request, got %d; body=%v", resp.StatusCode, errBody)
	}
}
