package atap

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testKeypair generates a deterministic-ish test keypair.
func testKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate test keypair: %v", err)
	}
	return pub, priv
}

// mockServer creates a test HTTP server that responds to ATAP API endpoints.
// The handler map maps "METHOD /path" to response data and status code.
type mockRoute struct {
	Status int
	Body   interface{}
}

func newMockServer(t *testing.T, routes map[string]mockRoute) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		route, ok := routes[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type":   "about:blank",
				"title":  "Not Found",
				"status": 404,
				"detail": "no mock route for " + key,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.Status)
		if route.Body != nil {
			json.NewEncoder(w).Encode(route.Body)
		}
	}))
}

// newOAuthMockServer creates a mock server that also handles OAuth token requests.
func newOAuthMockServer(t *testing.T, routes map[string]mockRoute) *httptest.Server {
	t.Helper()
	// Add the OAuth token endpoint.
	if _, ok := routes["POST /v1/oauth/token"]; !ok {
		routes["POST /v1/oauth/token"] = mockRoute{
			Status: 200,
			Body: map[string]interface{}{
				"access_token": "test-access-token",
				"token_type":   "DPoP",
				"expires_in":   3600,
				"scope":        "atap:inbox atap:send atap:revoke atap:manage",
			},
		}
	}
	return newMockServer(t, routes)
}

// newTestClient creates a Client backed by a mock server with OAuth.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	_, priv := testKeypair(t)

	client, err := NewClient(
		WithBaseURL(server.URL),
		WithDID("did:web:localhost%3A8080:agent:test123"),
		WithSigningKey(priv),
		WithClientSecret("atap_test_secret"),
		WithPlatformDomain("localhost:8080"),
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	return client
}
