package atap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClient_Request_Success(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/test": {Status: 200, Body: map[string]string{"hello": "world"}},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.Request(context.Background(), "GET", "/v1/test", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if data["hello"] != "world" {
		t.Errorf("data = %v, want hello:world", data)
	}
}

func TestHTTPClient_Request_WithJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"received": body["key"]})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.Request(context.Background(), "POST", "/v1/test", &RequestOptions{
		JSONBody: map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if data["received"] != "value" {
		t.Errorf("received = %v, want value", data["received"])
	}
}

func TestHTTPClient_Request_WithParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"entity": r.URL.Query().Get("entity")})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.Request(context.Background(), "GET", "/v1/test", &RequestOptions{
		Params: map[string]string{"entity": "did:web:test"},
	})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if data["entity"] != "did:web:test" {
		t.Errorf("entity = %v, want did:web:test", data["entity"])
	}
}

func TestHTTPClient_Request_204NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.Request(context.Background(), "DELETE", "/v1/test", nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty map for 204, got %v", data)
	}
}

func TestHTTPClient_Request_401Error(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/protected": {
			Status: 401,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Invalid token",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/protected", nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}

	var authErr *ATAPAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ATAPAuthError, got %T: %v", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("status = %d, want 401", authErr.StatusCode)
	}
	if authErr.Problem == nil {
		t.Fatal("expected problem detail")
	}
	if authErr.Problem.Detail != "Invalid token" {
		t.Errorf("detail = %q, want 'Invalid token'", authErr.Problem.Detail)
	}
}

func TestHTTPClient_Request_403Error(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/forbidden": {
			Status: 403,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Forbidden",
				"status": 403,
				"detail": "Insufficient scope",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/forbidden", nil)

	var authErr *ATAPAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ATAPAuthError, got %T", err)
	}
	if authErr.StatusCode != 403 {
		t.Errorf("status = %d, want 403", authErr.StatusCode)
	}
}

func TestHTTPClient_Request_404Error(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/missing": {
			Status: 404,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Not Found",
				"status": 404,
				"detail": "Entity not found",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/missing", nil)

	var notFoundErr *ATAPNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Fatalf("expected ATAPNotFoundError, got %T", err)
	}
}

func TestHTTPClient_Request_409Error(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"POST /v1/conflict": {
			Status: 409,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Conflict",
				"status": 409,
				"detail": "Entity already exists",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "POST", "/v1/conflict", nil)

	var conflictErr *ATAPConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ATAPConflictError, got %T", err)
	}
}

func TestHTTPClient_Request_429Error(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/limited": {
			Status: 429,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Too Many Requests",
				"status": 429,
				"detail": "Rate limit exceeded",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/limited", nil)

	var rateLimitErr *ATAPRateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Fatalf("expected ATAPRateLimitError, got %T", err)
	}
}

func TestHTTPClient_Request_500ProblemError(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/error": {
			Status: 500,
			Body: map[string]interface{}{
				"type":   "about:blank",
				"title":  "Internal Server Error",
				"status": 500,
				"detail": "Something went wrong",
			},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/error", nil)

	var problemErr *ATAPProblemError
	if !errors.As(err, &problemErr) {
		t.Fatalf("expected ATAPProblemError, got %T: %v", err, err)
	}
}

func TestHTTPClient_Request_500NonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("plain text error"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/error", nil)

	var atapErr *ATAPError
	if !errors.As(err, &atapErr) {
		t.Fatalf("expected ATAPError, got %T", err)
	}
}

func TestHTTPClient_Request_500GenericJSON(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/error": {
			Status: 500,
			Body:   map[string]interface{}{"message": "something broke"},
		},
	})
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.Request(context.Background(), "GET", "/v1/error", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClient_PostForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		dpop := r.Header.Get("DPoP")
		if dpop == "" {
			t.Error("missing DPoP header")
		}
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok123",
			"token_type":   "DPoP",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.PostForm(context.Background(), "/v1/oauth/token", map[string]string{
		"grant_type": "client_credentials",
	}, "dpop-proof-here")
	if err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	if data["access_token"] != "tok123" {
		t.Errorf("access_token = %v", data["access_token"])
	}
}

func TestHTTPClient_GetRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dpop := r.Header.Get("DPoP")
		if dpop == "" {
			t.Error("missing DPoP header on redirect request")
		}
		w.Header().Set("Location", "atap://callback?code=authcode123")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	location, err := client.GetRedirect(context.Background(), "/v1/oauth/authorize", map[string]string{
		"response_type": "code",
	}, "dpop-proof-here")
	if err != nil {
		t.Fatalf("GetRedirect: %v", err)
	}
	if location != "atap://callback?code=authcode123" {
		t.Errorf("location = %q", location)
	}
}

func TestHTTPClient_GetRedirect_Not302(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.GetRedirect(context.Background(), "/v1/test", nil, "")
	if err == nil {
		t.Fatal("expected error for non-302 response")
	}
}

func TestHTTPClient_GetRedirect_NoLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
		// No Location header.
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	_, err := client.GetRedirect(context.Background(), "/v1/test", nil, "")
	if err == nil {
		t.Fatal("expected error for missing Location header")
	}
}

func TestHTTPClient_AuthenticatedRequest(t *testing.T) {
	_, priv := testKeypair(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "DPoP test-token" {
			t.Errorf("Authorization = %q, want 'DPoP test-token'", auth)
		}
		dpop := r.Header.Get("DPoP")
		if dpop == "" {
			t.Error("missing DPoP header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.AuthenticatedRequest(context.Background(), "GET", "/v1/test", priv, "test-token", "example.com", nil)
	if err != nil {
		t.Fatalf("AuthenticatedRequest: %v", err)
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v", data["status"])
	}
}

func TestHTTPClient_Request_2xxNonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	data, err := client.Request(context.Background(), "GET", "/v1/test", nil)
	if err != nil {
		t.Fatalf("unexpected error for 2xx non-JSON: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty map, got %v", data)
	}
}
