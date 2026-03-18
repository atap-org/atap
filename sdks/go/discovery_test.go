package atap

import (
	"context"
	"testing"
)

func TestDiscoveryAPI_Discover(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /.well-known/atap.json": {
			Status: 200,
			Body: map[string]interface{}{
				"domain":           "localhost:8080",
				"api_base":         "http://localhost:8080/v1",
				"didcomm_endpoint": "http://localhost:8080/v1/didcomm",
				"claim_types":      []interface{}{"email", "phone", "personhood"},
				"max_approval_ttl": "168h",
				"trust_level":      3,
				"oauth": map[string]interface{}{
					"token_endpoint":     "http://localhost:8080/v1/oauth/token",
					"authorize_endpoint": "http://localhost:8080/v1/oauth/authorize",
				},
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	doc, err := client.Discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if doc.Domain != "localhost:8080" {
		t.Errorf("Domain = %q", doc.Domain)
	}
	if doc.APIBase != "http://localhost:8080/v1" {
		t.Errorf("APIBase = %q", doc.APIBase)
	}
	if len(doc.ClaimTypes) != 3 {
		t.Errorf("ClaimTypes count = %d", len(doc.ClaimTypes))
	}
	if doc.TrustLevel != 3 {
		t.Errorf("TrustLevel = %d", doc.TrustLevel)
	}
	if doc.OAuth == nil {
		t.Error("OAuth is nil")
	}
}

func TestDiscoveryAPI_ResolveDID(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /agent/01HXK123/did.json": {
			Status: 200,
			Body: map[string]interface{}{
				"id":       "did:web:localhost%3A8080:agent:01HXK123",
				"@context": []interface{}{"https://www.w3.org/ns/did/v1"},
				"verificationMethod": []interface{}{
					map[string]interface{}{
						"id":                 "#key-0",
						"type":               "Ed25519VerificationKey2020",
						"controller":         "did:web:localhost%3A8080:agent:01HXK123",
						"publicKeyMultibase": "z6Mk...",
					},
				},
				"authentication":  []interface{}{"#key-0"},
				"assertionMethod": []interface{}{"#key-0"},
				"keyAgreement":    []interface{}{"#key-1"},
				"service": []interface{}{
					map[string]interface{}{
						"id":              "#didcomm",
						"type":            "DIDCommMessaging",
						"serviceEndpoint": "http://localhost:8080/v1/didcomm",
					},
				},
				"atap:type":      "agent",
				"atap:principal": "did:web:localhost%3A8080:human:h1",
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	doc, err := client.Discovery.ResolveDID(context.Background(), "agent", "01HXK123")
	if err != nil {
		t.Fatalf("ResolveDID: %v", err)
	}
	if doc.ID != "did:web:localhost%3A8080:agent:01HXK123" {
		t.Errorf("ID = %q", doc.ID)
	}
	if len(doc.Context) != 1 {
		t.Errorf("Context count = %d", len(doc.Context))
	}
	if len(doc.VerificationMethod) != 1 {
		t.Errorf("VerificationMethod count = %d", len(doc.VerificationMethod))
	}
	if doc.VerificationMethod[0].Type != "Ed25519VerificationKey2020" {
		t.Errorf("VM type = %q", doc.VerificationMethod[0].Type)
	}
	if len(doc.Authentication) != 1 {
		t.Errorf("Authentication count = %d", len(doc.Authentication))
	}
	if doc.ATAPType != "agent" {
		t.Errorf("ATAPType = %q", doc.ATAPType)
	}
	if doc.ATAPPrincipal != "did:web:localhost%3A8080:human:h1" {
		t.Errorf("ATAPPrincipal = %q", doc.ATAPPrincipal)
	}
	if len(doc.Service) != 1 {
		t.Errorf("Service count = %d", len(doc.Service))
	}
}

func TestDiscoveryAPI_ServerDID(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /server/platform/did.json": {
			Status: 200,
			Body: map[string]interface{}{
				"id":        "did:web:localhost%3A8080:server:platform",
				"atap:type": "server",
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	doc, err := client.Discovery.ServerDID(context.Background())
	if err != nil {
		t.Fatalf("ServerDID: %v", err)
	}
	if doc.ATAPType != "server" {
		t.Errorf("ATAPType = %q", doc.ATAPType)
	}
}

func TestDiscoveryAPI_Health(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/health": {
			Status: 200,
			Body:   map[string]interface{}{"status": "ok", "version": "1.0.0"},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	data, err := client.Discovery.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v", data["status"])
	}
}
