package api

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// fakeStore implements EntityStore with in-memory maps.
type fakeStore struct {
	entities map[string]*models.Entity
	keyIndex map[string]*models.Entity // keyID -> entity
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		entities: make(map[string]*models.Entity),
		keyIndex: make(map[string]*models.Entity),
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

func setupTestApp(store EntityStore) *fiber.App {
	log := zerolog.Nop()
	cfg := &config.Config{PlatformDomain: "test.atap.app"}
	handler := NewHandler(store, cfg, log)

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
func signedRequest(method, path string, privKey ed25519.PrivateKey, keyID string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	ts := time.Now().UTC()
	req.Header.Set("X-Atap-Timestamp", ts.Format(time.RFC3339))
	req.Header.Set("Authorization", crypto.SignRequest(privKey, keyID, method, path, ts))
	return req
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
