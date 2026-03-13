package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/config"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	cfg := &config.Config{
		PlatformDomain: "test.atap.dev",
	}
	return &Handler{
		config: cfg,
		log:    zerolog.Nop(),
	}
}

// newTestHandlerWithStores creates a Handler with mock stores for entity tests.
func newTestHandlerWithStores(es EntityStore, kvs KeyVersionStore, cfg *config.Config) (*Handler, *fiber.App) {
	h := &Handler{
		entityStore:     es,
		keyVersionStore: kvs,
		config:          cfg,
		log:             zerolog.Nop(),
	}
	app := fiber.New()
	h.SetupRoutes(app)
	return h, app
}

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()
	h := newTestHandler(t)
	app := fiber.New()
	h.SetupRoutes(app)
	return app
}

func TestDiscovery(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/.well-known/atap.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Test required top-level fields
	requiredFields := []string{"domain", "api_base", "didcomm_endpoint", "claim_types", "max_approval_ttl", "trust_level", "oauth"}
	for _, field := range requiredFields {
		if _, ok := doc[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Test domain matches config
	if domain, ok := doc["domain"].(string); !ok || domain != "test.atap.dev" {
		t.Errorf("expected domain=test.atap.dev, got %v", doc["domain"])
	}

	// Test api_base
	if apiBase, ok := doc["api_base"].(string); !ok || apiBase != "https://test.atap.dev/v1" {
		t.Errorf("expected api_base=https://test.atap.dev/v1, got %v", doc["api_base"])
	}

	// Test didcomm_endpoint
	if didcomm, ok := doc["didcomm_endpoint"].(string); !ok || didcomm != "https://test.atap.dev/v1/didcomm" {
		t.Errorf("expected didcomm_endpoint=https://test.atap.dev/v1/didcomm, got %v", doc["didcomm_endpoint"])
	}

	// Test claim_types is array (empty)
	claimTypes, ok := doc["claim_types"].([]any)
	if !ok {
		t.Errorf("expected claim_types to be array, got %T", doc["claim_types"])
	} else if len(claimTypes) != 0 {
		t.Errorf("expected claim_types to be empty, got %v", claimTypes)
	}

	// Test max_approval_ttl is 86400
	if ttl, ok := doc["max_approval_ttl"].(float64); !ok || ttl != 86400 {
		t.Errorf("expected max_approval_ttl=86400, got %v", doc["max_approval_ttl"])
	}

	// Test trust_level is 1
	if tl, ok := doc["trust_level"].(float64); !ok || tl != 1 {
		t.Errorf("expected trust_level=1, got %v", doc["trust_level"])
	}

	// Test oauth object
	oauth, ok := doc["oauth"].(map[string]any)
	if !ok {
		t.Fatalf("expected oauth to be object, got %T", doc["oauth"])
	}

	// oauth required fields
	oauthFields := []string{"token_endpoint", "authorize_endpoint", "grant_types", "dpop_required"}
	for _, field := range oauthFields {
		if _, ok := oauth[field]; !ok {
			t.Errorf("missing oauth field: %s", field)
		}
	}

	if te, ok := oauth["token_endpoint"].(string); !ok || te != "https://test.atap.dev/v1/oauth/token" {
		t.Errorf("expected oauth.token_endpoint=https://test.atap.dev/v1/oauth/token, got %v", oauth["token_endpoint"])
	}

	if ae, ok := oauth["authorize_endpoint"].(string); !ok || ae != "https://test.atap.dev/v1/oauth/authorize" {
		t.Errorf("expected oauth.authorize_endpoint=https://test.atap.dev/v1/oauth/authorize, got %v", oauth["authorize_endpoint"])
	}

	if dpop, ok := oauth["dpop_required"].(bool); !ok || !dpop {
		t.Errorf("expected oauth.dpop_required=true, got %v", oauth["dpop_required"])
	}

	grantTypes, ok := oauth["grant_types"].([]any)
	if !ok {
		t.Errorf("expected oauth.grant_types to be array, got %T", oauth["grant_types"])
	} else {
		grantMap := make(map[string]bool)
		for _, g := range grantTypes {
			if s, ok := g.(string); ok {
				grantMap[s] = true
			}
		}
		if !grantMap["client_credentials"] {
			t.Error("oauth.grant_types missing client_credentials")
		}
		if !grantMap["authorization_code"] {
			t.Error("oauth.grant_types missing authorization_code")
		}
	}
}
