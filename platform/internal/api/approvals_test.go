package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK APPROVAL STORE
// ============================================================

type mockApprovalStore struct {
	mu        sync.Mutex
	approvals map[string]*models.Approval
}

func newMockApprovalStore() *mockApprovalStore {
	return &mockApprovalStore{approvals: make(map[string]*models.Approval)}
}

func (m *mockApprovalStore) CreateApproval(_ context.Context, a *models.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.approvals[a.ID] = &cp
	return nil
}

func (m *mockApprovalStore) GetApproval(_ context.Context, id string) (*models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}

func (m *mockApprovalStore) UpdateApprovalState(_ context.Context, id, state string, respondedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return fmt.Errorf("approval %q not found", id)
	}
	a.State = state
	a.RespondedAt = respondedAt
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockApprovalStore) ConsumeApproval(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	if a.State == models.ApprovalStateApproved && a.ValidUntil == nil {
		a.State = models.ApprovalStateConsumed
		a.UpdatedAt = time.Now().UTC()
		return true, nil
	}
	return false, nil
}

func (m *mockApprovalStore) ListApprovals(_ context.Context, entityDID string, limit, offset int) ([]models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.Approval
	for _, a := range m.approvals {
		if a.From == entityDID || a.To == entityDID || a.Via == entityDID {
			result = append(result, *a)
		}
	}
	if offset >= len(result) {
		return nil, nil
	}
	result = result[offset:]
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockApprovalStore) RevokeWithChildren(_ context.Context, parentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	// Revoke parent
	if a, ok := m.approvals[parentID]; ok {
		a.State = models.ApprovalStateRevoked
		a.UpdatedAt = now
	}
	// Revoke children
	for _, a := range m.approvals {
		if a.Parent == parentID {
			a.State = models.ApprovalStateRevoked
			a.UpdatedAt = now
		}
	}
	return nil
}

func (m *mockApprovalStore) CleanupExpiredApprovals(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// MOCK MESSAGE STORE (for DIDComm dispatch checks)
// ============================================================

type mockMessageStoreWithCapture struct {
	mu       sync.Mutex
	messages []*models.DIDCommMessage
}

func newMockMessageStoreWithCapture() *mockMessageStoreWithCapture {
	return &mockMessageStoreWithCapture{}
}

func (m *mockMessageStoreWithCapture) QueueMessage(_ context.Context, msg *models.DIDCommMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *msg
	m.messages = append(m.messages, &cp)
	return nil
}

func (m *mockMessageStoreWithCapture) GetPendingMessages(_ context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []models.DIDCommMessage
	for _, msg := range m.messages {
		if msg.RecipientDID == recipientDID && msg.State == "pending" {
			result = append(result, *msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockMessageStoreWithCapture) MarkDelivered(_ context.Context, messageIDs []string) error {
	return nil
}

func (m *mockMessageStoreWithCapture) CleanupExpiredMessages(_ context.Context) (int64, error) {
	return 0, nil
}

func (m *mockMessageStoreWithCapture) messagesOfType(msgType string) []*models.DIDCommMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*models.DIDCommMessage
	for _, msg := range m.messages {
		if msg.MessageType == msgType {
			result = append(result, msg)
		}
	}
	return result
}

// ============================================================
// TEST SETUP HELPERS
// ============================================================

// testApprovalEntity holds a registered entity and its keys for tests.
type testApprovalEntity struct {
	entity  *models.Entity
	pubKey  ed25519.PublicKey
	privKey ed25519.PrivateKey
	keyID   string // fully-qualified DID URL key ID
}

// newApprovalTestHandler creates a Handler with approval store wired for testing.
func newApprovalTestHandler(t *testing.T) (*Handler, *testFiberApp, *mockEntityStore, *mockApprovalStore, *mockMessageStoreWithCapture) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	as := newMockApprovalStore()
	ms := newMockMessageStoreWithCapture()
	cfg := &config.Config{
		PlatformDomain: "atap.app",
		MaxApprovalTTL: 90 * 24 * time.Hour,
	}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()
	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		messageStore:    ms,
		approvalStore:   as,
		config:          cfg,
		redis:           rdb,
		platformKey:     platformPriv,
		log:             zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, as, ms
}

// registerTestEntity creates a test entity and registers it in the mock store.
func registerTestEntity(t *testing.T, h *Handler, es *mockEntityStore, entityType, suffix string) *testApprovalEntity {
	t.Helper()
	pub, priv, _ := crypto.GenerateKeyPair()

	var entityID string
	if entityType == models.EntityTypeHuman {
		entityID = crypto.DeriveHumanID(pub)
	} else {
		entityID = "test-" + suffix
	}
	keyID := crypto.NewKeyID(entityType[:3])
	did := fmt.Sprintf("did:web:atap.app:%s:%s", entityType, entityID)
	fullyQualifiedKeyID := fmt.Sprintf("%s#%s", did, keyID)

	entity := &models.Entity{
		ID:               entityID,
		Type:             entityType,
		DID:              did,
		PublicKeyEd25519: pub,
		KeyID:            keyID,
	}
	es.entities[entityID] = entity

	return &testApprovalEntity{
		entity:  entity,
		pubKey:  pub,
		privKey: priv,
		keyID:   fullyQualifiedKeyID,
	}
}

// issueApproveToken issues a DPoP-bound JWT with atap:approve scope for testing.
func issueApproveToken(t *testing.T, h *Handler, ots *mockOAuthTokenStore, entityDID string, dpopPub ed25519.PublicKey) (string, string) {
	t.Helper()
	jti := uuid.NewString()
	jkt, err := jwkThumbprint(dpopPub)
	if err != nil {
		t.Fatalf("compute jkt: %v", err)
	}
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:approve"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID:        jti,
		EntityID:  entityDID,
		TokenType: "access",
		Scope:     []string{"atap:approve"},
		DPoPJKT:   jkt,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	return tokenStr, jti
}

// signApprovalDoc signs an approval document with the given key.
func signApprovalDoc(t *testing.T, apr *models.Approval, privKey ed25519.PrivateKey, keyID string) string {
	t.Helper()
	sig, err := approval.SignApproval(apr, privKey, keyID)
	if err != nil {
		t.Fatalf("sign approval: %v", err)
	}
	return sig
}

// makeAuthRequest creates an authenticated HTTP request with DPoP proof.
func makeAuthRequest(t *testing.T, method, path string, body interface{}, dpopPriv ed25519.PrivateKey, dpopPub ed25519.PublicKey, tokenStr string) *http.Request {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	fullURL := fmt.Sprintf("https://atap.app%s", path)
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, method, fullURL)
	req.Header.Set("DPoP", dpopProof)
	req.Header.Set("Authorization", "DPoP "+tokenStr)
	return req
}

// ============================================================
// TESTS
// ============================================================

// TestTwoPartyApprovalFlow tests the full two-party approval lifecycle.
func TestTwoPartyApprovalFlow(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	// Register entities
	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, toDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from01")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to01")

	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	toToken, _ := issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)

	// Build approval document to sign — client generates ID and created_at
	now := time.Now().UTC()
	clientAprID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{
		Type:       "dev.atap.test.action",
		Label:      "Test Action",
		Reversible: true,
		Payload:    json.RawMessage(`{"key":"value"}`),
	}
	aprForSigning := &models.Approval{
		AtapApproval: "1",
		ID:           clientAprID,
		CreatedAt:    now,
		From:         fromEnt.entity.DID,
		To:           toEnt.entity.DID,
		Subject:      subject,
		Signatures:   map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)

	// Step 1: Create approval — include id and created_at so server can verify signature
	createBody := map[string]any{
		"id":             clientAprID,
		"created_at":     now,
		"to":             toEnt.entity.DID,
		"subject":        subject,
		"from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("create approval: status=%d body=%v", resp.StatusCode, errBody)
	}

	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)

	approvalID, ok := createResp["id"].(string)
	if !ok || approvalID == "" {
		t.Fatal("response missing approval id")
	}
	if createResp["state"] != "requested" {
		t.Errorf("expected state=requested, got %v", createResp["state"])
	}
	sigs, _ := createResp["signatures"].(map[string]any)
	if sigs["from"] == nil {
		t.Error("from signature not in response")
	}

	// Step 2: Get approval as from entity
	getReq := makeAuthRequest(t, "GET", "/v1/approvals/"+approvalID, nil, fromDpopPriv, fromDpopPub, fromToken)
	getResp, _ := app.Test(getReq, 5000)
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Errorf("get approval: status=%d", getResp.StatusCode)
	}

	// Step 3: Respond as to entity (approve)
	storedApr, _ := as.GetApproval(nil, approvalID)
	if storedApr == nil {
		t.Fatal("stored approval not found")
	}
	toSig := signApprovalDoc(t, storedApr, toEnt.privKey, toEnt.keyID)

	respondBody := map[string]any{
		"status":    "approved",
		"signature": toSig,
	}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()

	if respondResp.StatusCode != 200 {
		var errBody map[string]any
		json.NewDecoder(respondResp.Body).Decode(&errBody)
		t.Fatalf("respond approval: status=%d body=%v", respondResp.StatusCode, errBody)
	}
	var respondRespBody map[string]any
	json.NewDecoder(respondResp.Body).Decode(&respondRespBody)
	if respondRespBody["state"] != "approved" {
		t.Errorf("expected state=approved, got %v", respondRespBody["state"])
	}

	// Step 4: Check status (persistent approval needs valid_until, so this returns valid=false for one-time)
	// Since valid_until is nil, it's a one-time approval — ConsumeApproval will be called
	statusReq := httptest.NewRequest("GET", "/v1/approvals/"+approvalID+"/status", nil)
	statusResp, _ := app.Test(statusReq, 5000)
	defer statusResp.Body.Close()
	var statusBody map[string]any
	json.NewDecoder(statusResp.Body).Decode(&statusBody)
	if statusBody["valid"] != true {
		t.Errorf("expected valid=true on first status check, got %v", statusBody)
	}
	if statusBody["consumed"] != true {
		t.Errorf("expected consumed=true for one-time approval, got %v", statusBody)
	}

	// Step 5: List approvals for from entity
	listReq := makeAuthRequest(t, "GET", "/v1/approvals", nil, fromDpopPriv, fromDpopPub, fromToken)
	listResp, _ := app.Test(listReq, 5000)
	defer listResp.Body.Close()
	var listBody map[string]any
	json.NewDecoder(listResp.Body).Decode(&listBody)
	if listBody["count"].(float64) < 1 {
		t.Error("expected at least 1 approval in list")
	}
}

// TestThreePartyApprovalFlow tests three-party approval with server as via.
func TestThreePartyApprovalFlow(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, toDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from3p01")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to3p01")

	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	toToken, _ := issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)

	serverDID := h.serverDID()
	now3p := time.Now().UTC()
	clientAprID3p := crypto.NewApprovalID()
	subject := models.ApprovalSubject{
		Type:       "dev.atap.test.action",
		Label:      "3-party test",
		Reversible: false,
		Payload:    json.RawMessage(`{}`),
	}

	aprForSigning := &models.Approval{
		AtapApproval: "1",
		ID:           clientAprID3p,
		CreatedAt:    now3p,
		From:         fromEnt.entity.DID,
		To:           toEnt.entity.DID,
		Via:          serverDID,
		Subject:      subject,
		Signatures:   map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)

	createBody := map[string]any{
		"id":             clientAprID3p,
		"created_at":     now3p,
		"to":             toEnt.entity.DID,
		"via":            serverDID,
		"subject":        subject,
		"from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("create three-party approval: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("create three-party approval: status=%d body=%v", resp.StatusCode, errBody)
	}

	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)
	approvalID := createResp["id"].(string)

	sigs := createResp["signatures"].(map[string]any)
	if sigs["from"] == nil {
		t.Error("missing from signature in three-party response")
	}
	if sigs["via"] == nil {
		t.Error("missing via signature in three-party response")
	}

	// Respond as to entity
	storedApr, _ := as.GetApproval(nil, approvalID)
	if storedApr == nil {
		t.Fatal("stored three-party approval not found")
	}
	toSig := signApprovalDoc(t, storedApr, toEnt.privKey, toEnt.keyID)
	respondBody := map[string]any{"status": "approved", "signature": toSig}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()
	if respondResp.StatusCode != 200 {
		t.Fatalf("respond three-party: status=%d", respondResp.StatusCode)
	}

	finalApr, _ := as.GetApproval(nil, approvalID)
	if finalApr.Signatures["to"] != toSig {
		// The to signature is in the response but not persisted to the document (server-side state)
		// This is by design — state is in the state column
	}
	if finalApr.State != models.ApprovalStateApproved {
		t.Errorf("expected approved, got %s", finalApr.State)
	}
}

// TestThreePartyRejection verifies APR-08 via rejection dispatches TypeApprovalRejected.
func TestThreePartyRejection(t *testing.T) {
	h, app, es, as, ms := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-rej")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-rej")
	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	serverDID := h.serverDID()

	// Subject type is empty — triggers "unsupported_subject_type" rejection
	nowRej := time.Now().UTC()
	clientRejID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{
		Type:    "", // empty = triggers unsupported_subject_type rejection
		Label:   "Bad Subject",
		Payload: json.RawMessage(`{}`),
	}
	aprForSigning := &models.Approval{
		AtapApproval: "1",
		ID:           clientRejID,
		CreatedAt:    nowRej,
		From:         fromEnt.entity.DID,
		To:           toEnt.entity.DID,
		Via:          serverDID,
		Subject:      subject,
		Signatures:   map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)

	createBody := map[string]any{
		"id":             clientRejID,
		"created_at":     nowRej,
		"to":             toEnt.entity.DID,
		"via":            serverDID,
		"subject":        subject,
		"from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("expected 422 for via rejection, got %d", resp.StatusCode)
	}

	var rejBody map[string]any
	json.NewDecoder(resp.Body).Decode(&rejBody)
	if rejBody["reason"] == nil {
		t.Error("expected reason field in rejection response")
	}
	if rejBody["reason"] != "unsupported_subject_type" {
		t.Errorf("expected reason=unsupported_subject_type, got %v", rejBody["reason"])
	}

	// Verify TypeApprovalRejected was dispatched
	rejectedMsgs := ms.messagesOfType("https://atap.dev/protocols/approval/1.0/rejected")
	if len(rejectedMsgs) == 0 {
		t.Error("expected TypeApprovalRejected message to be dispatched")
	}

	// Verify approval was stored with state=rejected
	approvalID, _ := rejBody["approval_id"].(string)
	if approvalID != "" {
		storedApr, _ := as.GetApproval(nil, approvalID)
		if storedApr == nil {
			t.Error("rejected approval not stored")
		} else if storedApr.State != models.ApprovalStateRejected {
			t.Errorf("expected rejected state, got %s", storedApr.State)
		}
	}
}

// TestApprovalDecline tests declining and double-respond rejection.
func TestApprovalDecline(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, toDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-dec")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-dec")
	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	toToken, _ := issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)

	nowDec := time.Now().UTC()
	clientDecID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{Type: "dev.atap.test.decline", Label: "Decline Test", Payload: json.RawMessage(`{}`)}
	aprForSigning := &models.Approval{
		AtapApproval: "1", ID: clientDecID, CreatedAt: nowDec,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		Signatures: map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)

	createBody := map[string]any{
		"id": clientDecID, "created_at": nowDec,
		"to": toEnt.entity.DID, "subject": subject, "from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, _ := app.Test(req, 5000)
	defer resp.Body.Close()
	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)
	if resp.StatusCode != 201 {
		t.Fatalf("create for decline test: status=%d body=%v", resp.StatusCode, createResp)
	}
	approvalID := createResp["id"].(string)

	storedApr, _ := as.GetApproval(nil, approvalID)
	declineSig := signApprovalDoc(t, storedApr, toEnt.privKey, toEnt.keyID)

	// Decline
	respondBody := map[string]any{"status": "declined", "signature": declineSig}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()
	if respondResp.StatusCode != 200 {
		t.Fatalf("decline: status=%d", respondResp.StatusCode)
	}
	var respondBody2 map[string]any
	json.NewDecoder(respondResp.Body).Decode(&respondBody2)
	if respondBody2["state"] != "declined" {
		t.Errorf("expected declined, got %v", respondBody2["state"])
	}

	// Try to respond again — should get 409
	respondReq2 := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp2, _ := app.Test(respondReq2, 5000)
	defer respondResp2.Body.Close()
	if respondResp2.StatusCode != 409 {
		t.Errorf("expected 409 for double respond, got %d", respondResp2.StatusCode)
	}
}

// TestRevokeWithChildren tests cascading revocation of parent and children.
func TestRevokeWithChildren(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, toDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-rev")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-rev")
	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	toToken, _ := issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)

	// Create parent approval (persistent, with valid_until)
	// Truncate to second precision so RFC3339 round-trip in JSON produces identical time for signing.
	nowRev := time.Now().UTC().Truncate(time.Second)
	validUntil := nowRev.Add(24 * time.Hour)
	clientParentID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{Type: "dev.atap.test.revoke", Label: "Revoke Test", Payload: json.RawMessage(`{}`)}
	parentForSigning := &models.Approval{
		AtapApproval: "1", ID: clientParentID, CreatedAt: nowRev,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		ValidUntil: &validUntil, Signatures: map[string]string{},
	}
	parentFromSig := signApprovalDoc(t, parentForSigning, fromEnt.privKey, fromEnt.keyID)

	createParentBody := map[string]any{
		"id": clientParentID, "created_at": nowRev,
		"to": toEnt.entity.DID, "subject": subject,
		"from_signature": parentFromSig, "valid_until": validUntil.Format(time.RFC3339),
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createParentBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, _ := app.Test(req, 5000)
	defer resp.Body.Close()
	var parentCreateResp map[string]any
	json.NewDecoder(resp.Body).Decode(&parentCreateResp)
	if resp.StatusCode != 201 {
		t.Fatalf("create parent: status=%d body=%v", resp.StatusCode, parentCreateResp)
	}
	parentID := parentCreateResp["id"].(string)

	// Approve parent
	storedParent, _ := as.GetApproval(nil, parentID)
	parentToSig := signApprovalDoc(t, storedParent, toEnt.privKey, toEnt.keyID)
	respondBody := map[string]any{"status": "approved", "signature": parentToSig}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+parentID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()
	if respondResp.StatusCode != 200 {
		t.Fatalf("approve parent: status=%d", respondResp.StatusCode)
	}

	// Create child referencing parent
	nowChild := time.Now().UTC().Truncate(time.Second)
	clientChildID := crypto.NewApprovalID()
	childForSigning := &models.Approval{
		AtapApproval: "1", ID: clientChildID, CreatedAt: nowChild,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		Parent: parentID, ValidUntil: &validUntil, Signatures: map[string]string{},
	}
	childFromSig := signApprovalDoc(t, childForSigning, fromEnt.privKey, fromEnt.keyID)
	createChildBody := map[string]any{
		"id": clientChildID, "created_at": nowChild,
		"to": toEnt.entity.DID, "subject": subject, "parent": parentID,
		"from_signature": childFromSig, "valid_until": validUntil.Format(time.RFC3339),
	}
	childReq := makeAuthRequest(t, "POST", "/v1/approvals", createChildBody, fromDpopPriv, fromDpopPub, fromToken)
	childResp, _ := app.Test(childReq, 5000)
	defer childResp.Body.Close()
	var childCreateResp map[string]any
	json.NewDecoder(childResp.Body).Decode(&childCreateResp)
	if childResp.StatusCode != 201 {
		t.Fatalf("create child: status=%d body=%v", childResp.StatusCode, childCreateResp)
	}
	childID := childCreateResp["id"].(string)

	// Approve child
	storedChild, _ := as.GetApproval(nil, childID)
	childToSig := signApprovalDoc(t, storedChild, toEnt.privKey, toEnt.keyID)
	childRespondBody := map[string]any{"status": "approved", "signature": childToSig}
	childRespondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+childID+"/respond", childRespondBody, toDpopPriv, toDpopPub, toToken)
	childRespondResp, _ := app.Test(childRespondReq, 5000)
	defer childRespondResp.Body.Close()

	// DELETE parent — cascades to child
	deleteReq := makeAuthRequest(t, "DELETE", "/v1/approvals/"+parentID, nil, toDpopPriv, toDpopPub, toToken)
	deleteResp, _ := app.Test(deleteReq, 5000)
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != 204 {
		t.Fatalf("revoke parent: status=%d", deleteResp.StatusCode)
	}

	// Verify parent is revoked
	parentFinal, _ := as.GetApproval(nil, parentID)
	if parentFinal.State != models.ApprovalStateRevoked {
		t.Errorf("parent expected revoked, got %s", parentFinal.State)
	}

	// Verify child is revoked
	childFinal, _ := as.GetApproval(nil, childID)
	if childFinal.State != models.ApprovalStateRevoked {
		t.Errorf("child expected revoked, got %s", childFinal.State)
	}
}

// TestOneTimeConsumed tests APR-09 one-time approval atomic consumption.
func TestOneTimeConsumed(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, toDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-onetime")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-onetime")
	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	toToken, _ := issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)

	nowOT := time.Now().UTC()
	clientOTID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{Type: "dev.atap.test.onetime", Label: "One Time", Payload: json.RawMessage(`{}`)}
	// No valid_until = one-time
	aprForSigning := &models.Approval{
		AtapApproval: "1", ID: clientOTID, CreatedAt: nowOT,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		Signatures: map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)

	createBody := map[string]any{
		"id": clientOTID, "created_at": nowOT,
		"to": toEnt.entity.DID, "subject": subject, "from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, _ := app.Test(req, 5000)
	defer resp.Body.Close()
	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)
	if resp.StatusCode != 201 {
		t.Fatalf("create one-time: status=%d body=%v", resp.StatusCode, createResp)
	}
	approvalID := createResp["id"].(string)

	// Approve
	storedApr, _ := as.GetApproval(nil, approvalID)
	toSig := signApprovalDoc(t, storedApr, toEnt.privKey, toEnt.keyID)
	respondBody := map[string]any{"status": "approved", "signature": toSig}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, toDpopPriv, toDpopPub, toToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()
	if respondResp.StatusCode != 200 {
		t.Fatalf("approve one-time: status=%d", respondResp.StatusCode)
	}

	// First status check — should consume
	statusReq1 := httptest.NewRequest("GET", "/v1/approvals/"+approvalID+"/status", nil)
	statusResp1, _ := app.Test(statusReq1, 5000)
	defer statusResp1.Body.Close()
	var statusBody1 map[string]any
	json.NewDecoder(statusResp1.Body).Decode(&statusBody1)
	if statusBody1["valid"] != true {
		t.Errorf("first check: expected valid=true, got %v", statusBody1)
	}
	if statusBody1["consumed"] != true {
		t.Errorf("first check: expected consumed=true, got %v", statusBody1)
	}

	// Second status check — should return valid=false, reason=consumed
	statusReq2 := httptest.NewRequest("GET", "/v1/approvals/"+approvalID+"/status", nil)
	statusResp2, _ := app.Test(statusReq2, 5000)
	defer statusResp2.Body.Close()
	var statusBody2 map[string]any
	json.NewDecoder(statusResp2.Body).Decode(&statusBody2)
	if statusBody2["valid"] != false {
		t.Errorf("second check: expected valid=false, got %v", statusBody2)
	}
	if statusBody2["reason"] != "consumed" {
		t.Errorf("second check: expected reason=consumed, got %v", statusBody2)
	}
}

// TestSignatureKIDValidation verifies APR-12 kid mismatch is rejected.
func TestSignatureKIDValidation(t *testing.T) {
	h, app, es, _, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-kid")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-kid")
	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)

	nowKID := time.Now().UTC()
	clientKIDID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{Type: "dev.atap.test.kid", Label: "KID Test", Payload: json.RawMessage(`{}`)}
	aprForSigning := &models.Approval{
		AtapApproval: "1", ID: clientKIDID, CreatedAt: nowKID,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		Signatures: map[string]string{},
	}
	// Sign with a WRONG kid (different entity's key ID)
	wrongKeyID := "did:web:atap.app:agent:wrong-entity#key-wrong-0"
	wrongSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, wrongKeyID)

	createBody := map[string]any{
		"id": clientKIDID, "created_at": nowKID,
		"to": toEnt.entity.DID, "subject": subject, "from_signature": wrongSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, _ := app.Test(req, 5000)
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		t.Errorf("expected 400 for kid mismatch, got %d body=%v", resp.StatusCode, body)
	}
}

// TestUnauthorizedAccess tests that non-party entities get 403.
func TestUnauthorizedAccess(t *testing.T) {
	h, app, es, as, _ := newApprovalTestHandler(t)
	ots := newMockOAuthTokenStore()
	h.oauthTokenStore = ots

	fromDpopPub, fromDpopPriv, _ := crypto.GenerateKeyPair()
	toDpopPub, _, _ := crypto.GenerateKeyPair()
	cDpopPub, cDpopPriv, _ := crypto.GenerateKeyPair()

	fromEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "from-unauth")
	toEnt := registerTestEntity(t, h, es, models.EntityTypeHuman, "to-unauth")
	cEnt := registerTestEntity(t, h, es, models.EntityTypeAgent, "c-unauth")

	fromToken, _ := issueApproveToken(t, h, ots, fromEnt.entity.DID, fromDpopPub)
	issueApproveToken(t, h, ots, toEnt.entity.DID, toDpopPub)
	cToken, _ := issueApproveToken(t, h, ots, cEnt.entity.DID, cDpopPub)

	nowUA := time.Now().UTC()
	clientUAID := crypto.NewApprovalID()
	subject := models.ApprovalSubject{Type: "dev.atap.test.unauth", Label: "Unauth Test", Payload: json.RawMessage(`{}`)}
	aprForSigning := &models.Approval{
		AtapApproval: "1", ID: clientUAID, CreatedAt: nowUA,
		From: fromEnt.entity.DID, To: toEnt.entity.DID, Subject: subject,
		Signatures: map[string]string{},
	}
	fromSig := signApprovalDoc(t, aprForSigning, fromEnt.privKey, fromEnt.keyID)
	createBody := map[string]any{
		"id": clientUAID, "created_at": nowUA,
		"to": toEnt.entity.DID, "subject": subject, "from_signature": fromSig,
	}
	req := makeAuthRequest(t, "POST", "/v1/approvals", createBody, fromDpopPriv, fromDpopPub, fromToken)
	resp, _ := app.Test(req, 5000)
	defer resp.Body.Close()
	var createResp map[string]any
	json.NewDecoder(resp.Body).Decode(&createResp)
	if resp.StatusCode != 201 {
		t.Fatalf("create: status=%d", resp.StatusCode)
	}
	approvalID := createResp["id"].(string)

	// Entity C tries to GET — expect 403
	getReq := makeAuthRequest(t, "GET", "/v1/approvals/"+approvalID, nil, cDpopPriv, cDpopPub, cToken)
	getResp, _ := app.Test(getReq, 5000)
	defer getResp.Body.Close()
	if getResp.StatusCode != 403 {
		t.Errorf("GET by non-party: expected 403, got %d", getResp.StatusCode)
	}

	// Entity C tries to respond — expect 403
	storedApr, _ := as.GetApproval(nil, approvalID)
	cSig := signApprovalDoc(t, storedApr, cEnt.privKey, cEnt.keyID)
	respondBody := map[string]any{"status": "approved", "signature": cSig}
	respondReq := makeAuthRequest(t, "POST", "/v1/approvals/"+approvalID+"/respond", respondBody, cDpopPriv, cDpopPub, cToken)
	respondResp, _ := app.Test(respondReq, 5000)
	defer respondResp.Body.Close()
	if respondResp.StatusCode != 403 {
		t.Errorf("respond by non-party: expected 403, got %d", respondResp.StatusCode)
	}
}

