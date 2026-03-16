package api

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/didcomm"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK MESSAGE STORE
// ============================================================

type mockMessageStore struct {
	messages map[string]*models.DIDCommMessage
}

func newMockMessageStore() *mockMessageStore {
	return &mockMessageStore{
		messages: make(map[string]*models.DIDCommMessage),
	}
}

func (m *mockMessageStore) QueueMessage(_ context.Context, msg *models.DIDCommMessage) error {
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockMessageStore) GetPendingMessages(_ context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error) {
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

func (m *mockMessageStore) MarkDelivered(_ context.Context, messageIDs []string) error {
	now := time.Now()
	for _, id := range messageIDs {
		if msg, ok := m.messages[id]; ok && msg.State == "pending" {
			msg.State = "delivered"
			msg.DeliveredAt = &now
		}
	}
	return nil
}

func (m *mockMessageStore) CleanupExpiredMessages(_ context.Context) (int64, error) {
	return 0, nil
}

// ============================================================
// TEST HELPER
// ============================================================

// newTestHandlerWithMessageStore creates a handler with all stores including MessageStore.
func newTestHandlerWithMessageStore(
	es EntityStore,
	kvs KeyVersionStore,
	ots OAuthTokenStore,
	ms MessageStore,
	cfg *config.Config,
) (*Handler, *testFiberApp) {
	_, platformPriv, _ := crypto.GenerateKeyPair()
	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		messageStore:    ms,
		config:          cfg,
		redis:           newTestRedisClient(),
		platformKey:     platformPriv,
	}
	app := newTestFiberAppFromHandler(h)
	return h, app
}

// buildTestJWEForEntity builds a valid JWE addressed to the given entity.
// Returns the raw JWE bytes and the recipient KID used.
// The entity must have X25519PublicKey set.
func buildTestJWEForEntity(t *testing.T, recipientEntity *models.Entity, platformDomain string) ([]byte, string) {
	t.Helper()

	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}

	// Parse recipient X25519 public key from entity bytes
	if len(recipientEntity.X25519PublicKey) == 0 {
		t.Fatalf("entity has no X25519PublicKey set")
	}
	recipientPub, err := ecdh.X25519().NewPublicKey(recipientEntity.X25519PublicKey)
	if err != nil {
		t.Fatalf("parse recipient X25519 public key: %v", err)
	}

	entityID := recipientEntity.ID
	entityType := recipientEntity.Type

	recipientKID := "did:web:" + platformDomain + ":" + entityType + ":" + entityID + "#key-x25519-1"
	senderKID := "did:web:" + platformDomain + ":agent:sender01#key-x25519-1"

	plaintext := []byte(`{"id":"msg_test","type":"https://atap.dev/protocols/basic/1.0/ping","body":{}}`)
	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, recipientPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	return jwe, recipientKID
}

// buildTestJWEWithKID builds a real JWE with a specified recipient KID (domain may differ from entity).
func buildTestJWEWithKID(t *testing.T, recipientKID string) []byte {
	t.Helper()

	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}
	_, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	senderKID := "did:web:atap.app:agent:sender#key-x25519-1"
	plaintext := []byte(`{"id":"msg_test","type":"https://atap.dev/protocols/basic/1.0/ping","body":{}}`)
	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, recipientPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return jwe
}

// ============================================================
// POST /v1/didcomm TESTS
// ============================================================

func TestDIDCommSend(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}

	t.Run("valid JWE returns 202 Accepted", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		_, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), newMockOAuthTokenStore(), ms, cfg)

		// Create a recipient entity with X25519 key
		recipientPriv, recipientPub, _ := didcomm.GenerateX25519KeyPair()
		entityID := "recip01"
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			X25519PublicKey:  recipientPub.Bytes(),
			X25519PrivateKey: recipientPriv.Bytes(),
		}
		es.entities[entityID] = entity

		jwe, _ := buildTestJWEForEntity(t, entity, "atap.app")

		req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
		req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 202 {
			var body map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&body)
			t.Fatalf("status = %d, want 202; body = %v", resp.StatusCode, body)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body["status"] != "queued" {
			t.Errorf("status = %q, want queued", body["status"])
		}
		if _, ok := body["id"].(string); !ok {
			t.Error("response missing string 'id' field")
		}

		// Verify message was queued
		if len(ms.messages) != 1 {
			t.Errorf("expected 1 queued message, got %d", len(ms.messages))
		}
	})

	t.Run("wrong Content-Type returns 415", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		_, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), newMockOAuthTokenStore(), ms, cfg)

		req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader([]byte(`{"test":true}`)))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 415 {
			t.Errorf("status = %d, want 415", resp.StatusCode)
		}
	})

	t.Run("empty body returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		_, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), newMockOAuthTokenStore(), ms, cfg)

		req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("JWE with foreign DID recipient returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		_, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), newMockOAuthTokenStore(), ms, cfg)

		// Build JWE addressed to a foreign domain
		jwe := buildTestJWEWithKID(t, "did:web:evil.com:agent:hacker#key-x25519-1")

		req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
		req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for foreign DID", resp.StatusCode)
		}
	})

	t.Run("JWE with unknown recipient DID returns 400", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		_, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), newMockOAuthTokenStore(), ms, cfg)

		// Build JWE addressed to a valid domain but unknown entity
		jwe := buildTestJWEWithKID(t, "did:web:atap.app:agent:notexist#key-x25519-1")

		req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
		req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for unknown DID", resp.StatusCode)
		}
	})
}

// ============================================================
// GET /v1/didcomm/inbox TESTS
// ============================================================

func TestDIDCommInbox(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}

	t.Run("authenticated entity gets pending messages and marks delivered", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		ots := newMockOAuthTokenStore()
		h, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), ots, ms, cfg)

		// Create entity
		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "inbox01"
		entityDID := "did:web:atap.app:agent:" + entityID
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              entityDID,
			PublicKeyEd25519: entityPub,
		}
		es.entities[entityID] = entity

		// Queue some messages
		msg1 := &models.DIDCommMessage{
			ID:           "msg_inbox_01",
			RecipientDID: entityDID,
			SenderDID:    "did:web:atap.app:agent:sender01",
			MessageType:  "https://atap.dev/protocols/basic/1.0/ping",
			Payload:      []byte(`{"hello":"world"}`),
			State:        "pending",
			CreatedAt:    time.Now().UTC(),
		}
		ms.messages[msg1.ID] = msg1

		// Set up DPoP auth
		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "inbox-test-jti-001"
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:inbox"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:inbox"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/didcomm/inbox")
		req := httptest.NewRequest("GET", "/v1/didcomm/inbox", nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
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

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		msgs, ok := body["messages"].([]interface{})
		if !ok {
			t.Fatalf("body.messages not a list: %T", body["messages"])
		}
		if len(msgs) != 1 {
			t.Errorf("expected 1 message, got %d", len(msgs))
		}

		// Verify message was marked delivered
		if ms.messages["msg_inbox_01"].State != "delivered" {
			t.Error("expected message to be marked delivered after inbox pickup")
		}

		// Verify payload is base64-encoded in the response
		if len(msgs) > 0 {
			m := msgs[0].(map[string]interface{})
			payloadStr, ok := m["payload"].(string)
			if !ok {
				t.Error("message payload should be a base64 string")
			} else {
				decoded, err := base64.StdEncoding.DecodeString(payloadStr)
				if err != nil {
					t.Errorf("payload not valid base64: %v", err)
				}
				if string(decoded) != string(msg1.Payload) {
					t.Errorf("payload = %q, want %q", string(decoded), string(msg1.Payload))
				}
			}
		}
	})

	t.Run("no pending messages returns 200 with empty array", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		ots := newMockOAuthTokenStore()
		h, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), ots, ms, cfg)

		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "inbox02"
		entityDID := "did:web:atap.app:agent:" + entityID
		entity := &models.Entity{
			ID: entityID, Type: models.EntityTypeAgent, DID: entityDID,
			PublicKeyEd25519: entityPub,
		}
		es.entities[entityID] = entity

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "inbox-empty-jti-001"
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:inbox"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:inbox"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/didcomm/inbox")
		req := httptest.NewRequest("GET", "/v1/didcomm/inbox", nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		msgs, ok := body["messages"].([]interface{})
		if !ok {
			// nil/null "messages" field is acceptable for empty
			if body["messages"] == nil {
				return // nil is fine
			}
			t.Fatalf("body.messages not a list: %T", body["messages"])
		}
		if len(msgs) != 0 {
			t.Errorf("expected 0 messages, got %d", len(msgs))
		}
	})

	t.Run("inbox requires atap:inbox scope", func(t *testing.T) {
		es := newMockEntityStore()
		ms := newMockMessageStore()
		ots := newMockOAuthTokenStore()
		h, app := newTestHandlerWithMessageStore(es, newMockKeyVersionStore(), ots, ms, cfg)

		entityPub, _, _ := crypto.GenerateKeyPair()
		entityID := "inbox03"
		entityDID := "did:web:atap.app:agent:" + entityID
		entity := &models.Entity{
			ID: entityID, Type: models.EntityTypeAgent, DID: entityDID,
			PublicKeyEd25519: entityPub,
		}
		es.entities[entityID] = entity

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "inbox-scope-jti-001"
		// Token with wrong scope
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "GET", "https://atap.app/v1/didcomm/inbox")
		req := httptest.NewRequest("GET", "/v1/didcomm/inbox", nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("status = %d, want 403 for missing atap:inbox scope", resp.StatusCode)
		}
	})
}

// ============================================================
// HELPERS FOR REVOKE DIDComm TESTS
// ============================================================

// buildServerHandlerWithRevocationStore creates a Handler with revocationStore and a real
// platform X25519 key so that server-addressed JWEs can be decrypted.
func buildServerHandlerWithRevocationStore(t *testing.T, cfg *config.Config) (
	*Handler, *testFiberApp, *mockEntityStore, *mockMessageStore, *mockRevocationStore, *ecdh.PrivateKey,
) {
	t.Helper()
	es := newMockEntityStore()
	ms := newMockMessageStore()
	rs := newMockRevocationStore()

	_, platformEdPriv, _ := crypto.GenerateKeyPair()
	platformX25519Priv, _, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate platform X25519 key: %v", err)
	}

	h := &Handler{
		entityStore:       es,
		keyVersionStore:   newMockKeyVersionStore(),
		oauthTokenStore:   newMockOAuthTokenStore(),
		messageStore:      ms,
		revocationStore:   rs,
		config:            cfg,
		redis:             newTestRedisClient(),
		platformKey:       platformEdPriv,
		platformX25519Key: platformX25519Priv,
		log:               zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, ms, rs, platformX25519Priv
}

// buildServerAddressedJWE builds a JWE addressed to the server platform DID.
// The approver's identity is the sender. The plaintext is a serialized PlaintextMessage.
// Returns (jweBytes, senderX25519PublicKey) so the caller can register the sender entity.
func buildServerAddressedJWE(t *testing.T, platformDomain string, serverX25519Priv *ecdh.PrivateKey, plaintext []byte, senderDID string) ([]byte, *ecdh.PublicKey) {
	t.Helper()

	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender X25519 keypair: %v", err)
	}

	serverDID := "did:web:" + platformDomain + ":server:platform"
	recipientKID := serverDID + "#key-x25519-1"
	senderKID := senderDID + "#key-x25519-1"

	serverPub := serverX25519Priv.PublicKey()

	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, serverPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt server-addressed JWE: %v", err)
	}
	return jwe, senderPub
}

// ============================================================
// DIDCOMM REVOKE MESSAGE TESTS
// ============================================================

// TestDIDCommRevokeStoresRevocation tests that a TypeApprovalRevoke message addressed to
// the server DID causes a revocation to be stored in the revocation store.
func TestDIDCommRevokeStoresRevocation(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, _, rs, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	// Register the server entity so the DID lookup succeeds
	serverDID := "did:web:atap.app:server:platform"
	serverEntity := &models.Entity{
		ID:               "server_platform",
		Type:             "server",
		DID:              serverDID,
		X25519PublicKey:  serverX25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: serverX25519Priv.Bytes(),
	}
	es.entities["server_platform"] = serverEntity

	approverDID := "did:web:atap.app:human:approver01"
	approvalID := "apr_revoke_test_01"

	body := map[string]any{
		"approval_id":  approvalID,
		"approver_did": approverDID,
	}
	msg := didcomm.NewMessage(didcomm.TypeApprovalRevoke, approverDID, []string{serverDID}, body)
	plaintext, _ := json.Marshal(msg)

	jwe, senderX25519Pub := buildServerAddressedJWE(t, "atap.app", serverX25519Priv, plaintext, approverDID)

	// Register the approver entity so the handler can look up their X25519 public key for ECDH-1PU decryption
	es.entities["approver01"] = &models.Entity{
		ID:              "approver01",
		Type:            models.EntityTypeHuman,
		DID:             approverDID,
		X25519PublicKey: senderX25519Pub.Bytes(),
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		var b map[string]any
		json.NewDecoder(resp.Body).Decode(&b)
		t.Fatalf("status = %d, want 202; body = %v", resp.StatusCode, b)
	}

	// Verify revocation was stored
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if len(rs.revocations) != 1 {
		t.Fatalf("expected 1 revocation in store, got %d", len(rs.revocations))
	}
	for _, rev := range rs.revocations {
		if rev.ApprovalID != approvalID {
			t.Errorf("revocation approval_id = %q, want %q", rev.ApprovalID, approvalID)
		}
		if rev.ApproverDID != approverDID {
			t.Errorf("revocation approver_did = %q, want %q", rev.ApproverDID, approverDID)
		}
	}
}

// TestDIDCommRevokeDefaultExpiresAt tests that a TypeApprovalRevoke without valid_until
// sets expires_at to revoked_at + 60 minutes.
func TestDIDCommRevokeDefaultExpiresAt(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, _, rs, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	serverDID := "did:web:atap.app:server:platform"
	es.entities["server_platform"] = &models.Entity{
		ID:               "server_platform",
		Type:             "server",
		DID:              serverDID,
		X25519PublicKey:  serverX25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: serverX25519Priv.Bytes(),
	}

	approverDID := "did:web:atap.app:human:approver02"
	body := map[string]any{
		"approval_id":  "apr_expires_default",
		"approver_did": approverDID,
		// no valid_until
	}
	msg := didcomm.NewMessage(didcomm.TypeApprovalRevoke, approverDID, []string{serverDID}, body)
	plaintext, _ := json.Marshal(msg)
	jwe, senderX25519Pub := buildServerAddressedJWE(t, "atap.app", serverX25519Priv, plaintext, approverDID)
	es.entities["approver02"] = &models.Entity{
		ID: "approver02", Type: models.EntityTypeHuman, DID: approverDID,
		X25519PublicKey: senderX25519Pub.Bytes(),
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	before := time.Now().UTC()
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()
	if len(rs.revocations) == 0 {
		t.Fatal("expected revocation to be stored")
	}
	for _, rev := range rs.revocations {
		expectedMinExpiry := before.Add(55 * time.Minute) // allow 5 min slack
		if rev.ExpiresAt.Before(expectedMinExpiry) {
			t.Errorf("expires_at %v is less than expected min %v (want ~revoked_at+60min)", rev.ExpiresAt, expectedMinExpiry)
		}
	}
}

// TestDIDCommRevokeWithValidUntil tests that a TypeApprovalRevoke with valid_until
// uses that value for expires_at.
func TestDIDCommRevokeWithValidUntil(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, _, rs, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	serverDID := "did:web:atap.app:server:platform"
	es.entities["server_platform"] = &models.Entity{
		ID:               "server_platform",
		Type:             "server",
		DID:              serverDID,
		X25519PublicKey:  serverX25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: serverX25519Priv.Bytes(),
	}

	approverDID := "did:web:atap.app:human:approver03"
	validUntil := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"approval_id":  "apr_valid_until_test",
		"approver_did": approverDID,
		"valid_until":  validUntil,
	}
	msg := didcomm.NewMessage(didcomm.TypeApprovalRevoke, approverDID, []string{serverDID}, body)
	plaintext, _ := json.Marshal(msg)
	jwe, senderX25519Pub := buildServerAddressedJWE(t, "atap.app", serverX25519Priv, plaintext, approverDID)
	es.entities["approver03"] = &models.Entity{
		ID: "approver03", Type: models.EntityTypeHuman, DID: approverDID,
		X25519PublicKey: senderX25519Pub.Bytes(),
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()
	if len(rs.revocations) == 0 {
		t.Fatal("expected revocation to be stored")
	}
	for _, rev := range rs.revocations {
		// ExpiresAt should be approximately 24h from now
		minExpiry := time.Now().UTC().Add(23 * time.Hour)
		if rev.ExpiresAt.Before(minExpiry) {
			t.Errorf("expires_at %v should be ~24h from now, got less than 23h", rev.ExpiresAt)
		}
	}
}

// TestDIDCommRevokeSkipsForwardingWithNoVia tests that a TypeApprovalRevoke without
// a via field does NOT dispatch any additional DIDComm message (two-party case).
func TestDIDCommRevokeSkipsForwardingWithNoVia(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, ms, rs, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	serverDID := "did:web:atap.app:server:platform"
	es.entities["server_platform"] = &models.Entity{
		ID:               "server_platform",
		Type:             "server",
		DID:              serverDID,
		X25519PublicKey:  serverX25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: serverX25519Priv.Bytes(),
	}

	approverDID := "did:web:atap.app:human:approver04"
	body := map[string]any{
		"approval_id":  "apr_no_via_test",
		"approver_did": approverDID,
		// no via field
	}
	msg := didcomm.NewMessage(didcomm.TypeApprovalRevoke, approverDID, []string{serverDID}, body)
	plaintext, _ := json.Marshal(msg)
	jwe, senderX25519Pub := buildServerAddressedJWE(t, "atap.app", serverX25519Priv, plaintext, approverDID)
	es.entities["approver04"] = &models.Entity{
		ID: "approver04", Type: models.EntityTypeHuman, DID: approverDID,
		X25519PublicKey: senderX25519Pub.Bytes(),
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	// Revocation should be stored
	rs.mu.Lock()
	revCount := len(rs.revocations)
	rs.mu.Unlock()
	if revCount != 1 {
		t.Errorf("expected 1 revocation, got %d", revCount)
	}

	// No additional DIDComm messages should have been dispatched
	if len(ms.messages) != 0 {
		t.Errorf("expected 0 forwarded messages (no via), got %d", len(ms.messages))
	}
}

// TestDIDCommRevokeForwardsToVia tests that a TypeApprovalRevoke with a via field
// dispatches a forward DIDComm message to that via DID.
func TestDIDCommRevokeForwardsToVia(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, ms, rs, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	serverDID := "did:web:atap.app:server:platform"
	es.entities["server_platform"] = &models.Entity{
		ID:               "server_platform",
		Type:             "server",
		DID:              serverDID,
		X25519PublicKey:  serverX25519Priv.PublicKey().Bytes(),
		X25519PrivateKey: serverX25519Priv.Bytes(),
	}

	approverDID := "did:web:atap.app:human:approver05"
	viaDID := "did:web:atap.app:org:viasystem01"

	body := map[string]any{
		"approval_id":  "apr_via_forward_test",
		"approver_did": approverDID,
		"via":          viaDID,
	}
	msg := didcomm.NewMessage(didcomm.TypeApprovalRevoke, approverDID, []string{serverDID}, body)
	plaintext, _ := json.Marshal(msg)
	jwe, senderX25519Pub := buildServerAddressedJWE(t, "atap.app", serverX25519Priv, plaintext, approverDID)
	es.entities["approver05"] = &models.Entity{
		ID: "approver05", Type: models.EntityTypeHuman, DID: approverDID,
		X25519PublicKey: senderX25519Pub.Bytes(),
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	// Revocation should be stored
	rs.mu.Lock()
	revCount := len(rs.revocations)
	rs.mu.Unlock()
	if revCount != 1 {
		t.Errorf("expected 1 revocation, got %d", revCount)
	}

	// A forward message to via should have been queued
	if len(ms.messages) != 1 {
		t.Errorf("expected 1 forwarded message (to via), got %d", len(ms.messages))
	}
	for _, qmsg := range ms.messages {
		if qmsg.RecipientDID != viaDID {
			t.Errorf("forwarded message recipient = %q, want %q", qmsg.RecipientDID, viaDID)
		}
		if qmsg.MessageType != didcomm.TypeApprovalRevoke {
			t.Errorf("forwarded message type = %q, want %q", qmsg.MessageType, didcomm.TypeApprovalRevoke)
		}
	}
}

// TestDIDCommRejectedPassthrough tests that TypeApprovalRejected messages addressed
// to a non-server entity are queued unchanged (APR-08 regression test).
func TestDIDCommRejectedPassthrough(t *testing.T) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, app, es, ms, _, serverX25519Priv := buildServerHandlerWithRevocationStore(t, cfg)

	// Register a recipient entity (NOT the server DID)
	recipientPriv, recipientPub, _ := didcomm.GenerateX25519KeyPair()
	entityID := "rejected-recipient-01"
	recipientEntity := &models.Entity{
		ID:               entityID,
		Type:             models.EntityTypeAgent,
		DID:              "did:web:atap.app:agent:" + entityID,
		X25519PublicKey:  recipientPub.Bytes(),
		X25519PrivateKey: recipientPriv.Bytes(),
	}
	es.entities[entityID] = recipientEntity

	// Build a JWE addressed to the recipient entity (NOT the server)
	_ = serverX25519Priv // not used for this test — recipient is a non-server entity
	senderPriv, senderPub, _ := didcomm.GenerateX25519KeyPair()
	approverDID := "did:web:atap.app:human:approver-rejected"
	recipientKID := "did:web:atap.app:agent:" + entityID + "#key-x25519-1"
	senderKID := approverDID + "#key-x25519-1"

	msgBody := map[string]any{
		"approval_id": "apr_rejected_01",
		"reason":      "declined by user",
	}
	pmsg := didcomm.NewMessage(didcomm.TypeApprovalRejected, approverDID, []string{recipientEntity.DID}, msgBody)
	plaintext, _ := json.Marshal(pmsg)

	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, recipientPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/didcomm", bytes.NewReader(jwe))
	req.Header.Set("Content-Type", didcomm.ContentTypeEncrypted)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 202 {
		t.Fatalf("status = %d, want 202 for passthrough", resp.StatusCode)
	}

	// The message should be queued for the recipient entity (passthrough)
	if len(ms.messages) != 1 {
		t.Errorf("expected 1 queued message (passthrough), got %d", len(ms.messages))
	}
	for _, qmsg := range ms.messages {
		if qmsg.RecipientDID != recipientEntity.DID {
			t.Errorf("queued message recipient = %q, want %q", qmsg.RecipientDID, recipientEntity.DID)
		}
	}
}
