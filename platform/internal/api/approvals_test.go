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
// MOCK APPROVAL STORE (for API handler tests)
// ============================================================

type mockApprovalStore struct {
	mu        sync.Mutex
	approvals map[string]*models.Approval
	created   []*models.Approval // track CreateApproval calls
}

func newMockApprovalStore() *mockApprovalStore {
	return &mockApprovalStore{
		approvals: make(map[string]*models.Approval),
	}
}

func (m *mockApprovalStore) CreateApproval(_ context.Context, apr *models.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *apr
	cp.State = models.ApprovalStateRequested
	cp.UpdatedAt = time.Now().UTC()
	m.approvals[apr.ID] = &cp
	m.created = append(m.created, &cp)
	return nil
}

func (m *mockApprovalStore) GetApprovals(_ context.Context, entityDID string) ([]models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Approval
	for _, a := range m.approvals {
		if a.To == entityDID {
			results = append(results, *a)
		}
	}
	return results, nil
}

func (m *mockApprovalStore) UpdateApprovalState(_ context.Context, id, newState, responderSignature string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apr, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	if apr.State != models.ApprovalStateRequested {
		return false, nil
	}
	apr.State = newState
	now := time.Now().UTC()
	apr.RespondedAt = &now
	apr.UpdatedAt = now
	if apr.Signatures == nil {
		apr.Signatures = make(map[string]string)
	}
	apr.Signatures["to"] = responderSignature
	return true, nil
}

func (m *mockApprovalStore) RevokeApproval(_ context.Context, id, entityDID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apr, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	if apr.To != entityDID {
		return false, nil
	}
	if apr.State != models.ApprovalStateRequested && apr.State != models.ApprovalStateApproved {
		return false, nil
	}
	apr.State = models.ApprovalStateRevoked
	apr.UpdatedAt = time.Now().UTC()
	return true, nil
}

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
	*mockApprovalStore,
) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	ms := newSyncMockMessageStore()
	ods := newMockOrgDelegateStore()
	as := newMockApprovalStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()

	h := &Handler{
		entityStore:      es,
		keyVersionStore:  kvs,
		oauthTokenStore:  ots,
		messageStore:     ms,
		orgDelegateStore: ods,
		approvalStore:    as,
		config:           cfg,
		redis:            rdb,
		platformKey:      platformPriv,
		log:              zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, ms, ods, ots, as
}

// ============================================================
// TESTS: CreateApproval fan-out
// ============================================================

// TestCreateApproval_OrgFanOut verifies that when the approval target is an org entity,
// DIDComm messages are dispatched to all org delegates.
func TestCreateApproval_OrgFanOut(t *testing.T) {
	h, app, es, ms, ods, ots, _ := newApprovalTestHandler(t)

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
	h, app, es, ms, _, ots, _ := newApprovalTestHandler(t)

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
	h, app, es, _, ods, ots, _ := newApprovalTestHandler(t)

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
	h, app, es, _, ods, ots, _ := newApprovalTestHandler(t)

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

// ============================================================
// TESTS: CreateApproval persistence
// ============================================================

// TestCreateApproval_PersistsBeforeDispatch verifies that CreateApproval calls approvalStore.CreateApproval.
func TestCreateApproval_PersistsBeforeDispatch(t *testing.T) {
	h, app, es, _, _, ots, as := newApprovalTestHandler(t)

	fromPub, _, _ := crypto.GenerateKeyPair()
	fromID := "persist-from-01"
	fromDID := "did:web:atap.app:agent:" + fromID
	es.entities[fromID] = &models.Entity{
		ID:               fromID,
		Type:             models.EntityTypeAgent,
		DID:              fromDID,
		PublicKeyEd25519: fromPub,
		KeyID:            crypto.NewKeyID("agt"),
	}

	toID := "persist-to-01"
	toDID := "did:web:atap.app:human:" + toID
	es.entities[toID] = &models.Entity{
		ID:   toID,
		Type: models.EntityTypeHuman,
		DID:  toDID,
	}

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "persist-jti-01"
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
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("expected 202, got %d; body=%v", resp.StatusCode, errBody)
	}

	// Verify the approval was persisted to the store
	as.mu.Lock()
	defer as.mu.Unlock()
	if len(as.created) == 0 {
		t.Fatal("expected approvalStore.CreateApproval to be called, but no approvals were created")
	}
	if as.created[0].From != fromDID {
		t.Errorf("persisted approval From = %q, want %q", as.created[0].From, fromDID)
	}
	if as.created[0].To != toDID {
		t.Errorf("persisted approval To = %q, want %q", as.created[0].To, toDID)
	}
}

// ============================================================
// TESTS: RespondApproval
// ============================================================

// TestRespondApproval_Success verifies that POST /v1/approvals/:id/respond returns 200.
func TestRespondApproval_Success(t *testing.T) {
	h, app, es, _, _, ots, as := newApprovalTestHandler(t)

	entityID := "respond-entity-01"
	entityDID := "did:web:atap.app:human:" + entityID
	entityPub, _, _ := crypto.GenerateKeyPair()
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		KeyID:            crypto.NewKeyID("hmn"),
	}

	// Seed an approval in the mock store
	as.mu.Lock()
	as.approvals["apr_respond01"] = &models.Approval{
		ID:    "apr_respond01",
		From:  "did:web:atap.app:agent:requester01",
		To:    entityDID,
		State: models.ApprovalStateRequested,
		Subject: models.ApprovalSubject{
			Type:  "com.example.test",
			Label: "Respond Test",
		},
		Signatures: map[string]string{},
	}
	as.mu.Unlock()

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "respond-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{"signature": "eyJhbGciOiJFZERTQSJ9.test.signature"}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals/apr_respond01/respond"
	req := httptest.NewRequest("POST", "/v1/approvals/apr_respond01/respond", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

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

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["state"] != "approved" {
		t.Errorf("state = %q, want %q", result["state"], "approved")
	}
}

// TestRespondApproval_409Conflict verifies that a second respond returns 409.
func TestRespondApproval_409Conflict(t *testing.T) {
	h, app, es, _, _, ots, as := newApprovalTestHandler(t)

	entityID := "respond-conflict-01"
	entityDID := "did:web:atap.app:human:" + entityID
	entityPub, _, _ := crypto.GenerateKeyPair()
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		KeyID:            crypto.NewKeyID("hmn"),
	}

	// Seed an already-approved approval
	as.mu.Lock()
	as.approvals["apr_conflict01"] = &models.Approval{
		ID:    "apr_conflict01",
		From:  "did:web:atap.app:agent:requester01",
		To:    entityDID,
		State: models.ApprovalStateApproved, // already responded
		Subject: models.ApprovalSubject{
			Type:  "com.example.test",
			Label: "Conflict Test",
		},
		Signatures: map[string]string{"to": "existing-sig"},
	}
	as.mu.Unlock()

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "respond-conflict-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	body := map[string]any{"signature": "eyJhbGciOiJFZERTQSJ9.test.sig2"}
	bodyBytes, _ := json.Marshal(body)

	fullURL := "https://atap.app/v1/approvals/apr_conflict01/respond"
	req := httptest.NewRequest("POST", "/v1/approvals/apr_conflict01/respond", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 409 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Errorf("expected 409 Conflict, got %d; body=%v", resp.StatusCode, errBody)
	}
}

// ============================================================
// TESTS: ListApprovals
// ============================================================

// TestListApprovals_ReturnsEntityApprovals verifies GET /v1/approvals returns the entity's approvals.
func TestListApprovals_ReturnsEntityApprovals(t *testing.T) {
	h, app, es, _, _, ots, as := newApprovalTestHandler(t)

	entityID := "list-entity-01"
	entityDID := "did:web:atap.app:human:" + entityID
	entityPub, _, _ := crypto.GenerateKeyPair()
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		KeyID:            crypto.NewKeyID("hmn"),
	}

	// Seed approvals
	as.mu.Lock()
	as.approvals["apr_list01"] = &models.Approval{
		ID:    "apr_list01",
		From:  "did:web:atap.app:agent:req01",
		To:    entityDID,
		State: models.ApprovalStateRequested,
		Subject: models.ApprovalSubject{
			Type:  "com.example.test",
			Label: "List Test 1",
		},
	}
	as.approvals["apr_list02"] = &models.Approval{
		ID:    "apr_list02",
		From:  "did:web:atap.app:agent:req02",
		To:    entityDID,
		State: models.ApprovalStateApproved,
		Subject: models.ApprovalSubject{
			Type:  "com.example.test",
			Label: "List Test 2",
		},
	}
	as.mu.Unlock()

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "list-jti-01"
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

	fullURL := "https://atap.app/v1/approvals"
	req := httptest.NewRequest("GET", "/v1/approvals", nil)
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

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

	var results []map[string]any
	json.NewDecoder(resp.Body).Decode(&results)
	if len(results) != 2 {
		t.Errorf("expected 2 approvals, got %d", len(results))
	}
}

// ============================================================
// TESTS: RevokeApproval
// ============================================================

// TestRevokeApproval_Success verifies DELETE /v1/approvals/:id returns 200 for owned approval.
func TestRevokeApproval_Success(t *testing.T) {
	h, app, es, _, _, ots, as := newApprovalTestHandler(t)

	entityID := "revoke-entity-01"
	entityDID := "did:web:atap.app:human:" + entityID
	entityPub, _, _ := crypto.GenerateKeyPair()
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		KeyID:            crypto.NewKeyID("hmn"),
	}

	as.mu.Lock()
	as.approvals["apr_revoke01"] = &models.Approval{
		ID:    "apr_revoke01",
		From:  "did:web:atap.app:agent:req01",
		To:    entityDID,
		State: models.ApprovalStateRequested,
		Subject: models.ApprovalSubject{
			Type:  "com.example.test",
			Label: "Revoke Test",
		},
	}
	as.mu.Unlock()

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "revoke-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	fullURL := "https://atap.app/v1/approvals/apr_revoke01"
	req := httptest.NewRequest("DELETE", "/v1/approvals/apr_revoke01", nil)
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

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

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["state"] != "revoked" {
		t.Errorf("state = %q, want %q", result["state"], "revoked")
	}
}

// TestRevokeApproval_404NotFound verifies DELETE returns 404 for non-existent approval.
func TestRevokeApproval_404NotFound(t *testing.T) {
	h, app, es, _, _, ots, _ := newApprovalTestHandler(t)

	entityID := "revoke-notfound-01"
	entityDID := "did:web:atap.app:human:" + entityID
	entityPub, _, _ := crypto.GenerateKeyPair()
	es.entities[entityID] = &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeHuman,
		DID:              entityDID,
		PublicKeyEd25519: entityPub,
		KeyID:            crypto.NewKeyID("hmn"),
	}

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jti := "revoke-nf-jti-01"
	jkt, _ := jwkThumbprint(dpopPub)
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	fullURL := "https://atap.app/v1/approvals/apr_nonexistent"
	req := httptest.NewRequest("DELETE", "/v1/approvals/apr_nonexistent", nil)
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
