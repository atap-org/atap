package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK STORES
// ============================================================

type mockEntityStore struct {
	entities map[string]*models.Entity
}

func newMockEntityStore() *mockEntityStore {
	return &mockEntityStore{entities: make(map[string]*models.Entity)}
}

func (m *mockEntityStore) CreateEntity(_ context.Context, e *models.Entity) error {
	m.entities[e.ID] = e
	return nil
}

func (m *mockEntityStore) GetEntity(_ context.Context, id string) (*models.Entity, error) {
	e, ok := m.entities[id]
	if !ok {
		return nil, nil
	}
	return e, nil
}

func (m *mockEntityStore) GetEntityByKeyID(_ context.Context, keyID string) (*models.Entity, error) {
	for _, e := range m.entities {
		if e.KeyID == keyID {
			return e, nil
		}
	}
	return nil, nil
}

func (m *mockEntityStore) GetEntityByDID(_ context.Context, did string) (*models.Entity, error) {
	for _, e := range m.entities {
		if e.DID == did {
			return e, nil
		}
	}
	return nil, nil
}

func (m *mockEntityStore) DeleteEntity(_ context.Context, id string) error {
	delete(m.entities, id)
	return nil
}

type mockKeyVersionStore struct {
	versions map[string][]models.KeyVersion
}

func newMockKeyVersionStore() *mockKeyVersionStore {
	return &mockKeyVersionStore{versions: make(map[string][]models.KeyVersion)}
}

func (m *mockKeyVersionStore) CreateKeyVersion(_ context.Context, kv *models.KeyVersion) error {
	m.versions[kv.EntityID] = append(m.versions[kv.EntityID], *kv)
	return nil
}

func (m *mockKeyVersionStore) GetActiveKeyVersion(_ context.Context, entityID string) (*models.KeyVersion, error) {
	for _, kv := range m.versions[entityID] {
		if kv.ValidUntil == nil {
			return &kv, nil
		}
	}
	return nil, nil
}

func (m *mockKeyVersionStore) GetKeyVersions(_ context.Context, entityID string) ([]models.KeyVersion, error) {
	return m.versions[entityID], nil
}

func (m *mockKeyVersionStore) RotateKey(_ context.Context, entityID string, newPubKey []byte) (*models.KeyVersion, error) {
	now := time.Now().UTC()
	versions := m.versions[entityID]
	for i := range versions {
		if versions[i].ValidUntil == nil {
			versions[i].ValidUntil = &now
		}
	}
	m.versions[entityID] = versions

	maxIndex := 0
	for _, kv := range versions {
		if kv.KeyIndex > maxIndex {
			maxIndex = kv.KeyIndex
		}
	}

	newKV := models.KeyVersion{
		ID:        "kv-new",
		EntityID:  entityID,
		PublicKey: newPubKey,
		KeyIndex:  maxIndex + 1,
		ValidFrom: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
	m.versions[entityID] = append(m.versions[entityID], newKV)
	return &newKV, nil
}

// ============================================================
// CREATE ENTITY TESTS
// ============================================================

func TestCreateEntity(t *testing.T) {
	agentPub, _, _ := crypto.GenerateKeyPair()
	humanPub, _, _ := crypto.GenerateKeyPair()
	machinePub, _, _ := crypto.GenerateKeyPair()
	invalidPub, _, _ := crypto.GenerateKeyPair()
	missingPrincipalPub, _, _ := crypto.GenerateKeyPair()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		checkResp  func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "create agent with public key",
			body: map[string]interface{}{
				"type":          "agent",
				"name":          "Test Agent",
				"public_key":    crypto.EncodePublicKey(agentPub),
				"principal_did": "did:web:atap.app:human:kzdvvj2umnduyauf",
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, body map[string]interface{}) {
				did, ok := body["did"].(string)
				if !ok || did == "" {
					t.Error("response missing 'did' field")
				}
				if ok && len(did) >= 8 && did[:8] != "did:web:" {
					t.Errorf("did = %q, want did:web: prefix", did)
				}
				if body["type"] != "agent" {
					t.Errorf("type = %q, want 'agent'", body["type"])
				}
				// Agent should receive client_secret
				if _, ok := body["client_secret"]; !ok {
					t.Error("agent response missing 'client_secret'")
				}
			},
		},
		{
			name: "create human with public key gets DID with derived ID",
			body: map[string]interface{}{
				"type":       "human",
				"public_key": crypto.EncodePublicKey(humanPub),
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, body map[string]interface{}) {
				did, ok := body["did"].(string)
				if !ok || did == "" {
					t.Error("response missing 'did' field")
				}
				if body["type"] != "human" {
					t.Errorf("type = %q, want 'human'", body["type"])
				}
				// Human DID should contain the derived ID
				humanID := crypto.DeriveHumanID(humanPub)
				if ok && !containsSubstring(did, humanID) {
					t.Errorf("human DID %q should contain derived ID %q", did, humanID)
				}
			},
		},
		{
			name: "create machine entity",
			body: map[string]interface{}{
				"type":       "machine",
				"public_key": crypto.EncodePublicKey(machinePub),
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, body map[string]interface{}) {
				if body["type"] != "machine" {
					t.Errorf("type = %q, want 'machine'", body["type"])
				}
				// Machine should receive client_secret
				if _, ok := body["client_secret"]; !ok {
					t.Error("machine response missing 'client_secret'")
				}
			},
		},
		{
			name: "server generates keypair when public_key omitted",
			body: map[string]interface{}{
				"type":          "agent",
				"name":          "Auto-Key Agent",
				"principal_did": "did:web:atap.app:human:kzdvvj2umnduyauf",
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, body map[string]interface{}) {
				// Should return a private_key (base64 Ed25519 seed)
				pk, ok := body["private_key"].(string)
				if !ok || pk == "" {
					t.Error("response missing 'private_key' when public_key omitted")
				}
				// Should also return client_secret for agent
				if _, ok := body["client_secret"]; !ok {
					t.Error("agent response missing 'client_secret'")
				}
				// Should have a valid DID
				did, ok := body["did"].(string)
				if !ok || did == "" {
					t.Error("response missing 'did' field")
				}
			},
		},
		{
			name: "invalid type returns 400",
			body: map[string]interface{}{
				"type":       "invalid",
				"public_key": crypto.EncodePublicKey(invalidPub),
			},
			wantStatus: 400,
		},
		{
			name: "agent without principal_did succeeds (autonomous agent)",
			body: map[string]interface{}{
				"type":       "agent",
				"public_key": crypto.EncodePublicKey(missingPrincipalPub),
			},
			wantStatus: 201,
			checkResp: func(t *testing.T, body map[string]interface{}) {
				if body["type"] != "agent" {
					t.Errorf("type = %q, want 'agent'", body["type"])
				}
				if _, ok := body["client_secret"]; !ok {
					t.Error("agent response missing 'client_secret'")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			es := newMockEntityStore()
			kvs := newMockKeyVersionStore()
			cfg := &config.Config{PlatformDomain: "atap.app"}
			_, app := newTestHandlerWithStores(es, kvs, cfg)

			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest("POST", "/v1/entities", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			if tc.checkResp != nil && resp.StatusCode == tc.wantStatus {
				var respBody map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tc.checkResp(t, respBody)
			}
		})
	}
}

func TestGetEntity(t *testing.T) {
	t.Run("existing entity returns 200 with DID", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		pub, _, _ := crypto.GenerateKeyPair()
		entity := &models.Entity{
			ID:               "test01id",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:test01id",
			PublicKeyEd25519: pub,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		es.entities["test01id"] = entity

		req := httptest.NewRequest("GET", "/v1/entities/test01id", nil)
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

		if body["did"] != "did:web:atap.app:agent:test01id" {
			t.Errorf("did = %q, want 'did:web:atap.app:agent:test01id'", body["did"])
		}
	})

	t.Run("nonexistent entity returns 404", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		req := httptest.NewRequest("GET", "/v1/entities/nonexistent", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestDeleteEntity(t *testing.T) {
	t.Run("existing entity returns 204", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		pub, _, _ := crypto.GenerateKeyPair()
		entityID := "del01id"
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: pub,
		}
		es.entities[entityID] = entity

		// Set up DPoP auth
		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "del-test-jti-001"
		tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
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

		if resp.StatusCode != 204 {
			t.Errorf("status = %d, want 204", resp.StatusCode)
		}

		// Verify entity was removed from store
		if _, ok := es.entities[entityID]; ok {
			t.Error("entity still exists after DELETE")
		}
	})

	t.Run("nonexistent entity returns 404", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		// Create an entity to own the token (the non-existent entity is the target, not the caller)
		callerPub, _, _ := crypto.GenerateKeyPair()
		callerID := "del-caller-001"
		caller := &models.Entity{
			ID: callerID, Type: models.EntityTypeAgent,
			DID: "did:web:atap.app:agent:" + callerID, PublicKeyEd25519: callerPub,
		}
		es.entities[callerID] = caller

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "del-nonexistent-jti-001"
		tokenStr := issueTestToken(t, h, caller.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: callerID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/nonexistent")
		req := httptest.NewRequest("DELETE", "/v1/entities/nonexistent", nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

func TestCreateEntityStorePrincipal(t *testing.T) {
	t.Run("agent stores principal_did", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		pub, _, _ := crypto.GenerateKeyPair()
		principalDID := "did:web:atap.app:human:kzdvvj2umnduyauf"

		body, _ := json.Marshal(map[string]interface{}{
			"type":          "agent",
			"public_key":    crypto.EncodePublicKey(pub),
			"principal_did": principalDID,
		})
		req := httptest.NewRequest("POST", "/v1/entities", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}

		var respBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respBody)

		// Find the entity in the store and check principal_did
		var found *models.Entity
		for _, e := range es.entities {
			found = e
			break
		}
		if found == nil {
			t.Fatal("entity not found in store")
		}
		if found.PrincipalDID != principalDID {
			t.Errorf("stored principal_did = %q, want %q", found.PrincipalDID, principalDID)
		}
	})
}

func TestCreateEntityKeyVersion(t *testing.T) {
	t.Run("registration creates initial key version", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		pub, _, _ := crypto.GenerateKeyPair()
		body, _ := json.Marshal(map[string]interface{}{
			"type":          "agent",
			"public_key":    crypto.EncodePublicKey(pub),
			"principal_did": "did:web:atap.app:human:kzdvvj2umnduyauf",
		})
		req := httptest.NewRequest("POST", "/v1/entities", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}

		// Find entity ID from response
		var respBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respBody)
		entityID, ok := respBody["id"].(string)
		if !ok || entityID == "" {
			t.Fatal("response missing 'id' field")
		}

		// Check key version was created
		versions, _ := kvs.GetKeyVersions(context.Background(), entityID)
		if len(versions) != 1 {
			t.Errorf("key versions count = %d, want 1", len(versions))
		}
		if len(versions) > 0 && versions[0].KeyIndex != 1 {
			t.Errorf("key_index = %d, want 1", versions[0].KeyIndex)
		}
	})
}

func TestRotateKey(t *testing.T) {
	t.Run("rotate key returns 200 with new key version", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		// Create entity with initial key version
		oldPub, _, _ := crypto.GenerateKeyPair()
		entityID := "rot01id"
		entity := &models.Entity{
			ID:               entityID,
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:" + entityID,
			PublicKeyEd25519: oldPub,
		}
		es.entities[entityID] = entity
		kvs.versions[entityID] = []models.KeyVersion{
			{ID: "kv1", EntityID: entityID, PublicKey: oldPub, KeyIndex: 1},
		}

		// Set up DPoP auth
		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "rot-test-jti-001"
		tokenStr := issueTestToken(t, h, entity.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		// Rotate to new key
		newPub, _, _ := crypto.GenerateKeyPair()
		body, _ := json.Marshal(map[string]interface{}{
			"public_key": crypto.EncodePublicKey(newPub),
		})
		rotateURL := "/v1/entities/" + entityID + "/keys/rotate"
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app"+rotateURL)
		req := httptest.NewRequest("POST", rotateURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

		// After rotation: 2 key versions, old has valid_until, new is active
		versions, _ := kvs.GetKeyVersions(context.Background(), entityID)
		if len(versions) != 2 {
			t.Errorf("versions count = %d, want 2 after rotation", len(versions))
		}

		active, _ := kvs.GetActiveKeyVersion(context.Background(), entityID)
		if active == nil {
			t.Fatal("expected active key after rotation")
		}
		if active.KeyIndex != 2 {
			t.Errorf("active key_index = %d, want 2", active.KeyIndex)
		}
	})

	t.Run("rotate key for nonexistent entity returns 404", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		ots := newMockOAuthTokenStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		h, app := newTestHandlerFull(es, kvs, ots, cfg)

		// Create caller entity for the token
		callerPub, _, _ := crypto.GenerateKeyPair()
		callerID := "rot-caller-001"
		caller := &models.Entity{
			ID: callerID, Type: models.EntityTypeAgent,
			DID: "did:web:atap.app:agent:" + callerID, PublicKeyEd25519: callerPub,
		}
		es.entities[callerID] = caller

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "rot-nonexist-jti-001"
		tokenStr := issueTestToken(t, h, caller.DID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: callerID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		newPub, _, _ := crypto.GenerateKeyPair()
		body, _ := json.Marshal(map[string]interface{}{
			"public_key": crypto.EncodePublicKey(newPub),
		})
		rotateURL := "/v1/entities/nonexistent/keys/rotate"
		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "POST", "https://atap.app"+rotateURL)
		req := httptest.NewRequest("POST", rotateURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// ============================================================
// TESTS: DeleteEntity crypto-shredding (Task 2 - PRV-03)
// ============================================================

// newTestHandlerWithCryptoShred creates a Handler with entity, credential, and message stores.
func newTestHandlerWithCryptoShred(t *testing.T) (
	*Handler,
	*testFiberApp,
	*mockEntityStore,
	*mockCredentialStore,
	*mockMessageStore,
	*mockOAuthTokenStore,
) {
	t.Helper()
	es := newMockEntityStore()
	kvs := newMockKeyVersionStore()
	ots := newMockOAuthTokenStore()
	cs := newMockCredentialStore()
	ms := newMockMessageStore()
	cfg := &config.Config{PlatformDomain: "atap.app"}
	_, platformPriv, _ := crypto.GenerateKeyPair()
	rdb := newTestRedisClient()

	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		oauthTokenStore: ots,
		credentialStore: cs,
		messageStore:    ms,
		config:          cfg,
		redis:           rdb,
		platformKey:     platformPriv,
		log:             zerolog.Nop(),
	}
	app := newTestFiberAppFromHandler(h)
	return h, app, es, cs, ms, ots
}

// TestDeleteEntityCryptoShred verifies the full crypto-shred behavior.
func TestDeleteEntityCryptoShred(t *testing.T) {
	t.Run("delete entity deletes enc key before entity", func(t *testing.T) {
		h, app, es, cs, _, ots := newTestHandlerWithCryptoShred(t)

		pub, _, _ := crypto.GenerateKeyPair()
		entityID := "cryptoshred-01"
		entityDID := "did:web:atap.app:agent:" + entityID
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeAgent,
			DID: entityDID, PublicKeyEd25519: pub,
		}

		// Pre-seed an enc key for the entity
		cs.encKeys[entityID] = []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0") // 32 bytes

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "cryptoshred-jti-01"
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// Entity should be gone
		if _, ok := es.entities[entityID]; ok {
			t.Error("entity still exists after DELETE")
		}

		// Enc key should be deleted (crypto-shredded)
		ctx := context.Background()
		key, _ := cs.GetEncKey(ctx, entityID)
		if key != nil {
			t.Error("enc key still exists after DELETE (crypto-shred failed)")
		}
	})

	t.Run("delete entity with no enc key still succeeds", func(t *testing.T) {
		h, app, es, _, _, ots := newTestHandlerWithCryptoShred(t)

		pub, _, _ := crypto.GenerateKeyPair()
		entityID := "cryptoshred-nokey-01"
		entityDID := "did:web:atap.app:agent:" + entityID
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeAgent,
			DID: entityDID, PublicKeyEd25519: pub,
		}
		// No enc key for this entity (never issued credentials)

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "cryptoshred-nokey-jti-01"
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			t.Fatalf("expected 204, got %d (entity without enc key should still delete)", resp.StatusCode)
		}
	})

	t.Run("delete entity queues DIDComm shredded notification", func(t *testing.T) {
		h, app, es, _, ms, ots := newTestHandlerWithCryptoShred(t)

		pub, _, _ := crypto.GenerateKeyPair()
		entityID := "cryptoshred-msg-01"
		entityDID := "did:web:atap.app:agent:" + entityID
		es.entities[entityID] = &models.Entity{
			ID: entityID, Type: models.EntityTypeAgent,
			DID: entityDID, PublicKeyEd25519: pub,
		}

		dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
		jkt := computeTestJWKThumbprint(t, dpopPub)
		jti := "cryptoshred-msg-jti-01"
		tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:manage"}, time.Hour)
		ots.tokens[jti] = &models.OAuthToken{
			ID: jti, EntityID: entityID, TokenType: "access",
			Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
		}

		dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
		req := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
		req.Header.Set("Authorization", "DPoP "+tokenStr)
		req.Header.Set("DPoP", dpopProof)

		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// A DIDComm shredded message should have been queued
		if len(ms.messages) == 0 {
			t.Error("expected DIDComm shredded notification to be queued, got none")
		}
		// Check message type
		for _, msg := range ms.messages {
			if msg.MessageType != "https://atap.dev/protocols/entity/1.0/shredded" {
				t.Errorf("message type = %q, want entity/1.0/shredded", msg.MessageType)
			}
			break
		}
	})
}

// TestDeleteEntityDIDDocDeactivation verifies PRV-03: DID Doc returns 410 Gone after entity deletion.
func TestDeleteEntityDIDDocDeactivation(t *testing.T) {
	// Step 1: Create entity
	h, app, es, _, _, ots := newTestHandlerWithCryptoShred(t)

	pub, _, _ := crypto.GenerateKeyPair()
	entityID := "deactivate-did-01"
	entityDID := "did:web:atap.app:agent:" + entityID
	es.entities[entityID] = &models.Entity{
		ID: entityID, Type: models.EntityTypeAgent,
		DID: entityDID, PublicKeyEd25519: pub,
	}
	es.entities[entityID].KeyID = crypto.NewKeyID("agt")
	kvs := newMockKeyVersionStore()
	kvs.versions[entityID] = []models.KeyVersion{
		{ID: "kv1", EntityID: entityID, PublicKey: pub, KeyIndex: 1},
	}
	h.keyVersionStore = kvs

	dpopPub, dpopPriv, _ := crypto.GenerateKeyPair()
	jkt := computeTestJWKThumbprint(t, dpopPub)
	jti := "deactivate-jti-01"
	tokenStr := issueTestToken(t, h, entityDID, jti, jkt, []string{"atap:manage"}, time.Hour)
	ots.tokens[jti] = &models.OAuthToken{
		ID: jti, EntityID: entityID, TokenType: "access",
		Scope: []string{"atap:manage"}, DPoPJKT: jkt, ExpiresAt: time.Now().Add(time.Hour),
	}

	// Step 2: Resolve DID Document (should return 200)
	didReq := httptest.NewRequest("GET", "/agent/"+entityID+"/did.json", nil)
	didResp, err := app.Test(didReq)
	if err != nil {
		t.Fatalf("app.Test DID resolve: %v", err)
	}
	didResp.Body.Close()
	if didResp.StatusCode != 200 {
		t.Fatalf("expected 200 before delete, got %d", didResp.StatusCode)
	}

	// Step 3: Delete the entity
	dpopProof := generateDPoPProof(t, dpopPriv, dpopPub, "DELETE", "https://atap.app/v1/entities/"+entityID)
	delReq := httptest.NewRequest("DELETE", "/v1/entities/"+entityID, nil)
	delReq.Header.Set("Authorization", "DPoP "+tokenStr)
	delReq.Header.Set("DPoP", dpopProof)

	delResp, err := app.Test(delReq, 5000)
	if err != nil {
		t.Fatalf("app.Test DELETE: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	// Step 4: Resolve DID Document again — must return 410 Gone (not 404)
	didReq2 := httptest.NewRequest("GET", "/agent/"+entityID+"/did.json", nil)
	didResp2, err := app.Test(didReq2)
	if err != nil {
		t.Fatalf("app.Test DID resolve after delete: %v", err)
	}
	defer didResp2.Body.Close()

	if didResp2.StatusCode != 410 {
		t.Errorf("expected 410 Gone after entity deletion (PRV-03), got %d", didResp2.StatusCode)
	}

	// Step 5: Verify body contains deactivated: true
	var body map[string]any
	if err := json.NewDecoder(didResp2.Body).Decode(&body); err != nil {
		t.Fatalf("decode 410 body: %v", err)
	}
	if body["deactivated"] != true {
		t.Errorf("expected deactivated=true in 410 body, got %v", body["deactivated"])
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
