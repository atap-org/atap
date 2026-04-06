package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK CLAIM STORE
// ============================================================

type mockClaimStore struct {
	claims      map[string]*models.Claim // keyed by code
	principalOf map[string]string        // entityID -> principalDID
}

func newMockClaimStore() *mockClaimStore {
	return &mockClaimStore{
		claims:      make(map[string]*models.Claim),
		principalOf: make(map[string]string),
	}
}

func (m *mockClaimStore) CreateClaim(_ context.Context, cl *models.Claim) error {
	m.claims[cl.Code] = cl
	return nil
}

func (m *mockClaimStore) GetClaimByCode(_ context.Context, code string) (*models.Claim, error) {
	cl, ok := m.claims[code]
	if !ok {
		return nil, nil
	}
	return cl, nil
}

func (m *mockClaimStore) RedeemClaim(_ context.Context, code, humanEntityID string) (bool, error) {
	cl, ok := m.claims[code]
	if !ok {
		return false, nil
	}
	if cl.Status != models.ClaimStatusPending || time.Now().After(cl.ExpiresAt) {
		return false, nil
	}
	cl.Status = models.ClaimStatusRedeemed
	cl.RedeemedBy = humanEntityID
	now := time.Now()
	cl.RedeemedAt = &now
	return true, nil
}

func (m *mockClaimStore) SetEntityPrincipalDID(_ context.Context, entityID, principalDID string) error {
	m.principalOf[entityID] = principalDID
	return nil
}

func (m *mockClaimStore) CleanupExpiredClaims(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// MOCK MESSAGE STORE (for claim notification)
// ============================================================

type mockClaimMessageStore struct {
	messages []*models.DIDCommMessage
}

func (m *mockClaimMessageStore) QueueMessage(_ context.Context, msg *models.DIDCommMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockClaimMessageStore) GetPendingMessages(_ context.Context, _ string, _ int) ([]models.DIDCommMessage, error) {
	return nil, nil
}

func (m *mockClaimMessageStore) MarkDelivered(_ context.Context, _ []string) error {
	return nil
}

func (m *mockClaimMessageStore) CleanupExpiredMessages(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// HELPERS
// ============================================================

func newClaimTestHandler() (*Handler, *mockClaimStore, *mockEntityStore, *mockClaimMessageStore) {
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	cls := newMockClaimStore()
	ms := &mockClaimMessageStore{}
	_, platformPriv, _ := crypto.GenerateKeyPair()

	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: newMockOAuthTokenStore(),
		claimStore:      cls,
		messageStore:    ms,
		config:          &config.Config{PlatformDomain: "test.atap.dev"},
		redis:           newTestRedisClient(),
		platformKey:     platformPriv,
		log:             zerolog.Nop(),
	}
	return h, cls, es, ms
}

func newClaimTestApp(h *Handler) *fiber.App {
	app := fiber.New()
	h.SetupRoutes(app)
	return app
}

// seedAgent adds a pre-registered agent to the mock entity store and
// sets c.Locals("entity") by adding a test middleware.
func seedAgent(es *mockEntityStore) *models.Entity {
	agentID := crypto.NewEntityID()
	agent := &models.Entity{
		ID:        agentID,
		Type:      models.EntityTypeAgent,
		DID:       crypto.BuildDID("test.atap.dev", models.EntityTypeAgent, agentID),
		Name:      "TestBot",
		CreatedAt: time.Now().UTC(),
	}
	es.entities[agentID] = agent
	return agent
}

// ============================================================
// TESTS: POST /v1/claims (agent creates claim)
// ============================================================

func TestCreateClaim(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()

	agent := seedAgent(es)

	// Build a Fiber app with auth bypass (inject entity into locals)
	app := fiber.New()
	app.Post("/v1/claims", func(c *fiber.Ctx) error {
		c.Locals("entity", agent)
		c.Locals("scopes", []string{"atap:send"})
		return c.Next()
	}, h.CreateClaim)

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "My Travel Bot",
		"description": "Books flights for you",
		"scopes":      []string{"atap:inbox", "atap:send"},
	})

	req := httptest.NewRequest("POST", "/v1/claims", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result models.CreateClaimResponse
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code == "" {
		t.Error("expected non-empty claim code")
	}
	if result.URL == "" {
		t.Error("expected non-empty claim URL")
	}
	if result.ID == "" {
		t.Error("expected non-empty claim ID")
	}

	// Verify claim was stored
	stored := cls.claims[result.Code]
	if stored == nil {
		t.Fatal("claim not found in store")
	}
	if stored.AgentID != agent.ID {
		t.Errorf("expected agent_id %s, got %s", agent.ID, stored.AgentID)
	}
	if stored.AgentName != "My Travel Bot" {
		t.Errorf("expected agent_name 'My Travel Bot', got %q", stored.AgentName)
	}
	if stored.Status != models.ClaimStatusPending {
		t.Errorf("expected status pending, got %s", stored.Status)
	}
}

func TestCreateClaimOnlyAgents(t *testing.T) {
	h, _, _, _ := newClaimTestHandler()

	human := &models.Entity{
		ID:   "human123",
		Type: models.EntityTypeHuman,
		DID:  "did:web:test.atap.dev:human:human123",
	}

	app := fiber.New()
	app.Post("/v1/claims", func(c *fiber.Ctx) error {
		c.Locals("entity", human)
		c.Locals("scopes", []string{"atap:send"})
		return c.Next()
	}, h.CreateClaim)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "Test",
	})
	req := httptest.NewRequest("POST", "/v1/claims", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for non-agent, got %d", resp.StatusCode)
	}
}

func TestCreateClaimInvalidScope(t *testing.T) {
	h, _, es, _ := newClaimTestHandler()

	agent := seedAgent(es)

	app := fiber.New()
	app.Post("/v1/claims", func(c *fiber.Ctx) error {
		c.Locals("entity", agent)
		c.Locals("scopes", []string{"atap:send"})
		return c.Next()
	}, h.CreateClaim)

	body, _ := json.Marshal(map[string]interface{}{
		"name":   "Test",
		"scopes": []string{"atap:admin"},
	})
	req := httptest.NewRequest("POST", "/v1/claims", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid scope, got %d", resp.StatusCode)
	}
}

// ============================================================
// TESTS: GET /claim/:code (landing page)
// ============================================================

func TestClaimPage(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	// Seed a claim
	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-TEST",
		AgentID:   agent.ID,
		AgentName: "TestBot",
		Scopes:    []string{"atap:inbox"},
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)
	req := httptest.NewRequest("GET", "/claim/ATAP-TEST", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected HTML content type, got %q", ct)
	}
}

func TestClaimPageNotFound(t *testing.T) {
	h, _, _, _ := newClaimTestHandler()
	app := newClaimTestApp(h)

	req := httptest.NewRequest("GET", "/claim/ATAP-NOPE", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestClaimPageExpired(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-EXPD",
		AgentID:   agent.ID,
		AgentName: "TestBot",
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour), // already expired
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)
	req := httptest.NewRequest("GET", "/claim/ATAP-EXPD", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 410 {
		t.Fatalf("expected 410 for expired claim, got %d", resp.StatusCode)
	}
}

func TestClaimPageAlreadyRedeemed(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-USED",
		AgentID:   agent.ID,
		AgentName: "TestBot",
		Status:    models.ClaimStatusRedeemed,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)
	req := httptest.NewRequest("GET", "/claim/ATAP-USED", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 410 {
		t.Fatalf("expected 410 for redeemed claim, got %d", resp.StatusCode)
	}
}

// ============================================================
// TESTS: POST /claim/:code/auth (start OTP)
// ============================================================

func TestClaimStartAuth(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-AUTH",
		AgentID:   agent.ID,
		AgentName: "TestBot",
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest("POST", "/claim/ATAP-AUTH/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestClaimStartAuthMissingEmail(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-NOML",
		AgentID:   agent.ID,
		Status:    models.ClaimStatusPending,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/claim/ATAP-NOML/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing email, got %d", resp.StatusCode)
	}
}

// ============================================================
// TESTS: POST /claim/:code/approve (full flow)
// ============================================================

func TestClaimApprove(t *testing.T) {
	h, cls, es, ms := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-APRV",
		AgentID:   agent.ID,
		AgentName: "TestBot",
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	// Pre-store OTP in Redis for test
	email := "alice@example.com"
	otp := "123456"
	otpKey := fmt.Sprintf("otp:claim:%s:%s", claim.Code, email)
	h.redis.Set(context.Background(), otpKey, otp, 10*time.Minute)

	app := newClaimTestApp(h)

	body, _ := json.Marshal(map[string]string{
		"email": email,
		"otp":   otp,
	})
	req := httptest.NewRequest("POST", "/claim/ATAP-APRV/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, errResp)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "approved" {
		t.Errorf("expected status approved, got %v", result["status"])
	}
	if result["human_did"] == nil || result["human_did"] == "" {
		t.Error("expected human_did in response")
	}

	// Verify claim was redeemed
	if claim.Status != models.ClaimStatusRedeemed {
		t.Errorf("expected claim status redeemed, got %s", claim.Status)
	}

	// Verify a human entity was created
	humanDID := result["human_did"].(string)
	found := false
	for _, e := range es.entities {
		if e.DID == humanDID && e.Type == models.EntityTypeHuman {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected human entity to be created in store")
	}

	// Verify agent's principal_did was set
	if cls.principalOf[agent.ID] != humanDID {
		t.Errorf("expected agent principal_did to be %s, got %s", humanDID, cls.principalOf[agent.ID])
	}

	// Verify agent was notified
	if len(ms.messages) == 0 {
		t.Error("expected DIDComm notification to agent")
	} else {
		msg := ms.messages[0]
		if msg.RecipientDID != agent.DID {
			t.Errorf("expected notification to %s, got %s", agent.DID, msg.RecipientDID)
		}
		if msg.MessageType != "https://atap.dev/protocols/claim/1.0/redeemed" {
			t.Errorf("expected claim/redeemed message type, got %s", msg.MessageType)
		}
	}
}

func TestClaimApproveInvalidOTP(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-BADO",
		AgentID:   agent.ID,
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	// Store a different OTP
	email := "bob@example.com"
	otpKey := fmt.Sprintf("otp:claim:%s:%s", claim.Code, email)
	h.redis.Set(context.Background(), otpKey, "999999", 10*time.Minute)

	app := newClaimTestApp(h)

	body, _ := json.Marshal(map[string]string{
		"email": email,
		"otp":   "000000", // wrong
	})
	req := httptest.NewRequest("POST", "/claim/ATAP-BADO/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid OTP, got %d", resp.StatusCode)
	}

	// Claim should still be pending
	if claim.Status != models.ClaimStatusPending {
		t.Errorf("expected claim still pending, got %s", claim.Status)
	}
}

func TestClaimApproveExpired(t *testing.T) {
	h, cls, es, _ := newClaimTestHandler()
	agent := seedAgent(es)

	claim := &models.Claim{
		ID:        crypto.NewClaimID(),
		Code:      "ATAP-EXPR",
		AgentID:   agent.ID,
		Status:    models.ClaimStatusPending,
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	cls.claims[claim.Code] = claim

	app := newClaimTestApp(h)

	body, _ := json.Marshal(map[string]string{
		"email": "expired@example.com",
		"otp":   "123456",
	})
	req := httptest.NewRequest("POST", "/claim/ATAP-EXPR/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req)
	if resp.StatusCode != 410 {
		t.Fatalf("expected 410 for expired claim, got %d", resp.StatusCode)
	}
}

// ============================================================
// TESTS: Claim code generation
// ============================================================

func TestClaimCodeFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := crypto.NewClaimCode()
		if len(code) != 9 { // "ATAP-" + 4 chars
			t.Errorf("expected code length 9, got %d: %s", len(code), code)
		}
		if code[:5] != "ATAP-" {
			t.Errorf("expected ATAP- prefix, got %s", code)
		}
	}
}

func TestClaimIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := crypto.NewClaimID()
		if len(id) < 4 || id[:4] != "clm_" {
			t.Errorf("expected clm_ prefix, got %s", id)
		}
	}
}
