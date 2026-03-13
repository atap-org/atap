package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
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
// public key and verifies the human ID against a known value.
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
