package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

// fakeStore implements EntityStore, SignalStore, ChannelStore, and WebhookStore with in-memory maps.
type fakeStore struct {
	entities         map[string]*models.Entity
	keyIndex         map[string]*models.Entity // keyID -> entity
	signals          map[string]*models.Signal
	signalsByTarget  map[string][]*models.Signal
	idempotencyKeys  map[string]bool
	channels         map[string]*models.Channel
	webhookConfigs   map[string]*models.WebhookConfig
	deliveryAttempts []*models.DeliveryAttempt
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		entities:        make(map[string]*models.Entity),
		keyIndex:        make(map[string]*models.Entity),
		signals:         make(map[string]*models.Signal),
		signalsByTarget: make(map[string][]*models.Signal),
		idempotencyKeys: make(map[string]bool),
		channels:        make(map[string]*models.Channel),
		webhookConfigs:  make(map[string]*models.WebhookConfig),
	}
}

func (f *fakeStore) CreateEntity(_ context.Context, e *models.Entity) error {
	f.entities[e.ID] = e
	f.keyIndex[e.KeyID] = e
	return nil
}

func (f *fakeStore) GetEntity(_ context.Context, id string) (*models.Entity, error) {
	e, ok := f.entities[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (f *fakeStore) GetEntityByKeyID(_ context.Context, keyID string) (*models.Entity, error) {
	e, ok := f.keyIndex[keyID]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (f *fakeStore) SaveSignal(_ context.Context, s *models.Signal) error {
	// Check idempotency
	if s.Context.Idempotency != "" {
		if f.idempotencyKeys[s.Context.Idempotency] {
			return store.ErrDuplicateSignal
		}
		f.idempotencyKeys[s.Context.Idempotency] = true
	}
	f.signals[s.ID] = s
	f.signalsByTarget[s.TargetEntityID] = append(f.signalsByTarget[s.TargetEntityID], s)
	return nil
}

func (f *fakeStore) GetSignal(_ context.Context, id string) (*models.Signal, error) {
	s, ok := f.signals[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (f *fakeStore) GetInbox(_ context.Context, entityID, after string, limit int) ([]*models.Signal, bool, error) {
	all := f.signalsByTarget[entityID]

	// Sort by ID ascending
	sorted := make([]*models.Signal, len(all))
	copy(sorted, all)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	// Filter expired
	now := time.Now()
	var filtered []*models.Signal
	for _, s := range sorted {
		if s.ExpiresAt != nil && s.ExpiresAt.Before(now) {
			continue
		}
		filtered = append(filtered, s)
	}

	// Apply cursor
	if after != "" {
		var afterIdx int = -1
		for i, s := range filtered {
			if s.ID == after {
				afterIdx = i
				break
			}
		}
		if afterIdx >= 0 {
			filtered = filtered[afterIdx+1:]
		}
	}

	// Apply limit
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[:limit]
	}

	return filtered, hasMore, nil
}

func (f *fakeStore) GetSignalsAfter(_ context.Context, entityID, afterID string) ([]*models.Signal, error) {
	all := f.signalsByTarget[entityID]

	sorted := make([]*models.Signal, len(all))
	copy(sorted, all)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var result []*models.Signal
	for _, s := range sorted {
		if s.ID > afterID {
			result = append(result, s)
		}
		if len(result) >= 1000 {
			break
		}
	}
	return result, nil
}

// ChannelStore methods

func (f *fakeStore) CreateChannel(_ context.Context, ch *models.Channel) error {
	f.channels[ch.ID] = ch
	return nil
}

func (f *fakeStore) GetChannel(_ context.Context, id string) (*models.Channel, error) {
	ch, ok := f.channels[id]
	if !ok {
		return nil, nil
	}
	return ch, nil
}

func (f *fakeStore) ListChannels(_ context.Context, entityID string) ([]*models.Channel, error) {
	var result []*models.Channel
	for _, ch := range f.channels {
		if ch.EntityID == entityID && ch.Active {
			result = append(result, ch)
		}
	}
	return result, nil
}

func (f *fakeStore) RevokeChannel(_ context.Context, id string) error {
	if ch, ok := f.channels[id]; ok {
		ch.Active = false
		now := time.Now().UTC()
		ch.RevokedAt = &now
	}
	return nil
}

func (f *fakeStore) IncrementChannelSignalCount(_ context.Context, channelID string) error {
	if ch, ok := f.channels[channelID]; ok {
		ch.SignalCount++
	}
	return nil
}

// WebhookStore methods

func (f *fakeStore) GetWebhookConfig(_ context.Context, entityID string) (*models.WebhookConfig, error) {
	wh, ok := f.webhookConfigs[entityID]
	if !ok {
		return nil, nil
	}
	return wh, nil
}

func (f *fakeStore) SetWebhookConfig(_ context.Context, entityID, url string) error {
	now := time.Now().UTC()
	f.webhookConfigs[entityID] = &models.WebhookConfig{
		EntityID:  entityID,
		URL:       url,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}

func (f *fakeStore) DeleteWebhookConfig(_ context.Context, entityID string) error {
	delete(f.webhookConfigs, entityID)
	return nil
}

func (f *fakeStore) UpdateSignalDeliveryStatus(_ context.Context, signalID, status string) error {
	if s, ok := f.signals[signalID]; ok {
		s.DeliveryStatus = status
	}
	return nil
}

func (f *fakeStore) SaveDeliveryAttempt(_ context.Context, a *models.DeliveryAttempt) error {
	f.deliveryAttempts = append(f.deliveryAttempts, a)
	return nil
}

func (f *fakeStore) GetPendingRetries(_ context.Context, now time.Time) ([]*models.DeliveryAttempt, error) {
	var result []*models.DeliveryAttempt
	for _, a := range f.deliveryAttempts {
		if a.NextRetryAt != nil && !a.NextRetryAt.After(now) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (f *fakeStore) CleanupDeliveryAttempts(_ context.Context, olderThan time.Time) (int64, error) {
	var remaining []*models.DeliveryAttempt
	var deleted int64
	for _, a := range f.deliveryAttempts {
		if a.CreatedAt.Before(olderThan) {
			deleted++
		} else {
			remaining = append(remaining, a)
		}
	}
	f.deliveryAttempts = remaining
	return deleted, nil
}

func setupTestApp(s *fakeStore) *fiber.App {
	log := zerolog.Nop()
	cfg := &config.Config{PlatformDomain: "test.atap.app"}
	// Pass nil for redis and platformKey in unit tests; fakeStore satisfies all store interfaces
	handler := NewHandler(s, s, s, s, nil, nil, cfg, log)

	app := fiber.New()
	handler.SetupRoutes(app)
	return app
}

func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("parse body: %v (raw: %s)", err, string(body))
	}
	return result
}

// signedRequest creates an HTTP request with Ed25519 signature auth headers.
// The signature is computed over the path portion only (no query string),
// matching Fiber's c.Path() behavior in the auth middleware.
func signedRequest(method, fullPath string, privKey ed25519.PrivateKey, keyID string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, fullPath, body)
	ts := time.Now().UTC()
	req.Header.Set("X-Atap-Timestamp", ts.Format(time.RFC3339))
	// Sign over the path without query string (Fiber c.Path() strips query params)
	signPath := fullPath
	if idx := strings.Index(fullPath, "?"); idx >= 0 {
		signPath = fullPath[:idx]
	}
	req.Header.Set("Authorization", crypto.SignRequest(privKey, keyID, method, signPath, ts))
	return req
}

// createTestEntity creates and registers an entity in the fakeStore, returning the entity and its private key.
func createTestEntity(fs *fakeStore, id, name string) (*models.Entity, ed25519.PrivateKey) {
	pubKey, privKey, _ := crypto.GenerateKeyPair()
	keyID := fmt.Sprintf("key_%s_test1234", id[:8])
	entity := &models.Entity{
		ID:               id,
		Type:             models.EntityTypeAgent,
		URI:              "agent://" + id,
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             name,
		TrustLevel:       models.TrustLevel0,
		Registry:         "test.atap.app",
		CreatedAt:        time.Now().UTC(),
	}
	fs.CreateEntity(context.Background(), entity)
	return entity, privKey
}

// buildSignedSignalBody creates a JSON signal body with a valid Ed25519 signature.
func buildSignedSignalBody(t *testing.T, sender *models.Entity, privKey ed25519.PrivateKey, target *models.Entity, opts ...func(req *models.SendSignalRequest)) string {
	t.Helper()

	route := models.SignalRoute{
		Origin: sender.URI,
		Target: target.URI,
	}
	sigBody := models.SignalBody{
		Type: "message",
		Data: json.RawMessage(`{"text":"hello"}`),
	}

	req := &models.SendSignalRequest{
		Route:  route,
		Signal: sigBody,
		Trust: models.SignalTrust{
			SignerKeyID: sender.KeyID,
		},
		Context: models.SignalContext{
			Source: models.SignalSourceAgent,
		},
	}

	// Apply any overrides
	for _, opt := range opts {
		opt(req)
	}

	// Sign
	payload, err := crypto.SignablePayload(req.Route, req.Signal)
	if err != nil {
		t.Fatalf("build signable payload: %v", err)
	}
	sig := crypto.Sign(privKey, payload)
	req.Trust.Signature = base64.RawURLEncoding.EncodeToString(sig)

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal signal request: %v", err)
	}
	return string(data)
}

// ============================================================
// TESTS
// ============================================================

func TestHealth(t *testing.T) {
	app := setupTestApp(newFakeStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)
	tests := []struct {
		field string
		want  string
	}{
		{"status", "ok"},
		{"protocol", "ATAP"},
		{"version", "0.1"},
	}
	for _, tc := range tests {
		got, ok := body[tc.field]
		if !ok {
			t.Errorf("missing field %q", tc.field)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
	if _, ok := body["time"]; !ok {
		t.Error("missing field \"time\"")
	}
}

func TestRegisterAgent(t *testing.T) {
	app := setupTestApp(newFakeStore())

	req := httptest.NewRequest(http.MethodPost, "/v1/register",
		strings.NewReader(`{"name":"test-agent"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)

	// Required fields (no token)
	requiredFields := []string{"uri", "id", "public_key", "private_key", "key_id"}
	for _, f := range requiredFields {
		if _, ok := body[f]; !ok {
			t.Errorf("missing required field %q", f)
		}
	}

	// URI starts with agent://
	if uri, ok := body["uri"].(string); ok {
		if !strings.HasPrefix(uri, "agent://") {
			t.Errorf("uri = %q, want prefix agent://", uri)
		}
	}

	// ID is 26 chars lowercase (ULID)
	if id, ok := body["id"].(string); ok {
		if len(id) != 26 {
			t.Errorf("id length = %d, want 26", len(id))
		}
		if id != strings.ToLower(id) {
			t.Errorf("id = %q, want lowercase", id)
		}
	}

	// KeyID starts with key_
	if keyID, ok := body["key_id"].(string); ok {
		if !strings.HasPrefix(keyID, "key_") {
			t.Errorf("key_id = %q, want prefix key_", keyID)
		}
	}

	// NO token, inbox_url, or stream_url
	forbiddenFields := []string{"token", "inbox_url", "stream_url"}
	for _, f := range forbiddenFields {
		if _, ok := body[f]; ok {
			t.Errorf("unexpected field %q in response", f)
		}
	}
}

func TestRegisterAgent_EmptyBody(t *testing.T) {
	app := setupTestApp(newFakeStore())

	req := httptest.NewRequest(http.MethodPost, "/v1/register",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201 (name is optional), got %d", resp.StatusCode)
	}
}

func TestGetEntity(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	// Pre-populate entity
	pubKey, _, _ := crypto.GenerateKeyPair()
	entity := &models.Entity{
		ID:               "01hytest00000000testentity",
		Type:             models.EntityTypeAgent,
		URI:              "agent://01hytest00000000testentity",
		PublicKeyEd25519: pubKey,
		KeyID:            "key_test_abcd1234",
		Name:             "lookup-test",
		TrustLevel:       0,
		Registry:         "test.atap.app",
		CreatedAt:        time.Now().UTC(),
	}
	fs.CreateEntity(context.Background(), entity)

	req := httptest.NewRequest(http.MethodGet, "/v1/entities/01hytest00000000testentity", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)

	// Required fields present
	expectedFields := []string{"id", "type", "uri", "public_key", "key_id", "trust_level", "registry", "created_at"}
	for _, f := range expectedFields {
		if _, ok := body[f]; !ok {
			t.Errorf("missing expected field %q", f)
		}
	}

	// Verify values
	if body["id"] != "01hytest00000000testentity" {
		t.Errorf("id = %v, want 01hytest00000000testentity", body["id"])
	}
	if body["type"] != "agent" {
		t.Errorf("type = %v, want agent", body["type"])
	}
	if body["name"] != "lookup-test" {
		t.Errorf("name = %v, want lookup-test", body["name"])
	}

	// Secret/internal fields must NOT be present
	forbiddenFields := []string{"token_hash", "token", "delivery_pref", "webhook_url"}
	for _, f := range forbiddenFields {
		if _, ok := body[f]; ok {
			t.Errorf("unexpected secret field %q in response", f)
		}
	}
}

func TestGetEntity_NotFound(t *testing.T) {
	app := setupTestApp(newFakeStore())

	req := httptest.NewRequest(http.MethodGet, "/v1/entities/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)
	verifyRFC7807(t, body)
}

func TestAuthRequired(t *testing.T) {
	app := setupTestApp(newFakeStore())

	tests := []struct {
		name string
		auth string
		ts   string
	}{
		{"no header", "", ""},
		{"invalid format", "Bearer atap_invalidtoken123", ""},
		{"unknown key", "", "use_signature"}, // will create a valid sig with unknown key
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			if tc.ts == "use_signature" {
				// Valid signature format but unknown keyID
				_, priv, _ := crypto.GenerateKeyPair()
				ts := time.Now().UTC()
				req.Header.Set("X-Atap-Timestamp", ts.Format(time.RFC3339))
				req.Header.Set("Authorization", crypto.SignRequest(priv, "key_unknown_12345678", "GET", "/v1/me", ts))
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("test request: %v", err)
			}

			if resp.StatusCode != 401 {
				t.Fatalf("expected 401, got %d", resp.StatusCode)
			}

			body := parseBody(t, resp)
			verifyRFC7807(t, body)
		})
	}
}

func TestAuthValid(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	// Create entity with known keypair
	pubKey, privKey, _ := crypto.GenerateKeyPair()
	keyID := "key_auth_test1234"
	entity := &models.Entity{
		ID:               "01hyauth00000000validtoken",
		Type:             models.EntityTypeAgent,
		URI:              "agent://01hyauth00000000validtoken",
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             "auth-test",
		TrustLevel:       0,
		Registry:         "test.atap.app",
		CreatedAt:        time.Now().UTC(),
	}
	fs.CreateEntity(context.Background(), entity)

	req := signedRequest("GET", "/v1/me", privKey, keyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)
	if body["id"] != "01hyauth00000000validtoken" {
		t.Errorf("id = %v, want 01hyauth00000000validtoken", body["id"])
	}
	if body["name"] != "auth-test" {
		t.Errorf("name = %v, want auth-test", body["name"])
	}
}

func TestAuthWrongKey(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	// Create entity with one keypair
	pubKey, _, _ := crypto.GenerateKeyPair()
	keyID := "key_wrong_test1234"
	entity := &models.Entity{
		ID:               "01hywrong0000000wrongkey00",
		Type:             models.EntityTypeAgent,
		URI:              "agent://01hywrong0000000wrongkey00",
		PublicKeyEd25519: pubKey,
		KeyID:            keyID,
		Name:             "wrong-key-test",
		TrustLevel:       0,
		Registry:         "test.atap.app",
		CreatedAt:        time.Now().UTC(),
	}
	fs.CreateEntity(context.Background(), entity)

	// Sign with a DIFFERENT private key
	_, wrongPrivKey, _ := crypto.GenerateKeyPair()
	req := signedRequest("GET", "/v1/me", wrongPrivKey, keyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for wrong key, got %d", resp.StatusCode)
	}
}

func TestErrorFormat(t *testing.T) {
	app := setupTestApp(newFakeStore())

	// Use a 404 to test error format
	req := httptest.NewRequest(http.MethodGet, "/v1/entities/doesnotexist", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)
	verifyRFC7807(t, body)

	// Verify specific RFC 7807 field values
	if typeVal, ok := body["type"].(string); ok {
		if !strings.HasPrefix(typeVal, "https://atap.dev/errors/") {
			t.Errorf("type = %q, want prefix https://atap.dev/errors/", typeVal)
		}
	}

	if status, ok := body["status"].(float64); ok {
		if int(status) != 404 {
			t.Errorf("status = %v, want 404", status)
		}
	}

	if _, ok := body["instance"].(string); !ok {
		t.Error("instance should be a string path")
	}
}

// ============================================================
// SIGNAL TESTS
// ============================================================

func TestSendSignal(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000senderaa00", "sender-agent")
	targetEntity, _ := createTestEntity(fs, "01hytarg00000000targetaa00", "target-agent")

	body := buildSignedSignalBody(t, senderEntity, senderPriv, targetEntity)
	path := fmt.Sprintf("/v1/inbox/%s", targetEntity.ID)

	req := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 202 {
		respBody := parseBody(t, resp)
		t.Fatalf("expected 202, got %d: %v", resp.StatusCode, respBody)
	}

	respBody := parseBody(t, resp)

	// Signal ID starts with sig_
	sigID, ok := respBody["id"].(string)
	if !ok {
		t.Fatal("missing id field in response")
	}
	if !strings.HasPrefix(sigID, "sig_") {
		t.Errorf("signal id = %q, want prefix sig_", sigID)
	}

	// Signal persisted in store
	if len(fs.signals) != 1 {
		t.Errorf("expected 1 signal in store, got %d", len(fs.signals))
	}

	// Check route fields
	route, ok := respBody["route"].(map[string]interface{})
	if !ok {
		t.Fatal("missing route in response")
	}
	if route["origin"] != senderEntity.URI {
		t.Errorf("route.origin = %v, want %v", route["origin"], senderEntity.URI)
	}
	if route["target"] != targetEntity.URI {
		t.Errorf("route.target = %v, want %v", route["target"], targetEntity.URI)
	}

	// Check trust fields
	trust, ok := respBody["trust"].(map[string]interface{})
	if !ok {
		t.Fatal("missing trust in response")
	}
	if trust["signer"] != senderEntity.URI {
		t.Errorf("trust.signer = %v, want %v", trust["signer"], senderEntity.URI)
	}
	if trust["signer_key_id"] != senderEntity.KeyID {
		t.Errorf("trust.signer_key_id = %v, want %v", trust["signer_key_id"], senderEntity.KeyID)
	}
}

func TestSendSignal_TargetNotFound(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000sender0001", "sender")

	// Create a fake target for building the signed body (but don't register it)
	fakeTarget := &models.Entity{
		ID:   "01hynonexist00000000000000",
		URI:  "agent://01hynonexist00000000000000",
		KeyID: "key_fake_00000000",
	}

	body := buildSignedSignalBody(t, senderEntity, senderPriv, fakeTarget)
	path := "/v1/inbox/01hynonexist00000000000000"

	req := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSendSignal_PayloadTooLarge(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000sender0002", "sender")
	_, _ = createTestEntity(fs, "01hytarg00000000target0002", "target")

	// Create a body larger than MaxSignalPayload (64KB)
	largeData := strings.Repeat("x", models.MaxSignalPayload+1)
	path := "/v1/inbox/01hytarg00000000target0002"

	req := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(largeData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 413 {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestSendSignal_IdempotencyDedup(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000sender0003", "sender")
	targetEntity, _ := createTestEntity(fs, "01hytarg00000000target0003", "target")

	body := buildSignedSignalBody(t, senderEntity, senderPriv, targetEntity, func(req *models.SendSignalRequest) {
		req.Context.Idempotency = "unique-key-12345"
	})
	path := fmt.Sprintf("/v1/inbox/%s", targetEntity.ID)

	// First send: should succeed
	req1 := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")

	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatalf("test request 1: %v", err)
	}
	if resp1.StatusCode != 202 {
		t.Fatalf("first send: expected 202, got %d", resp1.StatusCode)
	}

	// Second send with same idempotency key: should get 409
	req2 := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("test request 2: %v", err)
	}
	if resp2.StatusCode != 409 {
		t.Fatalf("second send: expected 409, got %d", resp2.StatusCode)
	}
}

func TestSendSignal_NotAuthenticated(t *testing.T) {
	app := setupTestApp(newFakeStore())

	req := httptest.NewRequest(http.MethodPost, "/v1/inbox/someentity", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ============================================================
// INBOX TESTS
// ============================================================

func TestGetInbox(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000sender0010", "sender")
	targetEntity, targetPriv := createTestEntity(fs, "01hytarg00000000target0010", "target")

	// Send 3 signals
	for i := 0; i < 3; i++ {
		body := buildSignedSignalBody(t, senderEntity, senderPriv, targetEntity)
		path := fmt.Sprintf("/v1/inbox/%s", targetEntity.ID)

		req := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("send signal %d: %v", i, err)
		}
		if resp.StatusCode != 202 {
			t.Fatalf("send signal %d: expected 202, got %d", i, resp.StatusCode)
		}
	}

	// Get inbox as target
	path := fmt.Sprintf("/v1/inbox/%s", targetEntity.ID)
	req := signedRequest("GET", path, targetPriv, targetEntity.KeyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("get inbox: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)

	signals, ok := body["signals"].([]interface{})
	if !ok {
		t.Fatal("missing or invalid signals field")
	}
	if len(signals) != 3 {
		t.Errorf("expected 3 signals, got %d", len(signals))
	}

	hasMore, _ := body["has_more"].(bool)
	if hasMore {
		t.Error("expected has_more=false")
	}
}

func TestGetInbox_Pagination(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	senderEntity, senderPriv := createTestEntity(fs, "01hysend00000000sender0011", "sender")
	targetEntity, targetPriv := createTestEntity(fs, "01hytarg00000000target0011", "target")

	// Send 5 signals
	for i := 0; i < 5; i++ {
		body := buildSignedSignalBody(t, senderEntity, senderPriv, targetEntity)
		path := fmt.Sprintf("/v1/inbox/%s", targetEntity.ID)

		req := signedRequest("POST", path, senderPriv, senderEntity.KeyID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("send signal %d: %v", i, err)
		}
		if resp.StatusCode != 202 {
			t.Fatalf("send signal %d: expected 202, got %d", i, resp.StatusCode)
		}
	}

	// Page 1: limit=2
	path := fmt.Sprintf("/v1/inbox/%s?limit=2", targetEntity.ID)
	req := signedRequest("GET", path, targetPriv, targetEntity.KeyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("page 1: expected 200, got %d", resp.StatusCode)
	}

	page1 := parseBody(t, resp)
	signals1 := page1["signals"].([]interface{})
	if len(signals1) != 2 {
		t.Fatalf("page 1: expected 2 signals, got %d", len(signals1))
	}

	hasMore1, _ := page1["has_more"].(bool)
	if !hasMore1 {
		t.Error("page 1: expected has_more=true")
	}
	cursor1, _ := page1["cursor"].(string)
	if cursor1 == "" {
		t.Fatal("page 1: expected non-empty cursor")
	}

	// Page 2: limit=2, after=cursor
	path2 := fmt.Sprintf("/v1/inbox/%s?limit=2&after=%s", targetEntity.ID, cursor1)
	req2 := signedRequest("GET", path2, targetPriv, targetEntity.KeyID, nil)

	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	page2 := parseBody(t, resp2)
	signals2 := page2["signals"].([]interface{})
	if len(signals2) != 2 {
		t.Fatalf("page 2: expected 2 signals, got %d", len(signals2))
	}

	hasMore2, _ := page2["has_more"].(bool)
	if !hasMore2 {
		t.Error("page 2: expected has_more=true")
	}
	cursor2, _ := page2["cursor"].(string)

	// Page 3: should have 1 remaining
	path3 := fmt.Sprintf("/v1/inbox/%s?limit=2&after=%s", targetEntity.ID, cursor2)
	req3 := signedRequest("GET", path3, targetPriv, targetEntity.KeyID, nil)

	resp3, err := app.Test(req3)
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	page3 := parseBody(t, resp3)
	signals3 := page3["signals"].([]interface{})
	if len(signals3) != 1 {
		t.Fatalf("page 3: expected 1 signal, got %d", len(signals3))
	}

	hasMore3, _ := page3["has_more"].(bool)
	if hasMore3 {
		t.Error("page 3: expected has_more=false")
	}
}

func TestGetInbox_WrongEntity(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	entityA, privA := createTestEntity(fs, "01hyaaaa00000000entitya000", "entity-a")
	_, _ = createTestEntity(fs, "01hybbbb00000000entityb000", "entity-b")

	// Entity A tries to read entity B's inbox
	path := "/v1/inbox/01hybbbb00000000entityb000"
	req := signedRequest("GET", path, privA, entityA.KeyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// ============================================================
// WEBHOOK TESTS
// ============================================================

func TestSetWebhook(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	entity, privKey := createTestEntity(fs, "01hywh0000000000webhook000", "webhook-test")

	path := fmt.Sprintf("/v1/entities/%s/webhook", entity.ID)
	body := `{"url":"https://example.com/webhook"}`

	req := signedRequest("POST", path, privKey, entity.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 200 {
		respBody := parseBody(t, resp)
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, respBody)
	}

	respBody := parseBody(t, resp)
	if respBody["webhook_url"] != "https://example.com/webhook" {
		t.Errorf("webhook_url = %v, want https://example.com/webhook", respBody["webhook_url"])
	}
	if _, ok := respBody["updated_at"]; !ok {
		t.Error("missing updated_at in response")
	}

	// Verify stored
	wh, _ := fs.GetWebhookConfig(context.Background(), entity.ID)
	if wh == nil {
		t.Fatal("webhook config not stored")
	}
	if wh.URL != "https://example.com/webhook" {
		t.Errorf("stored URL = %v, want https://example.com/webhook", wh.URL)
	}
}

func TestSetWebhook_WrongEntity(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	entityA, privA := createTestEntity(fs, "01hywh0100000000webhookaaa", "entity-a")
	entityB, _ := createTestEntity(fs, "01hywh0200000000webhookbbb", "entity-b")

	// Entity A tries to set webhook for entity B
	path := fmt.Sprintf("/v1/entities/%s/webhook", entityB.ID)
	body := `{"url":"https://example.com/webhook"}`

	req := signedRequest("POST", path, privA, entityA.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// ============================================================
// CHANNEL TESTS
// ============================================================

func TestCreateChannel_Trusted(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hyownr00000000trustedch", "channel-owner")
	trustee, _ := createTestEntity(fs, "01hytrus00000000trusteeid", "trustee-agent")

	path := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	body := fmt.Sprintf(`{"type":"trusted","label":"GitHub Webhook","trustee_id":"%s"}`, trustee.ID)

	req := signedRequest("POST", path, ownerPriv, owner.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 201 {
		respBody := parseBody(t, resp)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, respBody)
	}

	respBody := parseBody(t, resp)

	// Channel ID starts with chn_
	chID, ok := respBody["id"].(string)
	if !ok || !strings.HasPrefix(chID, "chn_") {
		t.Errorf("channel id = %v, want prefix chn_", chID)
	}

	// Webhook URL constructed correctly
	webhookURL, _ := respBody["webhook_url"].(string)
	if !strings.Contains(webhookURL, "/v1/channels/") || !strings.Contains(webhookURL, "/signals") {
		t.Errorf("webhook_url = %v, unexpected format", webhookURL)
	}

	// No basic_auth_password for trusted channels
	if _, ok := respBody["basic_auth_password"]; ok {
		pw := respBody["basic_auth_password"]
		if pw != nil && pw != "" {
			t.Error("trusted channel should not have basic_auth_password")
		}
	}

	// Type is trusted
	if respBody["type"] != "trusted" {
		t.Errorf("type = %v, want trusted", respBody["type"])
	}
}

func TestCreateChannel_Open(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hychn00000000chowneropn", "channel-owner")

	path := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	body := `{"type":"open","label":"Public Webhook"}`

	req := signedRequest("POST", path, ownerPriv, owner.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 201 {
		respBody := parseBody(t, resp)
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, respBody)
	}

	respBody := parseBody(t, resp)

	// Open channel should include basic_auth_password
	pw, ok := respBody["basic_auth_password"].(string)
	if !ok || pw == "" {
		t.Error("open channel should include basic_auth_password")
	}

	// Type is open
	if respBody["type"] != "open" {
		t.Errorf("type = %v, want open", respBody["type"])
	}
}

func TestCreateChannel_InvalidType(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hychn00000000chinvtype0", "channel-owner")

	path := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	body := `{"type":"invalid","label":"Bad Channel"}`

	req := signedRequest("POST", path, ownerPriv, owner.KeyID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestListChannels(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hylsto00000000chlistowne", "channel-owner")
	trustee, _ := createTestEntity(fs, "01hylstt00000000chlisttrus", "trustee")

	// Create 2 channels
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(`{"type":"trusted","label":"Channel %d","trustee_id":"%s"}`, i, trustee.ID)
		path := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
		req := signedRequest("POST", path, ownerPriv, owner.KeyID, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("create channel %d: %v", i, err)
		}
		if resp.StatusCode != 201 {
			t.Fatalf("create channel %d: expected 201, got %d", i, resp.StatusCode)
		}
	}

	// List channels
	path := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	req := signedRequest("GET", path, ownerPriv, owner.KeyID, nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	respBody := parseBody(t, resp)
	channels, ok := respBody["channels"].([]interface{})
	if !ok {
		t.Fatal("missing channels field")
	}
	if len(channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(channels))
	}
}

func TestRevokeChannel(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hyrevo00000000chrevokeow", "channel-owner")
	trustee, _ := createTestEntity(fs, "01hyrevt00000000chrevoketr", "trustee")

	// Create channel
	createBody := fmt.Sprintf(`{"type":"trusted","label":"Revoke Me","trustee_id":"%s"}`, trustee.ID)
	createPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	createReq := signedRequest("POST", createPath, ownerPriv, owner.KeyID, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if createResp.StatusCode != 201 {
		t.Fatalf("create channel: expected 201, got %d", createResp.StatusCode)
	}

	createRespBody := parseBody(t, createResp)
	channelID := createRespBody["id"].(string)

	// Revoke channel
	revokePath := fmt.Sprintf("/v1/entities/%s/channels/%s", owner.ID, channelID)
	revokeReq := signedRequest("DELETE", revokePath, ownerPriv, owner.KeyID, nil)

	revokeResp, err := app.Test(revokeReq)
	if err != nil {
		t.Fatalf("revoke channel: %v", err)
	}
	if revokeResp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", revokeResp.StatusCode)
	}

	// List channels should return 0
	listPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	listReq := signedRequest("GET", listPath, ownerPriv, owner.KeyID, nil)

	listResp, err := app.Test(listReq)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	listBody := parseBody(t, listResp)
	channels := listBody["channels"].([]interface{})
	if len(channels) != 0 {
		t.Errorf("expected 0 active channels after revoke, got %d", len(channels))
	}
}

func TestChannelInbound_Open(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hychn00000000chinbndopn", "channel-owner")

	// Create open channel
	createPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	createBody := `{"type":"open","label":"Open Inbound"}`
	createReq := signedRequest("POST", createPath, ownerPriv, owner.KeyID, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if createResp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d", createResp.StatusCode)
	}

	createRespBody := parseBody(t, createResp)
	channelID := createRespBody["id"].(string)
	basicAuthPw := createRespBody["basic_auth_password"].(string)

	// POST to channel inbound with Basic Auth
	inboundPath := fmt.Sprintf("/v1/channels/%s/signals", channelID)
	inboundBody := `{"event":"payment.completed","amount":1000}`

	inboundReq := httptest.NewRequest("POST", inboundPath, strings.NewReader(inboundBody))
	inboundReq.Header.Set("Content-Type", "application/json")
	// Basic Auth: username can be anything, password is the basicAuthPw
	basicAuth := base64.StdEncoding.EncodeToString([]byte("channel:" + basicAuthPw))
	inboundReq.Header.Set("Authorization", "Basic "+basicAuth)

	inboundResp, err := app.Test(inboundReq)
	if err != nil {
		t.Fatalf("inbound request: %v", err)
	}

	if inboundResp.StatusCode != 202 {
		respBody := parseBody(t, inboundResp)
		t.Fatalf("expected 202, got %d: %v", inboundResp.StatusCode, respBody)
	}

	// Verify signal was created in store
	if len(fs.signals) != 1 {
		t.Fatalf("expected 1 signal in store, got %d", len(fs.signals))
	}

	// Find the signal and verify its properties
	for _, sig := range fs.signals {
		if sig.Context.Source != models.SignalSourceExternal {
			t.Errorf("signal source = %v, want %v", sig.Context.Source, models.SignalSourceExternal)
		}
		if sig.Trust.Level != 0 {
			t.Errorf("signal trust level = %d, want 0", sig.Trust.Level)
		}
		if sig.Signal.Type != "channel.inbound" {
			t.Errorf("signal type = %v, want channel.inbound", sig.Signal.Type)
		}
		if sig.Route.Channel != channelID {
			t.Errorf("signal channel = %v, want %v", sig.Route.Channel, channelID)
		}
		if !strings.HasPrefix(sig.Route.Origin, "external://") {
			t.Errorf("signal origin = %v, want prefix external://", sig.Route.Origin)
		}
	}

	// Verify channel signal count incremented
	ch, _ := fs.GetChannel(context.Background(), channelID)
	if ch.SignalCount != 1 {
		t.Errorf("channel signal count = %d, want 1", ch.SignalCount)
	}
}

func TestChannelInbound_Trusted(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hyitro00000000chinbndtru", "channel-owner")
	trustee, trusteePriv := createTestEntity(fs, "01hyitrt00000000chinbndtrs", "trustee-agent")

	// Create trusted channel
	createPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	createBody := fmt.Sprintf(`{"type":"trusted","label":"Trusted Inbound","trustee_id":"%s"}`, trustee.ID)
	createReq := signedRequest("POST", createPath, ownerPriv, owner.KeyID, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if createResp.StatusCode != 201 {
		t.Fatalf("create: expected 201, got %d", createResp.StatusCode)
	}

	createRespBody := parseBody(t, createResp)
	channelID := createRespBody["id"].(string)

	// POST to channel inbound with trustee's Ed25519 signature
	inboundPath := fmt.Sprintf("/v1/channels/%s/signals", channelID)
	inboundBody := `{"event":"agent.action","result":"ok"}`

	inboundReq := signedRequest("POST", inboundPath, trusteePriv, trustee.KeyID, strings.NewReader(inboundBody))
	inboundReq.Header.Set("Content-Type", "application/json")

	inboundResp, err := app.Test(inboundReq)
	if err != nil {
		t.Fatalf("inbound request: %v", err)
	}

	if inboundResp.StatusCode != 202 {
		respBody := parseBody(t, inboundResp)
		t.Fatalf("expected 202, got %d: %v", inboundResp.StatusCode, respBody)
	}

	// Verify signal was created with correct origin
	for _, sig := range fs.signals {
		if sig.Context.Source != models.SignalSourceAgent {
			t.Errorf("signal source = %v, want %v", sig.Context.Source, models.SignalSourceAgent)
		}
		expectedOrigin := fmt.Sprintf("agent://%s", trustee.ID)
		if sig.Route.Origin != expectedOrigin {
			t.Errorf("signal origin = %v, want %v", sig.Route.Origin, expectedOrigin)
		}
	}
}

func TestChannelInbound_Revoked(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hychn00000000chinbndrev", "channel-owner")

	// Create open channel
	createPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	createBody := `{"type":"open","label":"Revoked Channel"}`
	createReq := signedRequest("POST", createPath, ownerPriv, owner.KeyID, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	createRespBody := parseBody(t, createResp)
	channelID := createRespBody["id"].(string)
	basicAuthPw := createRespBody["basic_auth_password"].(string)

	// Revoke it
	revokePath := fmt.Sprintf("/v1/entities/%s/channels/%s", owner.ID, channelID)
	revokeReq := signedRequest("DELETE", revokePath, ownerPriv, owner.KeyID, nil)
	revokeResp, _ := app.Test(revokeReq)
	if revokeResp.StatusCode != 204 {
		t.Fatalf("revoke: expected 204, got %d", revokeResp.StatusCode)
	}

	// POST to revoked channel
	inboundPath := fmt.Sprintf("/v1/channels/%s/signals", channelID)
	inboundReq := httptest.NewRequest("POST", inboundPath, strings.NewReader(`{"data":"test"}`))
	inboundReq.Header.Set("Content-Type", "application/json")
	basicAuth := base64.StdEncoding.EncodeToString([]byte("channel:" + basicAuthPw))
	inboundReq.Header.Set("Authorization", "Basic "+basicAuth)

	inboundResp, err := app.Test(inboundReq)
	if err != nil {
		t.Fatalf("inbound request: %v", err)
	}

	if inboundResp.StatusCode != 410 {
		t.Fatalf("expected 410 Gone, got %d", inboundResp.StatusCode)
	}
}

func TestChannelInbound_BadAuth(t *testing.T) {
	fs := newFakeStore()
	app := setupTestApp(fs)

	owner, ownerPriv := createTestEntity(fs, "01hychn00000000chinbndba0", "channel-owner")

	// Create open channel
	createPath := fmt.Sprintf("/v1/entities/%s/channels", owner.ID)
	createBody := `{"type":"open","label":"Bad Auth Channel"}`
	createReq := signedRequest("POST", createPath, ownerPriv, owner.KeyID, strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	createRespBody := parseBody(t, createResp)
	channelID := createRespBody["id"].(string)

	// POST with wrong credentials
	inboundPath := fmt.Sprintf("/v1/channels/%s/signals", channelID)
	inboundReq := httptest.NewRequest("POST", inboundPath, strings.NewReader(`{"data":"test"}`))
	inboundReq.Header.Set("Content-Type", "application/json")
	wrongAuth := base64.StdEncoding.EncodeToString([]byte("channel:wrong-password"))
	inboundReq.Header.Set("Authorization", "Basic "+wrongAuth)

	inboundResp, err := app.Test(inboundReq)
	if err != nil {
		t.Fatalf("inbound request: %v", err)
	}

	if inboundResp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", inboundResp.StatusCode)
	}
}

func TestWebhookSignature(t *testing.T) {
	// Test that webhook delivery correctly signs payload with Ed25519
	_, platformPriv, _ := crypto.GenerateKeyPair()
	platformPub := platformPriv.Public().(ed25519.PublicKey)

	payload := []byte(`{"id":"sig_test123","signal":{"type":"message"}}`)

	// Sign with the platform key (same as WebhookWorker.deliver does)
	sig := crypto.Sign(platformPriv, payload)

	// Verify with the public key
	if !crypto.Verify(platformPub, payload, sig) {
		t.Error("webhook signature verification failed")
	}

	// Verify tampered payload fails
	tampered := []byte(`{"id":"sig_tampered","signal":{"type":"message"}}`)
	if crypto.Verify(platformPub, tampered, sig) {
		t.Error("tampered payload should not verify")
	}
}

// verifyRFC7807 checks that a response body contains all required RFC 7807 fields.
func verifyRFC7807(t *testing.T, body map[string]interface{}) {
	t.Helper()
	requiredFields := []string{"type", "title", "status"}
	for _, f := range requiredFields {
		if _, ok := body[f]; !ok {
			t.Errorf("RFC 7807: missing required field %q", f)
		}
	}

	// type should be a URL
	if typeVal, ok := body["type"].(string); ok {
		if !strings.HasPrefix(typeVal, "https://") {
			t.Errorf("RFC 7807: type = %q, should be a URL", typeVal)
		}
	}

	// status should be a number
	if _, ok := body["status"].(float64); !ok {
		t.Errorf("RFC 7807: status should be a number, got %T", body["status"])
	}
}
