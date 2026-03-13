package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/config"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

func TestResolveDID(t *testing.T) {
	// Set up shared test data
	pub, _, _ := crypto.GenerateKeyPair()
	agentEntityID := "testagent01"
	agentDID := "did:web:atap.app:agent:" + agentEntityID
	principalDID := "did:web:atap.app:human:kzdvvj2umnduyauf"

	humanEntityID := crypto.DeriveHumanID(pub)
	humanDID := "did:web:atap.app:human:" + humanEntityID

	tests := []struct {
		name        string
		path        string
		setup       func(es *mockEntityStore, kvs *mockKeyVersionStore)
		wantStatus  int
		wantCT      string
		checkResp   func(t *testing.T, doc map[string]interface{})
	}{
		{
			name: "agent DID document returns 200 with correct content type",
			path: "/agent/" + agentEntityID + "/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				es.entities[agentEntityID] = &models.Entity{
					ID:               agentEntityID,
					Type:             models.EntityTypeAgent,
					DID:              agentDID,
					PrincipalDID:     principalDID,
					PublicKeyEd25519: pub,
				}
				kvs.versions[agentEntityID] = []models.KeyVersion{
					{ID: "kv1", EntityID: agentEntityID, PublicKey: pub, KeyIndex: 1},
				}
			},
			wantStatus: 200,
			wantCT:     "application/did+ld+json",
			checkResp: func(t *testing.T, doc map[string]interface{}) {
				// Check @context is a 3-element array
				ctx, ok := doc["@context"].([]interface{})
				if !ok {
					t.Errorf("@context is not an array: %T", doc["@context"])
					return
				}
				if len(ctx) != 3 {
					t.Errorf("@context length = %d, want 3", len(ctx))
				}
				if len(ctx) >= 1 && ctx[0] != "https://www.w3.org/ns/did/v1" {
					t.Errorf("context[0] = %q, want W3C DID context", ctx[0])
				}
				if len(ctx) >= 2 && ctx[1] != "https://w3id.org/security/suites/ed25519-2020/v1" {
					t.Errorf("context[1] = %q, want ed25519-2020 context", ctx[1])
				}
				if len(ctx) >= 3 && ctx[2] != "https://atap.dev/ns/v1" {
					t.Errorf("context[2] = %q, want ATAP context", ctx[2])
				}

				// Check id
				if doc["id"] != agentDID {
					t.Errorf("id = %q, want %q", doc["id"], agentDID)
				}

				// Check verificationMethod
				vms, ok := doc["verificationMethod"].([]interface{})
				if !ok || len(vms) == 0 {
					t.Error("verificationMethod missing or empty")
					return
				}
				vm := vms[0].(map[string]interface{})
				if vm["type"] != "Ed25519VerificationKey2020" {
					t.Errorf("verificationMethod[0].type = %q, want Ed25519VerificationKey2020", vm["type"])
				}
				pkm, ok := vm["publicKeyMultibase"].(string)
				if !ok || !strings.HasPrefix(pkm, "z") {
					t.Errorf("publicKeyMultibase = %q, want 'z' prefix", pkm)
				}

				// Check authentication and assertionMethod reference the key
				auth, ok := doc["authentication"].([]interface{})
				if !ok || len(auth) == 0 {
					t.Error("authentication missing or empty")
				}
				asMeth, ok := doc["assertionMethod"].([]interface{})
				if !ok || len(asMeth) == 0 {
					t.Error("assertionMethod missing or empty")
				}

				// Agent-specific fields
				if doc["atap:type"] != "agent" {
					t.Errorf("atap:type = %q, want 'agent'", doc["atap:type"])
				}
				if doc["atap:principal"] != principalDID {
					t.Errorf("atap:principal = %q, want %q", doc["atap:principal"], principalDID)
				}
			},
		},
		{
			name: "human DID document has no atap:principal",
			path: "/human/" + humanEntityID + "/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				es.entities[humanEntityID] = &models.Entity{
					ID:               humanEntityID,
					Type:             models.EntityTypeHuman,
					DID:              humanDID,
					PublicKeyEd25519: pub,
				}
				kvs.versions[humanEntityID] = []models.KeyVersion{
					{ID: "kv1", EntityID: humanEntityID, PublicKey: pub, KeyIndex: 1},
				}
			},
			wantStatus: 200,
			wantCT:     "application/did+ld+json",
			checkResp: func(t *testing.T, doc map[string]interface{}) {
				if _, hasPrincipal := doc["atap:principal"]; hasPrincipal {
					t.Error("human DID document should not have atap:principal")
				}
				if doc["atap:type"] != "human" {
					t.Errorf("atap:type = %q, want 'human'", doc["atap:type"])
				}
			},
		},
		{
			name: "invalid type returns 404",
			path: "/notatype/someid/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				// no setup needed
			},
			wantStatus: 404,
		},
		{
			name: "nonexistent entity returns 404",
			path: "/agent/nonexistent/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				// no entity in store
			},
			wantStatus: 404,
		},
		{
			name: "type mismatch returns 404 (agent id looked up as human)",
			path: "/human/" + agentEntityID + "/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				// Entity exists but is type=agent, not human
				es.entities[agentEntityID] = &models.Entity{
					ID:   agentEntityID,
					Type: models.EntityTypeAgent,
					DID:  agentDID,
				}
			},
			wantStatus: 404,
		},
		{
			name: "DID document with rotated keys includes all key versions",
			path: "/agent/" + agentEntityID + "/did.json",
			setup: func(es *mockEntityStore, kvs *mockKeyVersionStore) {
				pub2, _, _ := crypto.GenerateKeyPair()
				expiredAt := time.Now().UTC().Add(-time.Hour)
				es.entities[agentEntityID] = &models.Entity{
					ID:               agentEntityID,
					Type:             models.EntityTypeAgent,
					DID:              agentDID,
					PrincipalDID:     principalDID,
					PublicKeyEd25519: pub2, // current key
				}
				kvs.versions[agentEntityID] = []models.KeyVersion{
					{
						ID:         "kv1",
						EntityID:   agentEntityID,
						PublicKey:  pub,
						KeyIndex:   1,
						ValidUntil: &expiredAt,
					},
					{
						ID:       "kv2",
						EntityID: agentEntityID,
						PublicKey: pub2,
						KeyIndex: 2,
					},
				}
			},
			wantStatus: 200,
			wantCT:     "application/did+ld+json",
			checkResp: func(t *testing.T, doc map[string]interface{}) {
				vms, ok := doc["verificationMethod"].([]interface{})
				if !ok {
					t.Error("verificationMethod missing")
					return
				}
				if len(vms) != 2 {
					t.Errorf("verificationMethod length = %d, want 2 (for rotated key history)", len(vms))
				}

				// Only the active key should be in authentication/assertionMethod
				auth, ok := doc["authentication"].([]interface{})
				if !ok {
					t.Error("authentication missing")
					return
				}
				if len(auth) != 1 {
					t.Errorf("authentication length = %d, want 1 (only active key)", len(auth))
				}
				// Active key should be key-2 (no valid_until)
				if len(auth) > 0 {
					authRef := auth[0].(string)
					if !strings.Contains(authRef, "key-2") {
						t.Errorf("authentication[0] = %q, should reference key-2 (active key)", authRef)
					}
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			es := newMockEntityStore()
			kvs := newMockKeyVersionStore()
			if tc.setup != nil {
				tc.setup(es, kvs)
			}
			cfg := &config.Config{PlatformDomain: "atap.app"}
			_, app := newTestHandlerWithStores(es, kvs, cfg)

			req := httptest.NewRequest("GET", tc.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}

			if tc.wantCT != "" {
				ct := resp.Header.Get("Content-Type")
				if ct != tc.wantCT {
					t.Errorf("Content-Type = %q, want %q", ct, tc.wantCT)
				}
			}

			if tc.checkResp != nil && resp.StatusCode == tc.wantStatus {
				var doc map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tc.checkResp(t, doc)
			}
		})
	}
}
