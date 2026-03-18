package atap

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Entities == nil {
		t.Error("Entities is nil")
	}
	if client.Approvals == nil {
		t.Error("Approvals is nil")
	}
	if client.Revocations == nil {
		t.Error("Revocations is nil")
	}
	if client.DIDComm == nil {
		t.Error("DIDComm is nil")
	}
	if client.Credentials == nil {
		t.Error("Credentials is nil")
	}
	if client.Discovery == nil {
		t.Error("Discovery is nil")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	privB64 := base64.StdEncoding.EncodeToString(priv.Seed())

	client, err := NewClient(
		WithBaseURL("http://custom:9090"),
		WithDID("did:web:custom%3A9090:agent:abc"),
		WithPrivateKey(privB64),
		WithClientSecret("atap_secret"),
		WithScopes([]string{"atap:inbox"}),
		WithPlatformDomain("custom:9090"),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if client.did != "did:web:custom%3A9090:agent:abc" {
		t.Errorf("did = %q", client.did)
	}
	if client.platformDomain != "custom:9090" {
		t.Errorf("platformDomain = %q", client.platformDomain)
	}
	if client.signingKey == nil {
		t.Error("signingKey is nil")
	}
	if client.tokenManager == nil {
		t.Error("tokenManager is nil")
	}
}

func TestNewClient_WithSigningKey(t *testing.T) {
	_, priv := testKeypair(t)
	client, err := NewClient(
		WithDID("did:web:localhost%3A8080:agent:test"),
		WithSigningKey(priv),
		WithClientSecret("secret"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.signingKey == nil {
		t.Error("signingKey is nil")
	}
}

func TestNewClient_InvalidPrivateKey(t *testing.T) {
	_, err := NewClient(
		WithPrivateKey("!!!invalid!!!"),
	)
	if err == nil {
		t.Fatal("expected error for invalid private key")
	}
}

func TestNewClient_DomainFromDID(t *testing.T) {
	_, priv := testKeypair(t)
	client, err := NewClient(
		WithDID("did:web:example.com%3A443:agent:xyz"),
		WithSigningKey(priv),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.platformDomain != "example.com:443" {
		t.Errorf("platformDomain = %q, want example.com:443", client.platformDomain)
	}
}

func TestClient_TokenManager_NotConfigured(t *testing.T) {
	client, _ := NewClient()
	_, err := client.TokenManager()
	if err == nil {
		t.Fatal("expected error when no token manager configured")
	}
}

func TestClient_TokenManager_Configured(t *testing.T) {
	_, priv := testKeypair(t)
	client, _ := NewClient(
		WithDID("did:web:localhost:agent:test"),
		WithSigningKey(priv),
		WithClientSecret("secret"),
	)
	tm, err := client.TokenManager()
	if err != nil {
		t.Fatalf("TokenManager: %v", err)
	}
	if tm == nil {
		t.Error("TokenManager is nil")
	}
}

func TestClient_AuthedRequest_NotConfigured(t *testing.T) {
	client, _ := NewClient()
	_, err := client.authedRequest(context.Background(), "GET", "/v1/test", nil)
	if err == nil {
		t.Fatal("expected error when auth not configured")
	}
}

func TestClient_Close(t *testing.T) {
	client, _ := NewClient()
	client.Close() // Should not panic.
}

func TestErrors_ATAPError(t *testing.T) {
	err := NewATAPError("test error", 500)
	if err.Error() != "[500] test error" {
		t.Errorf("Error() = %q", err.Error())
	}

	err2 := NewATAPError("no status", 0)
	if err2.Error() != "no status" {
		t.Errorf("Error() = %q", err2.Error())
	}
}

func TestErrors_ATAPProblemError(t *testing.T) {
	problem := ProblemDetail{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: 400,
		Detail: "Missing field",
	}
	err := NewATAPProblemError(problem)
	expected := "[400] Bad Request: Missing field"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestErrors_ATAPProblemError_NoDetail(t *testing.T) {
	problem := ProblemDetail{
		Type:   "about:blank",
		Title:  "Bad Request",
		Status: 400,
	}
	err := NewATAPProblemError(problem)
	if err.Error() != "[400] Bad Request: Bad Request" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestErrors_TypeAssertions(t *testing.T) {
	authErr := NewATAPAuthError("auth failed", 401, nil)
	notFoundErr := NewATAPNotFoundError("not found", nil)
	conflictErr := NewATAPConflictError("conflict", nil)
	rateLimitErr := NewATAPRateLimitError("rate limited", nil)

	var e error

	e = authErr
	var a *ATAPAuthError
	if !errors.As(e, &a) {
		t.Error("ATAPAuthError not detected with errors.As")
	}

	e = notFoundErr
	var n *ATAPNotFoundError
	if !errors.As(e, &n) {
		t.Error("ATAPNotFoundError not detected with errors.As")
	}

	e = conflictErr
	var c *ATAPConflictError
	if !errors.As(e, &c) {
		t.Error("ATAPConflictError not detected with errors.As")
	}

	e = rateLimitErr
	var r *ATAPRateLimitError
	if !errors.As(e, &r) {
		t.Error("ATAPRateLimitError not detected with errors.As")
	}
}

func TestErrors_WithProblem(t *testing.T) {
	problem := &ProblemDetail{
		Type:   "about:blank",
		Title:  "Not Found",
		Status: 404,
		Detail: "Entity xyz not found",
	}

	authErr := NewATAPAuthError("auth", 401, problem)
	if authErr.Problem == nil {
		t.Error("ATAPAuthError.Problem is nil")
	}

	notFoundErr := NewATAPNotFoundError("nf", problem)
	if notFoundErr.Problem == nil {
		t.Error("ATAPNotFoundError.Problem is nil")
	}

	conflictErr := NewATAPConflictError("c", problem)
	if conflictErr.Problem == nil {
		t.Error("ATAPConflictError.Problem is nil")
	}

	rateLimitErr := NewATAPRateLimitError("rl", problem)
	if rateLimitErr.Problem == nil {
		t.Error("ATAPRateLimitError.Problem is nil")
	}
}
