package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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

func TestNewToken(t *testing.T) {
	t.Run("prefix", func(t *testing.T) {
		token, _ := NewToken()
		if !strings.HasPrefix(token, "atap_") {
			t.Errorf("NewToken() prefix = %s, want atap_", token[:5])
		}
	})

	t.Run("length", func(t *testing.T) {
		token, _ := NewToken()
		// 5 prefix + 43 base64url chars (32 bytes)
		if len(token) < 45 || len(token) > 50 {
			t.Errorf("NewToken() length = %d, want ~48", len(token))
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		t1, _ := NewToken()
		t2, _ := NewToken()
		if t1 == t2 {
			t.Error("NewToken() produced duplicate tokens")
		}
	})
}

func TestHashToken(t *testing.T) {
	token, _ := NewToken()
	hash := HashToken(token)

	if len(hash) != 32 {
		t.Errorf("HashToken() length = %d, want 32", len(hash))
	}

	// Same token, same hash
	hash2 := HashToken(token)
	if !bytes.Equal(hash, hash2) {
		t.Error("HashToken() not deterministic")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	token, hash := NewToken()
	computed := HashToken(token)
	if !bytes.Equal(hash, computed) {
		t.Errorf("token round-trip failed: NewToken hash != HashToken(token)")
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
