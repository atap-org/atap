package atap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestTokenManager_GetAccessToken_ClientCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("DPoP") == "" {
			t.Error("missing DPoP header")
		}
		r.ParseForm()
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %q", r.FormValue("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token-123",
			"token_type":   "DPoP",
			"expires_in":   3600,
			"scope":        "atap:inbox",
		})
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		ClientSecret:   "atap_secret",
		PlatformDomain: "localhost:8080",
	})

	token, err := tm.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken: %v", err)
	}
	if token != "new-token-123" {
		t.Errorf("token = %q, want new-token-123", token)
	}
}

func TestTokenManager_GetAccessToken_Cached(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cached-token",
			"token_type":   "DPoP",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		ClientSecret:   "atap_secret",
		PlatformDomain: "localhost:8080",
	})

	ctx := context.Background()
	tok1, _ := tm.GetAccessToken(ctx)
	tok2, _ := tm.GetAccessToken(ctx)

	if tok1 != tok2 {
		t.Error("expected same cached token")
	}
	if callCount != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

func TestTokenManager_GetAccessToken_Refresh(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.FormValue("grant_type") == "refresh_token" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "refreshed-token",
				"token_type":    "DPoP",
				"expires_in":    3600,
				"refresh_token": "new-refresh",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "initial-token",
				"token_type":    "DPoP",
				"expires_in":    1, // expires immediately
				"refresh_token": "refresh-123",
			})
		}
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		ClientSecret:   "atap_secret",
		PlatformDomain: "localhost:8080",
	})

	ctx := context.Background()

	// First call obtains token.
	tok1, err := tm.GetAccessToken(ctx)
	if err != nil {
		t.Fatalf("first GetAccessToken: %v", err)
	}
	if tok1 != "initial-token" {
		t.Errorf("first token = %q", tok1)
	}

	// Force expiry by manipulating tokenObtainedAt.
	tm.mu.Lock()
	tm.tokenObtainedAt = time.Now().Add(-2 * time.Hour)
	tm.mu.Unlock()

	// Second call should refresh.
	tok2, err := tm.GetAccessToken(ctx)
	if err != nil {
		t.Fatalf("second GetAccessToken: %v", err)
	}
	if tok2 != "refreshed-token" {
		t.Errorf("refreshed token = %q", tok2)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestTokenManager_GetAccessToken_NoSecret(t *testing.T) {
	_, priv := testKeypair(t)
	httpClient := NewHTTPClient("http://localhost:9999", 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:human:test",
		PlatformDomain: "localhost:8080",
	})

	_, err := tm.GetAccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error when no client_secret")
	}
}

func TestTokenManager_Invalidate(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "token-" + string(rune('0'+callCount)),
			"token_type":   "DPoP",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		ClientSecret:   "atap_secret",
		PlatformDomain: "localhost:8080",
	})

	ctx := context.Background()
	tm.GetAccessToken(ctx)
	tm.Invalidate()
	tm.GetAccessToken(ctx)

	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls after invalidate, got %d", callCount)
	}
}

func TestTokenManager_Concurrent(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "concurrent-token",
			"token_type":   "DPoP",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		ClientSecret:   "atap_secret",
		PlatformDomain: "localhost:8080",
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := tm.GetAccessToken(context.Background())
			if err != nil {
				t.Errorf("concurrent GetAccessToken: %v", err)
			}
			if tok != "concurrent-token" {
				t.Errorf("concurrent token = %q", tok)
			}
		}()
	}
	wg.Wait()
}

func TestTokenManager_ObtainAuthorizationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/oauth/authorize":
			w.Header().Set("Location", "atap://callback?code=test-auth-code")
			w.WriteHeader(http.StatusFound)
		case r.URL.Path == "/v1/oauth/token":
			r.ParseForm()
			if r.FormValue("grant_type") != "authorization_code" {
				t.Errorf("grant_type = %q", r.FormValue("grant_type"))
			}
			if r.FormValue("code") != "test-auth-code" {
				t.Errorf("code = %q", r.FormValue("code"))
			}
			if r.FormValue("code_verifier") == "" {
				t.Error("missing code_verifier")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "authcode-token",
				"token_type":   "DPoP",
				"expires_in":   3600,
			})
		}
	}))
	defer server.Close()

	_, priv := testKeypair(t)
	httpClient := NewHTTPClient(server.URL, 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:human:test",
		PlatformDomain: "localhost:8080",
	})

	tok, err := tm.ObtainAuthorizationCode(context.Background(), "")
	if err != nil {
		t.Fatalf("ObtainAuthorizationCode: %v", err)
	}
	if tok.AccessToken != "authcode-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
}

func TestTokenManager_DefaultScopes(t *testing.T) {
	_, priv := testKeypair(t)
	httpClient := NewHTTPClient("http://localhost", 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient:     httpClient,
		SigningKey:     priv,
		DID:            "did:web:localhost%3A8080:agent:test",
		PlatformDomain: "localhost:8080",
	})
	if len(tm.scopes) != 4 {
		t.Errorf("default scopes count = %d, want 4", len(tm.scopes))
	}
}

func TestTokenManager_DomainFromDID(t *testing.T) {
	_, priv := testKeypair(t)
	httpClient := NewHTTPClient("http://localhost", 10*time.Second)
	tm := NewTokenManager(TokenManagerConfig{
		HTTPClient: httpClient,
		SigningKey: priv,
		DID:        "did:web:example.com%3A443:agent:test",
	})
	if tm.platformDomain != "example.com:443" {
		t.Errorf("platformDomain = %q, want example.com:443", tm.platformDomain)
	}
}
