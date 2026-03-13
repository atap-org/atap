package didcomm_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/atap-dev/atap/platform/internal/didcomm"
)

// TestEnvelopeRoundTrip tests that Encrypt followed by Decrypt returns the original plaintext.
func TestEnvelopeRoundTrip(t *testing.T) {
	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}

	recipientPriv, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	plaintext := []byte(`{"id":"msg_test","type":"https://atap.dev/protocols/approval/1.0/request","body":{"action":"approve"}}`)

	jwe, err := didcomm.Encrypt(plaintext, senderPriv, senderPub, recipientPub, "did:web:atap.app:agent:sender#key-x25519-1", "did:web:atap.app:agent:recipient#key-x25519-1")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if len(jwe) == 0 {
		t.Fatal("expected non-empty JWE bytes")
	}

	decrypted, err := didcomm.Decrypt(jwe, recipientPriv, senderPub)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip failed:\nwant: %s\ngot:  %s", plaintext, decrypted)
	}
}

// TestEnvelopeJWEStructure verifies the JWE output has the correct JSON structure and headers.
func TestEnvelopeJWEStructure(t *testing.T) {
	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}

	_, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	senderKID := "did:web:atap.app:agent:sender#key-x25519-1"
	recipientKID := "did:web:atap.app:agent:recipient#key-x25519-1"

	jwe, err := didcomm.Encrypt([]byte("hello world"), senderPriv, senderPub, recipientPub, senderKID, recipientKID)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Parse JWE JSON
	var jweObj map[string]any
	if err := json.Unmarshal(jwe, &jweObj); err != nil {
		t.Fatalf("JWE is not valid JSON: %v", err)
	}

	// Check required fields exist
	for _, field := range []string{"protected", "recipients", "iv", "ciphertext", "tag"} {
		if _, ok := jweObj[field]; !ok {
			t.Errorf("JWE missing required field: %q", field)
		}
	}

	// Decode and check protected header
	protectedB64, ok := jweObj["protected"].(string)
	if !ok {
		t.Fatal("protected header is not a string")
	}

	protectedBytes, err := base64.RawURLEncoding.DecodeString(protectedB64)
	if err != nil {
		t.Fatalf("protected header is not valid base64url: %v", err)
	}

	var header map[string]any
	if err := json.Unmarshal(protectedBytes, &header); err != nil {
		t.Fatalf("protected header is not valid JSON: %v", err)
	}

	if header["alg"] != "ECDH-1PU+A256KW" {
		t.Errorf("expected alg=ECDH-1PU+A256KW, got %v", header["alg"])
	}
	if header["enc"] != "A256CBC-HS512" {
		t.Errorf("expected enc=A256CBC-HS512, got %v", header["enc"])
	}
	if header["skid"] != senderKID {
		t.Errorf("expected skid=%s, got %v", senderKID, header["skid"])
	}

	// Check epk is present and has correct fields
	epk, ok := header["epk"].(map[string]any)
	if !ok {
		t.Fatalf("expected epk to be a map, got %T", header["epk"])
	}
	if epk["kty"] != "OKP" {
		t.Errorf("expected epk.kty=OKP, got %v", epk["kty"])
	}
	if epk["crv"] != "X25519" {
		t.Errorf("expected epk.crv=X25519, got %v", epk["crv"])
	}
	if epk["x"] == nil || epk["x"] == "" {
		t.Errorf("expected epk.x to be set")
	}

	// Check recipients[0].header.kid
	recipients, ok := jweObj["recipients"].([]any)
	if !ok || len(recipients) == 0 {
		t.Fatal("expected non-empty recipients array")
	}

	recipientObj, ok := recipients[0].(map[string]any)
	if !ok {
		t.Fatal("recipient entry is not a map")
	}

	recipientHeader, ok := recipientObj["header"].(map[string]any)
	if !ok {
		t.Fatal("recipient header is not a map")
	}

	if recipientHeader["kid"] != recipientKID {
		t.Errorf("expected recipients[0].header.kid=%s, got %v", recipientKID, recipientHeader["kid"])
	}

	// Check encrypted_key is present and non-empty
	encKey, ok := recipientObj["encrypted_key"].(string)
	if !ok || encKey == "" {
		t.Errorf("expected non-empty encrypted_key, got %v", recipientObj["encrypted_key"])
	}
}

// TestEnvelopeWrongRecipientKey verifies that decryption fails with wrong recipient key.
func TestEnvelopeWrongRecipientKey(t *testing.T) {
	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}

	_, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	// A different, wrong private key
	wrongPriv, _, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate wrong keypair: %v", err)
	}

	jwe, err := didcomm.Encrypt([]byte("secret"), senderPriv, senderPub, recipientPub, "sender-kid", "recipient-kid")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = didcomm.Decrypt(jwe, wrongPriv, senderPub)
	if err == nil {
		t.Error("expected error when decrypting with wrong recipient key, got nil")
	}
}

// TestEnvelopeWrongSenderKey verifies that decryption fails with wrong sender public key.
func TestEnvelopeWrongSenderKey(t *testing.T) {
	senderPriv, senderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate sender keypair: %v", err)
	}

	recipientPriv, recipientPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate recipient keypair: %v", err)
	}

	// A different, wrong sender public key
	_, wrongSenderPub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("generate wrong sender keypair: %v", err)
	}

	jwe, err := didcomm.Encrypt([]byte("secret"), senderPriv, senderPub, recipientPub, "sender-kid", "recipient-kid")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = didcomm.Decrypt(jwe, recipientPriv, wrongSenderPub)
	if err == nil {
		t.Error("expected error when decrypting with wrong sender public key, got nil")
	}
}

// TestEnvelopeMultiplePlaintexts tests encryption of various payloads including empty body.
func TestEnvelopeMultiplePlaintexts(t *testing.T) {
	senderPriv, senderPub, _ := didcomm.GenerateX25519KeyPair()
	recipientPriv, recipientPub, _ := didcomm.GenerateX25519KeyPair()

	cases := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"single byte", []byte{0x42}},
		{"json message", []byte(`{"id":"msg_abc","type":"ping","body":{}}`)},
		{"large payload", []byte(strings.Repeat("A", 4096))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jwe, err := didcomm.Encrypt(tc.plaintext, senderPriv, senderPub, recipientPub, "sender", "recipient")
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}

			decrypted, err := didcomm.Decrypt(jwe, recipientPriv, senderPub)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}

			if string(decrypted) != string(tc.plaintext) {
				t.Errorf("round-trip mismatch for %q", tc.name)
			}
		})
	}
}

// TestGenerateX25519KeyPair verifies key pair generation produces valid keys.
func TestGenerateX25519KeyPair(t *testing.T) {
	priv, pub, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateX25519KeyPair: %v", err)
	}
	if priv == nil {
		t.Error("expected non-nil private key")
	}
	if pub == nil {
		t.Error("expected non-nil public key")
	}

	// Key bytes should be 32 bytes for X25519
	if len(priv.Bytes()) != 32 {
		t.Errorf("expected 32-byte private key, got %d", len(priv.Bytes()))
	}
	if len(pub.Bytes()) != 32 {
		t.Errorf("expected 32-byte public key, got %d", len(pub.Bytes()))
	}

	// Two generated keys should be different
	priv2, pub2, err := didcomm.GenerateX25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateX25519KeyPair: %v", err)
	}
	if string(priv.Bytes()) == string(priv2.Bytes()) {
		t.Error("generated identical private keys")
	}
	if string(pub.Bytes()) == string(pub2.Bytes()) {
		t.Error("generated identical public keys")
	}
}
