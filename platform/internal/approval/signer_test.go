package approval_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

func makeTestApproval() *models.Approval {
	now := time.Now().UTC()
	return &models.Approval{
		AtapApproval: "1",
		ID:           "apr_01test",
		CreatedAt:    now,
		From:         "did:web:example.com:entities:from",
		To:           "did:web:example.com:entities:to",
		Subject: models.ApprovalSubject{
			Type:       "com.example.test",
			Label:      "Test approval",
			Reversible: false,
			Payload:    json.RawMessage(`{"action":"test"}`),
		},
		Signatures: map[string]string{},
	}
}

func decodeBase64URLHeader(s string) (map[string]interface{}, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func TestSignApproval(t *testing.T) {
	_, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, "did:web:example.com:entities:from#key-0")
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	// Detached compact JWS must have exactly two dots (header..signature)
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %q", len(parts), jws)
	}
	if parts[1] != "" {
		t.Fatalf("expected empty middle segment (detached payload), got %q", parts[1])
	}
}

func TestSignApprovalKID(t *testing.T) {
	_, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	parts := strings.Split(jws, ".")
	header, err := decodeBase64URLHeader(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}

	if alg, ok := header["alg"].(string); !ok || alg != "EdDSA" {
		t.Errorf("expected alg=EdDSA, got %v", header["alg"])
	}
	if kid, ok := header["kid"].(string); !ok || kid != keyID {
		t.Errorf("expected kid=%q, got %v", keyID, header["kid"])
	}
}

func TestVerifyApprovalSignature(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	if err := approval.VerifyApprovalSignature(a, jws, keyID, pub); err != nil {
		t.Errorf("VerifyApprovalSignature: %v", err)
	}
}

func TestVerifyApprovalSignatureKIDMismatch(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	_ = pub // unused but needed for signature

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	wrongKID := "did:web:example.com:entities:other#key-0"
	err = approval.VerifyApprovalSignature(a, jws, wrongKID, pub)
	if err == nil {
		t.Error("expected error on kid mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "kid mismatch") {
		t.Errorf("expected 'kid mismatch' in error, got: %v", err)
	}
}

func TestVerifyApprovalSignatureWrongKey(t *testing.T) {
	_, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	wrongPub, _, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	err = approval.VerifyApprovalSignature(a, jws, keyID, wrongPub)
	if err == nil {
		t.Error("expected error with wrong public key, got nil")
	}
}

func TestVerifyApprovalSignatureMutatedDoc(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()
	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	// Mutate the approval
	a.From = "did:web:example.com:entities:attacker"

	err = approval.VerifyApprovalSignature(a, jws, keyID, pub)
	if err == nil {
		t.Error("expected error after mutation, got nil")
	}
}

func TestCanonicalPayloadExcludesSignatures(t *testing.T) {
	pub, priv, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	_ = pub

	keyID := "did:web:example.com:entities:from#key-0"
	a := makeTestApproval()

	// Add a dummy signature — should not affect signing payload
	a.Signatures["requester"] = "some-existing-sig"

	jws, err := approval.SignApproval(a, priv, keyID)
	if err != nil {
		t.Fatalf("SignApproval: %v", err)
	}

	// Verify with a clean approval (no signatures) — must work because signatures excluded
	a2 := *a
	a2.Signatures = map[string]string{}
	if err := approval.VerifyApprovalSignature(&a2, jws, keyID, pub); err != nil {
		t.Errorf("signatures field must be excluded from signing payload, verify failed: %v", err)
	}
}

// Ensure ed25519.PublicKey is imported (used in function signatures above).
var _ ed25519.PublicKey
