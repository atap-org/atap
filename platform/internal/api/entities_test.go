package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

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

// newEntityTestApp creates a test Fiber app with entity handlers and mock stores.
func newEntityTestApp(es *mockEntityStore, kvs *mockKeyVersionStore) (EntityStore, *mockEntityStore, *mockKeyVersionStore, *config.Config, *Handler) {
	cfg := &config.Config{PlatformDomain: "atap.app"}
	h, _ := newTestHandlerWithStores(es, kvs, cfg)
	return es, es, kvs, cfg, h
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
			name: "missing public_key returns 400",
			body: map[string]interface{}{
				"type": "agent",
			},
			wantStatus: 400,
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
			name: "agent without principal_did returns 400",
			body: map[string]interface{}{
				"type":       "agent",
				"public_key": crypto.EncodePublicKey(missingPrincipalPub),
			},
			wantStatus: 400,
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
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		pub, _, _ := crypto.GenerateKeyPair()
		entity := &models.Entity{
			ID:               "del01id",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:del01id",
			PublicKeyEd25519: pub,
		}
		es.entities["del01id"] = entity

		req := httptest.NewRequest("DELETE", "/v1/entities/del01id", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			t.Errorf("status = %d, want 204", resp.StatusCode)
		}

		// Verify entity was removed from store
		if _, ok := es.entities["del01id"]; ok {
			t.Error("entity still exists after DELETE")
		}
	})

	t.Run("nonexistent entity returns 404", func(t *testing.T) {
		es := newMockEntityStore()
		kvs := newMockKeyVersionStore()
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		req := httptest.NewRequest("DELETE", "/v1/entities/nonexistent", nil)
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
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		// Create entity with initial key version
		oldPub, _, _ := crypto.GenerateKeyPair()
		entity := &models.Entity{
			ID:               "rot01id",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:rot01id",
			PublicKeyEd25519: oldPub,
		}
		es.entities["rot01id"] = entity
		kvs.versions["rot01id"] = []models.KeyVersion{
			{ID: "kv1", EntityID: "rot01id", PublicKey: oldPub, KeyIndex: 1},
		}

		// Rotate to new key
		newPub, _, _ := crypto.GenerateKeyPair()
		body, _ := json.Marshal(map[string]interface{}{
			"public_key": crypto.EncodePublicKey(newPub),
		})
		req := httptest.NewRequest("POST", "/v1/entities/rot01id/keys/rotate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		// After rotation: 2 key versions, old has valid_until, new is active
		versions, _ := kvs.GetKeyVersions(context.Background(), "rot01id")
		if len(versions) != 2 {
			t.Errorf("versions count = %d, want 2 after rotation", len(versions))
		}

		active, _ := kvs.GetActiveKeyVersion(context.Background(), "rot01id")
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
		cfg := &config.Config{PlatformDomain: "atap.app"}
		_, app := newTestHandlerWithStores(es, kvs, cfg)

		newPub, _, _ := crypto.GenerateKeyPair()
		body, _ := json.Marshal(map[string]interface{}{
			"public_key": crypto.EncodePublicKey(newPub),
		})
		req := httptest.NewRequest("POST", "/v1/entities/nonexistent/keys/rotate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

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
