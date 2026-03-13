package didcomm_test

import (
	"encoding/json"
	"testing"

	"github.com/atap-dev/atap/platform/internal/didcomm"
)

// buildTestJWE creates a minimal JWE JSON fixture with the given recipient KID
// and sender SKID for testing mediator extraction without full encryption.
func buildTestJWE(t *testing.T, recipientKID, senderKID string) []byte {
	t.Helper()

	// Build a minimal but structurally valid JWE for mediator parsing tests.
	// We use Encrypt from the package to produce a real JWE with correct fields.
	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}
	_, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	plaintext := []byte(`{"id":"msg_test","type":"https://atap.dev/protocols/basic/1.0/ping","body":{}}`)
	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, recipientPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return jwe
}

func TestMediatorExtractRecipientKID(t *testing.T) {
	recipientKID := "did:web:atap.app:agent:recip01#key-x25519-1"
	senderKID := "did:web:atap.app:agent:sender01#key-x25519-1"

	jwe := buildTestJWE(t, recipientKID, senderKID)

	t.Run("extracts recipient KID from valid JWE", func(t *testing.T) {
		kid, err := didcomm.ExtractRecipientKID(jwe)
		if err != nil {
			t.Fatalf("ExtractRecipientKID: %v", err)
		}
		if kid != recipientKID {
			t.Errorf("kid = %q, want %q", kid, recipientKID)
		}
	})

	t.Run("returns error for empty JWE", func(t *testing.T) {
		_, err := didcomm.ExtractRecipientKID([]byte(`{}`))
		if err == nil {
			t.Error("expected error for JWE with no recipients, got nil")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		_, err := didcomm.ExtractRecipientKID([]byte(`not-json`))
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("returns error for JWE with empty recipient KID", func(t *testing.T) {
		// Craft a JWE with a recipient that has no kid
		badJWE := map[string]interface{}{
			"protected":  "eyJhbGciOiJFQ0RILTFQVStBMjU2S1ciLCJlbmMiOiJBMjU2Q0JDLUhTNTEyIn0",
			"recipients": []interface{}{map[string]interface{}{"header": map[string]interface{}{}}},
			"iv":         "aaa",
			"ciphertext": "bbb",
			"tag":        "ccc",
		}
		badBytes, _ := json.Marshal(badJWE)
		_, err := didcomm.ExtractRecipientKID(badBytes)
		if err == nil {
			t.Error("expected error for JWE with empty recipient KID, got nil")
		}
	})
}

func TestMediatorExtractSenderKID(t *testing.T) {
	recipientKID := "did:web:atap.app:agent:recip02#key-x25519-1"
	senderKID := "did:web:atap.app:agent:sender02#key-x25519-1"

	jwe := buildTestJWE(t, recipientKID, senderKID)

	t.Run("extracts sender KID from valid JWE", func(t *testing.T) {
		kid, err := didcomm.ExtractSenderKID(jwe)
		if err != nil {
			t.Fatalf("ExtractSenderKID: %v", err)
		}
		if kid != senderKID {
			t.Errorf("kid = %q, want %q", kid, senderKID)
		}
	})

	t.Run("returns error for JWE without protected header", func(t *testing.T) {
		noProtected := map[string]interface{}{
			"recipients": []interface{}{},
		}
		bytes, _ := json.Marshal(noProtected)
		_, err := didcomm.ExtractSenderKID(bytes)
		if err == nil {
			t.Error("expected error for JWE without protected header, got nil")
		}
	})
}

func TestMediatorValidateRecipientDomain(t *testing.T) {
	tests := []struct {
		name           string
		recipientKID   string
		platformDomain string
		want           bool
	}{
		{
			name:           "KID with matching domain returns true",
			recipientKID:   "did:web:atap.app:agent:test01#key-x25519-1",
			platformDomain: "atap.app",
			want:           true,
		},
		{
			name:           "KID with foreign domain returns false",
			recipientKID:   "did:web:evil.com:agent:test01#key-x25519-1",
			platformDomain: "atap.app",
			want:           false,
		},
		{
			name:           "KID without fragment still validates domain",
			recipientKID:   "did:web:atap.app:agent:test02",
			platformDomain: "atap.app",
			want:           true,
		},
		{
			name:           "non-did-web KID returns false",
			recipientKID:   "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			platformDomain: "atap.app",
			want:           false,
		},
		{
			name:           "empty KID returns false",
			recipientKID:   "",
			platformDomain: "atap.app",
			want:           false,
		},
		{
			name:           "subdomain does not match base domain",
			recipientKID:   "did:web:sub.atap.app:agent:test01#key-x25519-1",
			platformDomain: "atap.app",
			want:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := didcomm.ValidateRecipientDomain(tc.recipientKID, tc.platformDomain)
			if got != tc.want {
				t.Errorf("ValidateRecipientDomain(%q, %q) = %v, want %v",
					tc.recipientKID, tc.platformDomain, got, tc.want)
			}
		})
	}
}
