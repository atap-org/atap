package atap

import (
	"context"
	"testing"
)

func TestEntityAPI_Register(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"POST /v1/entities": {
			Status: 200,
			Body: map[string]interface{}{
				"id":            "01HXK123",
				"type":          "agent",
				"did":           "did:web:localhost%3A8080:agent:01HXK123",
				"name":          "test-agent",
				"key_id":        "key_abc_123",
				"public_key":    "pubkey-b64",
				"trust_level":   1,
				"client_secret": "atap_secret123",
				"created_at":    "2024-01-01T00:00:00Z",
			},
		},
	})
	defer server.Close()

	client, err := NewClient(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	entity, err := client.Entities.Register(context.Background(), "agent", &RegisterOptions{
		Name: "test-agent",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if entity.ID != "01HXK123" {
		t.Errorf("ID = %q", entity.ID)
	}
	if entity.Type != "agent" {
		t.Errorf("Type = %q", entity.Type)
	}
	if entity.Name != "test-agent" {
		t.Errorf("Name = %q", entity.Name)
	}
	if entity.ClientSecret != "atap_secret123" {
		t.Errorf("ClientSecret = %q", entity.ClientSecret)
	}
	if entity.DID != "did:web:localhost%3A8080:agent:01HXK123" {
		t.Errorf("DID = %q", entity.DID)
	}
}

func TestEntityAPI_Register_WithPublicKey(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"POST /v1/entities": {
			Status: 200,
			Body: map[string]interface{}{
				"id":   "01HXK456",
				"type": "machine",
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	entity, err := client.Entities.Register(context.Background(), "machine", &RegisterOptions{
		Name:      "my-machine",
		PublicKey: "base64pubkey",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if entity.ID != "01HXK456" {
		t.Errorf("ID = %q", entity.ID)
	}
}

func TestEntityAPI_Register_NilOptions(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"POST /v1/entities": {
			Status: 200,
			Body:   map[string]interface{}{"id": "01HXK789", "type": "agent"},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	entity, err := client.Entities.Register(context.Background(), "agent", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if entity.ID != "01HXK789" {
		t.Errorf("ID = %q", entity.ID)
	}
}

func TestEntityAPI_Get(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/entities/01HXK123": {
			Status: 200,
			Body: map[string]interface{}{
				"id":          "01HXK123",
				"type":        "agent",
				"did":         "did:web:localhost%3A8080:agent:01HXK123",
				"trust_level": 2,
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	entity, err := client.Entities.Get(context.Background(), "01HXK123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entity.ID != "01HXK123" {
		t.Errorf("ID = %q", entity.ID)
	}
	if entity.TrustLevel != 2 {
		t.Errorf("TrustLevel = %d", entity.TrustLevel)
	}
}

func TestEntityAPI_Get_NotFound(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	_, err := client.Entities.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
}

func TestEntityAPI_Delete(t *testing.T) {
	routes := map[string]mockRoute{
		"DELETE /v1/entities/01HXK123": {Status: 204},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	err := client.Entities.Delete(context.Background(), "01HXK123")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestEntityAPI_RotateKey(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/entities/01HXK123/keys/rotate": {
			Status: 200,
			Body: map[string]interface{}{
				"id":         "key_new_abc",
				"entity_id":  "01HXK123",
				"key_index":  2,
				"valid_from": "2024-01-02T00:00:00Z",
				"created_at": "2024-01-02T00:00:00Z",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	kv, err := client.Entities.RotateKey(context.Background(), "01HXK123", "new-pubkey-b64")
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if kv.ID != "key_new_abc" {
		t.Errorf("ID = %q", kv.ID)
	}
	if kv.KeyIndex != 2 {
		t.Errorf("KeyIndex = %d", kv.KeyIndex)
	}
}
