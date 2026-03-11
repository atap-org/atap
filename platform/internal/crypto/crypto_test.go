package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateKeyPair(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("public key size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
}

func TestSign_Verify(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	data := []byte("hello atap")
	sig := Sign(priv, data)

	t.Run("valid signature", func(t *testing.T) {
		if !Verify(pub, data, sig) {
			t.Error("Verify() returned false for valid signature")
		}
	})

	t.Run("wrong data", func(t *testing.T) {
		if Verify(pub, []byte("wrong data"), sig) {
			t.Error("Verify() returned true for wrong data")
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		pub2, _, _ := GenerateKeyPair()
		if Verify(pub2, data, sig) {
			t.Error("Verify() returned true for wrong key")
		}
	})
}

func TestCanonicalJSON(t *testing.T) {
	t.Run("sorted keys", func(t *testing.T) {
		input := map[string]interface{}{
			"zebra": 1,
			"alpha": 2,
			"middle": map[string]interface{}{
				"z": true,
				"a": false,
			},
		}
		result, err := CanonicalJSON(input)
		if err != nil {
			t.Fatalf("CanonicalJSON() error: %v", err)
		}

		// RFC 8785 / JCS: keys sorted lexicographically, no whitespace
		expected := `{"alpha":2,"middle":{"a":false,"z":true},"zebra":1}`
		if string(result) != expected {
			t.Errorf("CanonicalJSON() = %s, want %s", string(result), expected)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		input := map[string]interface{}{"b": 2, "a": 1}
		r1, _ := CanonicalJSON(input)
		r2, _ := CanonicalJSON(input)
		if !bytes.Equal(r1, r2) {
			t.Error("CanonicalJSON() not deterministic")
		}
	})
}

func TestCanonicalJSON_FloatHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		contains string
	}{
		{
			name:     "large number",
			input:    map[string]interface{}{"n": 1e20},
			contains: "100000000000000000000",
		},
		{
			name:     "integer-valued float",
			input:    map[string]interface{}{"n": 10.0},
			contains: "10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CanonicalJSON(tc.input)
			if err != nil {
				t.Fatalf("CanonicalJSON() error: %v", err)
			}
			if !strings.Contains(string(result), tc.contains) {
				t.Errorf("CanonicalJSON() = %s, want to contain %s", string(result), tc.contains)
			}
		})
	}
}

func TestSignablePayload(t *testing.T) {
	route := map[string]string{"origin": "agent://abc", "target": "agent://def"}
	signal := map[string]interface{}{"type": "text", "data": "hello"}

	result, err := SignablePayload(route, signal)
	if err != nil {
		t.Fatalf("SignablePayload() error: %v", err)
	}

	// Verify format: JCS(route) + "." + JCS(signal)
	parts := strings.SplitN(string(result), ".", 2)
	if len(parts) != 2 {
		t.Fatalf("SignablePayload() does not contain dot separator, got: %s", string(result))
	}

	// Each part should be valid JSON
	var r, s json.RawMessage
	if err := json.Unmarshal([]byte(parts[0]), &r); err != nil {
		t.Errorf("route part not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(parts[1]), &s); err != nil {
		t.Errorf("signal part not valid JSON: %v", err)
	}
}

func TestSignRequest_VerifyRequest(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}
	keyID := "key_test_abcd1234"

	t.Run("valid signature", func(t *testing.T) {
		ts := time.Now().UTC()
		authHeader := SignRequest(priv, keyID, "GET", "/v1/me", ts)

		err := VerifyRequest(pub, authHeader, "GET", "/v1/me", ts.Format(time.RFC3339))
		if err != nil {
			t.Errorf("VerifyRequest() error: %v", err)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		ts := time.Now().UTC()
		authHeader := SignRequest(priv, keyID, "GET", "/v1/me", ts)

		err := VerifyRequest(pub, authHeader, "POST", "/v1/me", ts.Format(time.RFC3339))
		if err == nil {
			t.Error("VerifyRequest() should fail for wrong method")
		}
	})

	t.Run("wrong path", func(t *testing.T) {
		ts := time.Now().UTC()
		authHeader := SignRequest(priv, keyID, "GET", "/v1/me", ts)

		err := VerifyRequest(pub, authHeader, "GET", "/v1/other", ts.Format(time.RFC3339))
		if err == nil {
			t.Error("VerifyRequest() should fail for wrong path")
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		pub2, _, _ := GenerateKeyPair()
		ts := time.Now().UTC()
		authHeader := SignRequest(priv, keyID, "GET", "/v1/me", ts)

		err := VerifyRequest(pub2, authHeader, "GET", "/v1/me", ts.Format(time.RFC3339))
		if err == nil {
			t.Error("VerifyRequest() should fail for wrong key")
		}
	})

	t.Run("expired timestamp", func(t *testing.T) {
		ts := time.Now().UTC().Add(-10 * time.Minute) // 10 min ago, beyond 5 min skew
		authHeader := SignRequest(priv, keyID, "GET", "/v1/me", ts)

		err := VerifyRequest(pub, authHeader, "GET", "/v1/me", ts.Format(time.RFC3339))
		if err == nil {
			t.Error("VerifyRequest() should fail for expired timestamp")
		}
	})
}

func TestParseSignatureHeader(t *testing.T) {
	t.Run("valid header", func(t *testing.T) {
		header := `Signature keyId="key_test_1234",algorithm="ed25519",headers="(request-target) x-atap-timestamp",signature="abc123"`
		keyID, sig, err := ParseSignatureHeader(header)
		if err != nil {
			t.Fatalf("ParseSignatureHeader() error: %v", err)
		}
		if keyID != "key_test_1234" {
			t.Errorf("keyId = %q, want key_test_1234", keyID)
		}
		if sig != "abc123" {
			t.Errorf("signature = %q, want abc123", sig)
		}
	})

	t.Run("missing Signature prefix", func(t *testing.T) {
		_, _, err := ParseSignatureHeader("Bearer abc")
		if err == nil {
			t.Error("expected error for non-Signature header")
		}
	})

	t.Run("missing keyId", func(t *testing.T) {
		_, _, err := ParseSignatureHeader(`Signature algorithm="ed25519",signature="abc"`)
		if err == nil {
			t.Error("expected error for missing keyId")
		}
	})
}

func TestNewEntityID(t *testing.T) {
	t.Run("format", func(t *testing.T) {
		id := NewEntityID()
		if len(id) != 26 {
			t.Errorf("NewEntityID() length = %d, want 26", len(id))
		}
		if id != strings.ToLower(id) {
			t.Errorf("NewEntityID() not lowercase: %s", id)
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		id1 := NewEntityID()
		id2 := NewEntityID()
		if id1 == id2 {
			t.Error("NewEntityID() produced duplicate IDs")
		}
	})
}

func TestNewChannelID(t *testing.T) {
	t.Run("prefix", func(t *testing.T) {
		id := NewChannelID()
		if !strings.HasPrefix(id, "chn_") {
			t.Errorf("NewChannelID() prefix = %s, want chn_", id[:4])
		}
	})

	t.Run("hex length", func(t *testing.T) {
		id := NewChannelID()
		hexPart := strings.TrimPrefix(id, "chn_")
		if len(hexPart) != 32 {
			t.Errorf("NewChannelID() hex part length = %d, want 32 (128-bit entropy), got %s", len(hexPart), id)
		}
		// Verify it's valid hex
		if _, err := hex.DecodeString(hexPart); err != nil {
			t.Errorf("NewChannelID() hex part not valid hex: %v", err)
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		id1 := NewChannelID()
		id2 := NewChannelID()
		if id1 == id2 {
			t.Error("NewChannelID() produced duplicate IDs")
		}
	})
}

func TestNewKeyID(t *testing.T) {
	id := NewKeyID("ed25519")
	if !strings.HasPrefix(id, "key_ed25519_") {
		t.Errorf("NewKeyID() = %s, want prefix key_ed25519_", id)
	}
	hexPart := strings.TrimPrefix(id, "key_ed25519_")
	if len(hexPart) != 8 {
		t.Errorf("NewKeyID() hex part length = %d, want 8", len(hexPart))
	}
}

func TestEncodeDecodePublicKey(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	encoded := EncodePublicKey(pub)
	decoded, err := DecodePublicKey(encoded)
	if err != nil {
		t.Fatalf("DecodePublicKey() error: %v", err)
	}

	if !bytes.Equal(pub, decoded) {
		t.Error("public key encode/decode round-trip failed")
	}
}

func TestEncodePrivateKey(t *testing.T) {
	_, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	encoded := EncodePrivateKey(priv)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("EncodePrivateKey() not valid base64: %v", err)
	}

	// Should round-trip
	if !bytes.Equal(priv, decoded) {
		t.Error("EncodePrivateKey() round-trip failed")
	}
}

func TestDeriveHumanID(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	t.Run("format", func(t *testing.T) {
		id := DeriveHumanID(pub)
		if len(id) != 16 {
			t.Errorf("DeriveHumanID() length = %d, want 16", len(id))
		}
		if id != strings.ToLower(id) {
			t.Errorf("DeriveHumanID() not lowercase: %s", id)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := DeriveHumanID(pub)
		id2 := DeriveHumanID(pub)
		if id1 != id2 {
			t.Error("DeriveHumanID() not deterministic")
		}
	})

	t.Run("different keys produce different IDs", func(t *testing.T) {
		pub2, _, _ := GenerateKeyPair()
		id1 := DeriveHumanID(pub)
		id2 := DeriveHumanID(pub2)
		if id1 == id2 {
			t.Error("DeriveHumanID() produced same ID for different keys")
		}
	})
}

// TestDeriveHumanIDKnownVector uses a fixed seed to produce a deterministic
// public key and verifies the human ID against a known value. This test vector
// is shared with the Dart implementation in mobile/test/crypto_compat_test.dart.
func TestDeriveHumanIDKnownVector(t *testing.T) {
	// Fixed 32-byte seed: 0x00..0x1f
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	expectedPubHex := "03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8"
	actualPubHex := hex.EncodeToString(publicKey)
	if actualPubHex != expectedPubHex {
		t.Fatalf("public key mismatch: got %s, want %s", actualPubHex, expectedPubHex)
	}

	expectedHumanID := "kzdvvj2umnduyauf"
	actualHumanID := DeriveHumanID(publicKey)
	if actualHumanID != expectedHumanID {
		t.Fatalf("DeriveHumanID() = %q, want %q", actualHumanID, expectedHumanID)
	}
}

// TestSignRequestKnownVector verifies request signing against a known payload
// and seed. Shared with the Dart implementation for cross-language validation.
func TestSignRequestKnownVector(t *testing.T) {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	// Sign a known payload
	payload := "GET /v1/health 2024-01-01T00:00:00Z"
	sig := ed25519.Sign(privateKey, []byte(payload))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	expectedSig := "1ERwmMB-ThYieQXMTZ4naGuIvroq9kYQ6Jn2TV7OGrSCmoWrmG2ThsteyTL98zzR2bAkPD2GLW0F1I7aE17sBg"
	if sigB64 != expectedSig {
		t.Fatalf("signature mismatch:\ngot  %s\nwant %s", sigB64, expectedSig)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, []byte(payload), sig) {
		t.Fatal("signature verification failed")
	}
}
