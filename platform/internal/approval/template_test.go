package approval_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

func makeTestTemplate() *models.Template {
	return &models.Template{
		AtapTemplate: "1",
		SubjectType:  "com.example.test",
		Brand: models.TemplateBrand{
			Name:    "Test Brand",
			LogoURL: "https://example.com/logo.png",
			Colors: models.TemplateColors{
				Primary:    "#000000",
				Accent:     "#ffffff",
				Background: "#f5f5f5",
			},
		},
		Display: models.TemplateDisplay{
			Title: "Test Template",
			Fields: []models.TemplateField{
				{Key: "amount", Label: "Amount", Type: "currency"},
			},
		},
		Proof: models.TemplateProof{},
	}
}

func TestFetchTemplateRejectsHTTP(t *testing.T) {
	_, err := approval.FetchTemplate(context.Background(), "http://example.com/template.json")
	if err == nil {
		t.Error("expected error for http:// URL, got nil")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("expected error to mention https requirement, got: %v", err)
	}
}

func TestFetchTemplateRejectsRedirect(t *testing.T) {
	// Server that redirects to another HTTPS URL
	redirectTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(makeTestTemplate())
	}))
	defer redirectTarget.Close()

	redirectSource := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/template.json", http.StatusFound)
	}))
	defer redirectSource.Close()

	// FetchTemplate should reject the redirect
	_, err := approval.FetchTemplate(context.Background(), redirectSource.URL+"/template.json")
	if err == nil {
		t.Error("expected error for redirect, got nil")
	}
}

func TestSSRFBlocksRFC1918(t *testing.T) {
	cases := []string{
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
	}
	for _, ipStr := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", ipStr)
		}
		if !approval.IsBlockedIP(ip) {
			t.Errorf("expected %s to be blocked (RFC 1918)", ipStr)
		}
	}
}

func TestSSRFBlocksLoopback(t *testing.T) {
	cases := []string{
		"127.0.0.1",
		"127.0.0.2",
	}
	for _, ipStr := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", ipStr)
		}
		if !approval.IsBlockedIP(ip) {
			t.Errorf("expected %s to be blocked (loopback)", ipStr)
		}
	}
}

func TestSSRFBlocksLinkLocal(t *testing.T) {
	cases := []string{
		"169.254.0.1",
		"169.254.169.254", // AWS EC2 metadata endpoint
	}
	for _, ipStr := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", ipStr)
		}
		if !approval.IsBlockedIP(ip) {
			t.Errorf("expected %s to be blocked (link-local / cloud metadata)", ipStr)
		}
	}
}

func TestSSRFBlocksIPv6Loopback(t *testing.T) {
	ip := net.ParseIP("::1")
	if ip == nil {
		t.Fatal("failed to parse ::1")
	}
	if !approval.IsBlockedIP(ip) {
		t.Error("expected ::1 to be blocked (IPv6 loopback)")
	}
}

func TestVerifyTemplateProof(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	tmpl := makeTestTemplate()
	keyID := "did:web:example.com:entities:via#key-0"

	if err := approval.SignTemplateProof(tmpl, priv, keyID); err != nil {
		t.Fatalf("SignTemplateProof: %v", err)
	}

	if tmpl.Proof.KID != keyID {
		t.Errorf("expected proof.kid=%q, got %q", keyID, tmpl.Proof.KID)
	}
	if tmpl.Proof.Alg != "EdDSA" {
		t.Errorf("expected proof.alg=EdDSA, got %q", tmpl.Proof.Alg)
	}
	if tmpl.Proof.Sig == "" {
		t.Error("expected proof.sig to be set")
	}

	if err := approval.VerifyTemplateProof(tmpl, pub); err != nil {
		t.Errorf("VerifyTemplateProof: %v", err)
	}
}

func TestVerifyTemplateProofWrongKey(t *testing.T) {
	_, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	wrongPub, _, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	tmpl := makeTestTemplate()
	keyID := "did:web:example.com:entities:via#key-0"

	if err := approval.SignTemplateProof(tmpl, priv, keyID); err != nil {
		t.Fatalf("SignTemplateProof: %v", err)
	}

	if err := approval.VerifyTemplateProof(tmpl, wrongPub); err == nil {
		t.Error("expected error with wrong public key, got nil")
	}
}

func TestFallbackRendering(t *testing.T) {
	// FetchTemplate with empty templateURL (no via) should return nil, nil
	result, err := approval.FetchTemplate(context.Background(), "")
	if err != nil {
		t.Errorf("expected nil error for empty URL, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil template for empty URL, got: %v", result)
	}
}
