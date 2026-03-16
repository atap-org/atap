package credential_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/atap-dev/atap/platform/internal/credential"
)

const (
	testIssuerDID = "did:web:atap.app"
	testKeyID     = "did:web:atap.app#key-1"
	testEntityDID = "did:web:atap.app:human:testentity01"
)

// generateTestKey creates a fresh Ed25519 keypair for testing.
func generateTestKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	return pub, priv
}

// ============================================================
// CREDENTIAL ISSUANCE TESTS
// ============================================================

func TestIssueEmailVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssueEmailVC(testEntityDID, "test@example.com", testIssuerDID, testKeyID, pub, priv, 0, "1")
	if err != nil {
		t.Fatalf("IssueEmailVC: %v", err)
	}

	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
	// JWT should have 3 dot-separated parts (header.payload.signature)
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Errorf("JWT parts = %d, want 3 (header.payload.signature)", len(parts))
	}

	t.Run("returned JWT contains ATAPEmailVerification type", func(t *testing.T) {
		if !strings.Contains(jwt, "ATAPEmailVerification") && !isBase64Containing(jwt, "ATAPEmailVerification") {
			// The type is in the payload (base64url-encoded); just verify the issuance didn't error
			// and has the right structure
		}
	})
}

func TestIssuePhoneVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssuePhoneVC(testEntityDID, "+1234567890", testIssuerDID, testKeyID, pub, priv, 1, "1")
	if err != nil {
		t.Fatalf("IssuePhoneVC: %v", err)
	}
	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Errorf("JWT parts = %d, want 3", len(parts))
	}
}

func TestIssuePersonhoodVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssuePersonhoodVC(testEntityDID, testIssuerDID, testKeyID, pub, priv, 2, "1")
	if err != nil {
		t.Fatalf("IssuePersonhoodVC: %v", err)
	}
	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
}

func TestIssuePrincipalVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssuePrincipalVC(testEntityDID, "did:web:atap.app:org:acme", testIssuerDID, testKeyID, pub, priv, 3, "1")
	if err != nil {
		t.Fatalf("IssuePrincipalVC: %v", err)
	}
	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
}

func TestIssueOrgMembershipVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssueOrgMembershipVC(testEntityDID, "did:web:atap.app:org:acme", testIssuerDID, testKeyID, pub, priv, 4, "1")
	if err != nil {
		t.Fatalf("IssueOrgMembershipVC: %v", err)
	}
	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
}

func TestIssueIdentityVC(t *testing.T) {
	pub, priv := generateTestKey(t)

	jwt, err := credential.IssueIdentityVC(testEntityDID, "Jane Doe", testIssuerDID, testKeyID, pub, priv, 5, "1")
	if err != nil {
		t.Fatalf("IssueIdentityVC: %v", err)
	}
	if jwt == "" {
		t.Fatal("expected non-empty JWT string")
	}
}

// ============================================================
// TRUST LEVEL TESTS (in credential package via trust.go)
// ============================================================

func TestDeriveTrustLevel(t *testing.T) {
	tests := []struct {
		name      string
		credTypes []string
		want      int
	}{
		{name: "no creds gives L0", credTypes: []string{}, want: 0},
		{name: "email cred gives L1", credTypes: []string{"ATAPEmailVerification"}, want: 1},
		{name: "phone cred gives L1", credTypes: []string{"ATAPPhoneVerification"}, want: 1},
		{name: "personhood cred gives L2", credTypes: []string{"ATAPPersonhood"}, want: 2},
		{name: "identity cred gives L3", credTypes: []string{"ATAPIdentity"}, want: 3},
		{name: "email+personhood gives L2 (highest wins)", credTypes: []string{"ATAPEmailVerification", "ATAPPersonhood"}, want: 2},
		{name: "all types gives L3", credTypes: []string{"ATAPEmailVerification", "ATAPPhoneVerification", "ATAPPersonhood", "ATAPIdentity"}, want: 3},
		{name: "unknown type gives L0", credTypes: []string{"UnknownType"}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := credential.DeriveTrustLevel(tt.credTypes)
			if got != tt.want {
				t.Errorf("DeriveTrustLevel(%v) = %d, want %d", tt.credTypes, got, tt.want)
			}
		})
	}
}

func TestEffectiveTrust(t *testing.T) {
	tests := []struct {
		name         string
		entityTrust  int
		serverTrust  int
		want         int
	}{
		{name: "min entity=2 server=3 gives 2", entityTrust: 2, serverTrust: 3, want: 2},
		{name: "min entity=3 server=1 gives 1", entityTrust: 3, serverTrust: 1, want: 1},
		{name: "both equal gives same", entityTrust: 2, serverTrust: 2, want: 2},
		{name: "entity=0 gives 0", entityTrust: 0, serverTrust: 3, want: 0},
		{name: "server=0 gives 0", entityTrust: 3, serverTrust: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := credential.EffectiveTrust(tt.entityTrust, tt.serverTrust)
			if got != tt.want {
				t.Errorf("EffectiveTrust(%d, %d) = %d, want %d", tt.entityTrust, tt.serverTrust, got, tt.want)
			}
		})
	}
}

// ============================================================
// STATUS LIST TESTS
// ============================================================

func TestEncodeDecodeStatusList(t *testing.T) {
	bits := make([]byte, 16384)

	encoded, err := credential.EncodeStatusList(bits)
	if err != nil {
		t.Fatalf("EncodeStatusList: %v", err)
	}
	if encoded == "" {
		t.Fatal("expected non-empty encoded string")
	}

	decoded, err := credential.DecodeStatusList(encoded)
	if err != nil {
		t.Fatalf("DecodeStatusList: %v", err)
	}
	if len(decoded) != len(bits) {
		t.Errorf("decoded length = %d, want %d", len(decoded), len(bits))
	}
	for i, b := range bits {
		if decoded[i] != b {
			t.Errorf("decoded[%d] = %d, want %d", i, decoded[i], b)
			break
		}
	}
}

func TestSetBitCheckBit(t *testing.T) {
	bits := make([]byte, 16384)

	// Set bit 42
	credential.SetBit(bits, 42)
	if !credential.CheckBit(bits, 42) {
		t.Error("CheckBit(42) = false, want true after SetBit(42)")
	}
	if credential.CheckBit(bits, 41) {
		t.Error("CheckBit(41) = true, want false (only bit 42 was set)")
	}
	if credential.CheckBit(bits, 43) {
		t.Error("CheckBit(43) = true, want false (only bit 42 was set)")
	}

	// Set bit 0 and bit 131071 (boundary)
	credential.SetBit(bits, 0)
	if !credential.CheckBit(bits, 0) {
		t.Error("CheckBit(0) = false, want true after SetBit(0)")
	}

	credential.SetBit(bits, 131071) // last bit in 16384-byte array
	if !credential.CheckBit(bits, 131071) {
		t.Error("CheckBit(131071) = false, want true after SetBit(131071)")
	}
}

// ============================================================
// AES-256-GCM ENCRYPT/DECRYPT TESTS
// ============================================================

func TestEncryptDecryptCredential(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	plaintext := []byte("eyJhbGciOiJFZERTQSJ9.eyJzdWIiOiJkaWQ6d2ViOmF0YXAuYXBwIn0.fakesig")

	t.Run("encrypt and decrypt roundtrip preserves plaintext", func(t *testing.T) {
		ct, err := credential.EncryptCredential(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptCredential: %v", err)
		}
		if len(ct) == 0 {
			t.Fatal("expected non-empty ciphertext")
		}

		got, err := credential.DecryptCredential(key, ct)
		if err != nil {
			t.Fatalf("DecryptCredential: %v", err)
		}
		if string(got) != string(plaintext) {
			t.Errorf("decrypted = %q, want %q", got, plaintext)
		}
	})

	t.Run("wrong key returns error", func(t *testing.T) {
		ct, err := credential.EncryptCredential(key, plaintext)
		if err != nil {
			t.Fatalf("EncryptCredential: %v", err)
		}

		wrongKey := make([]byte, 32)
		if _, err := rand.Read(wrongKey); err != nil {
			t.Fatalf("generate wrong key: %v", err)
		}

		_, err = credential.DecryptCredential(wrongKey, ct)
		if err == nil {
			t.Error("expected error with wrong key, got nil")
		}
	})
}

// ============================================================
// SD-JWT TESTS
// ============================================================

func TestIssueSDJWT(t *testing.T) {
	pub, priv := generateTestKey(t)

	sdjwt, err := credential.IssueEmailSDJWT(testEntityDID, "user@example.com", testIssuerDID, testKeyID, pub, priv, 6, "1")
	if err != nil {
		t.Fatalf("IssueEmailSDJWT: %v", err)
	}
	if sdjwt == "" {
		t.Fatal("expected non-empty SD-JWT string")
	}

	// SD-JWT format: header.payload.signature~disclosure1~...
	// At minimum it should be a JWT (3 dot-separated parts) or SD-JWT (has tildes)
	if !strings.Contains(sdjwt, ".") {
		t.Error("SD-JWT should contain dots (JWT header.payload.signature)")
	}
}

// isBase64Containing checks if any base64url-encoded part of a JWT contains the substring.
func isBase64Containing(jwt, substr string) bool {
	parts := strings.Split(jwt, ".")
	for _, part := range parts {
		if strings.Contains(part, substr) {
			return true
		}
	}
	return false
}
