package approval_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// makeTestTemplate builds a Template using Adaptive Cards format (spec v1.0-rc1).
func makeTestTemplate() *models.Template {
	card, _ := json.Marshal(map[string]any{
		"type":    "AdaptiveCard",
		"version": "1.5",
		"body": []map[string]any{
			{"type": "TextBlock", "text": "Test Template", "weight": "Bolder"},
			{"type": "FactSet", "facts": []map[string]any{
				{"title": "Amount", "value": "${amount}"},
			}},
		},
	})
	return &models.Template{
		AtapTemplate: "1",
		Card:         json.RawMessage(card),
		Proof:        models.TemplateProof{},
	}
}

// signTestTemplate is a test helper that signs a template using EdDSA + go-jose.
// This replicates what a "via" entity would do client-side when publishing a template.
// It does NOT call SignTemplateProof (that function was removed from the server package).
func signTestTemplate(t *testing.T, tmpl *models.Template, keyID string) {
	t.Helper()

	// Build the template body without proof (same logic as VerifyTemplateProof uses internally).
	raw, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal template to map: %v", err)
	}
	delete(m, "proof")

	payload, err := crypto.CanonicalJSON(m)
	if err != nil {
		t.Fatalf("canonical JSON: %v", err)
	}

	_, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: priv},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	compact, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("compact serialize: %v", err)
	}

	// Detach payload: header..signature
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected JWS compact format")
	}
	detached := parts[0] + ".." + parts[2]

	tmpl.Proof.KID = keyID
	tmpl.Proof.Alg = "EdDSA"
	tmpl.Proof.Sig = detached
}

// signTestTemplateWithKey is like signTestTemplate but uses a specific key for verification.
func signTestTemplateWithKey(t *testing.T, tmpl *models.Template, keyID string) func() interface{} {
	t.Helper()

	raw, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal template to map: %v", err)
	}
	delete(m, "proof")

	payload, err := crypto.CanonicalJSON(m)
	if err != nil {
		t.Fatalf("canonical JSON: %v", err)
	}

	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: priv},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	compact, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("compact serialize: %v", err)
	}

	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected JWS compact format")
	}
	detached := parts[0] + ".." + parts[2]

	tmpl.Proof.KID = keyID
	tmpl.Proof.Alg = "EdDSA"
	tmpl.Proof.Sig = detached

	return func() interface{} { return pub }
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

// TestTemplateMarshalAdaptiveCard verifies that a Template JSON object contains
// "card" as a nested JSON object and does not include old fields like "brand",
// "display", or "subject_type".
func TestTemplateMarshalAdaptiveCard(t *testing.T) {
	tmpl := makeTestTemplate()

	raw, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	// Must have atap_template and card keys.
	if _, ok := m["atap_template"]; !ok {
		t.Error("expected atap_template key in JSON")
	}
	cardVal, ok := m["card"]
	if !ok {
		t.Fatal("expected card key in JSON")
	}

	// card must be a nested JSON object (not a string or null).
	if _, ok := cardVal.(map[string]any); !ok {
		t.Errorf("expected card to be a JSON object, got %T: %v", cardVal, cardVal)
	}

	// Must NOT have old fields.
	for _, oldField := range []string{"brand", "display", "subject_type", "fields"} {
		if _, ok := m[oldField]; ok {
			t.Errorf("unexpected legacy field %q in Template JSON", oldField)
		}
	}
}

// TestTemplateFormat verifies the atap_template / card / proof envelope structure.
func TestTemplateFormat(t *testing.T) {
	tmpl := &models.Template{
		AtapTemplate: "1",
		Card: json.RawMessage(`{"type":"AdaptiveCard","version":"1.5","body":[]}`),
		Proof: models.TemplateProof{
			KID: "did:web:example.com:entities:via#key-0",
			Alg: "EdDSA",
			Sig: "abc..def",
		},
	}

	raw, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Check envelope keys present.
	for _, key := range []string{"atap_template", "card", "proof"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected key %q in template JSON", key)
		}
	}

	// proof must be an object with kid/alg/sig.
	proofVal, _ := m["proof"].(map[string]any)
	if proofVal == nil {
		t.Fatal("expected proof to be a JSON object")
	}
	for _, k := range []string{"kid", "alg", "sig"} {
		if _, ok := proofVal[k]; !ok {
			t.Errorf("expected proof.%s key", k)
		}
	}
}

// TestTemplateJSONRoundTrip verifies that Card as json.RawMessage survives
// a marshal/unmarshal round-trip without mutation.
func TestTemplateJSONRoundTrip(t *testing.T) {
	originalCard := `{"type":"AdaptiveCard","version":"1.5","body":[{"type":"TextBlock","text":"Hello"}]}`
	tmpl := &models.Template{
		AtapTemplate: "1",
		Card:         json.RawMessage(originalCard),
	}

	raw, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var tmpl2 models.Template
	if err := json.Unmarshal(raw, &tmpl2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Compare card content by re-normalizing both through map round-trip.
	var orig, got map[string]any
	if err := json.Unmarshal([]byte(originalCard), &orig); err != nil {
		t.Fatalf("unmarshal original card: %v", err)
	}
	if err := json.Unmarshal(tmpl2.Card, &got); err != nil {
		t.Fatalf("unmarshal round-tripped card: %v", err)
	}

	origBytes, _ := json.Marshal(orig)
	gotBytes, _ := json.Marshal(got)
	if string(origBytes) != string(gotBytes) {
		t.Errorf("card content changed: orig=%s got=%s", origBytes, gotBytes)
	}
}

// TestVerifyTemplateProofAdaptiveCard signs a Template with an Adaptive Card
// using a test Ed25519 keypair and verifies the proof.
func TestVerifyTemplateProofAdaptiveCard(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	card := json.RawMessage(`{"type":"AdaptiveCard","version":"1.5","body":[{"type":"TextBlock","text":"Approve payment"}]}`)
	tmpl := &models.Template{
		AtapTemplate: "1",
		Card:         card,
	}

	keyID := "did:web:example.com:entities:via#key-0"

	// Sign using the helper (not SignTemplateProof — that function is removed).
	// Build the templateWithoutProof payload manually.
	raw, _ := json.Marshal(tmpl)
	var m map[string]any
	json.Unmarshal(raw, &m)
	delete(m, "proof")
	payload, _ := crypto.CanonicalJSON(m)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: priv},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	compact, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("compact serialize: %v", err)
	}
	parts := strings.Split(compact, ".")
	tmpl.Proof = models.TemplateProof{
		KID: keyID,
		Alg: "EdDSA",
		Sig: parts[0] + ".." + parts[2],
	}

	// VerifyTemplateProof must succeed.
	if err := approval.VerifyTemplateProof(tmpl, pub); err != nil {
		t.Errorf("VerifyTemplateProof: %v", err)
	}
}

func TestVerifyTemplateProof(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	tmpl := makeTestTemplate()
	keyID := "did:web:example.com:entities:via#key-0"

	// Sign using go-jose directly (SignTemplateProof was removed).
	raw, _ := json.Marshal(tmpl)
	var m map[string]any
	json.Unmarshal(raw, &m)
	delete(m, "proof")
	payload, _ := crypto.CanonicalJSON(m)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: priv},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	compact, _ := jws.CompactSerialize()
	parts := strings.Split(compact, ".")

	tmpl.Proof.KID = keyID
	tmpl.Proof.Alg = "EdDSA"
	tmpl.Proof.Sig = parts[0] + ".." + parts[2]

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

	raw, _ := json.Marshal(tmpl)
	var m map[string]any
	json.Unmarshal(raw, &m)
	delete(m, "proof")
	payload, _ := crypto.CanonicalJSON(m)

	signer, _ := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: priv},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	jws, _ := signer.Sign(payload)
	compact, _ := jws.CompactSerialize()
	parts := strings.Split(compact, ".")
	tmpl.Proof.KID = keyID
	tmpl.Proof.Alg = "EdDSA"
	tmpl.Proof.Sig = parts[0] + ".." + parts[2]

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
