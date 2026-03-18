package atap

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestB64URLEncode(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte{}, ""},
		{[]byte{0, 1, 2, 3}, "AAECAw"},
		{[]byte{255, 254, 253}, "__79"},
	}
	for _, tt := range tests {
		got := B64URLEncode(tt.input)
		if got != tt.want {
			t.Errorf("B64URLEncode(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestB64URLDecode(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantErr bool
	}{
		{"AAECAw", 4, false},
		{"__79", 3, false},
		{"AAECAw==", 4, false}, // with padding
		{"!!!invalid!!!", 0, true},
	}
	for _, tt := range tests {
		got, err := B64URLDecode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("B64URLDecode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if err == nil && len(got) != tt.wantLen {
			t.Errorf("B64URLDecode(%q) len = %d, want %d", tt.input, len(got), tt.wantLen)
		}
	}
}

func TestB64URLRoundTrip(t *testing.T) {
	data := []byte("hello, ATAP protocol!")
	encoded := B64URLEncode(data)
	decoded, err := B64URLDecode(encoded)
	if err != nil {
		t.Fatalf("B64URLDecode: %v", err)
	}
	if string(decoded) != string(data) {
		t.Errorf("round trip failed: got %q, want %q", decoded, data)
	}
}

func TestGenerateKeypair(t *testing.T) {
	pub, priv, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("public key length = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key length = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}

	// Verify signing works.
	msg := []byte("test message")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(pub, msg, sig) {
		t.Error("generated keypair cannot sign/verify")
	}
}

func TestLoadSigningKey_Seed(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	seed := priv.Seed()
	b64 := base64.StdEncoding.EncodeToString(seed)

	loaded, err := LoadSigningKey(b64)
	if err != nil {
		t.Fatalf("LoadSigningKey (seed): %v", err)
	}

	msg := []byte("test")
	sig := ed25519.Sign(loaded, msg)
	pub := priv.Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, msg, sig) {
		t.Error("loaded key (from seed) does not match original")
	}
}

func TestLoadSigningKey_FullKey(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	b64 := base64.StdEncoding.EncodeToString(priv)

	loaded, err := LoadSigningKey(b64)
	if err != nil {
		t.Fatalf("LoadSigningKey (full): %v", err)
	}

	msg := []byte("test")
	sig := ed25519.Sign(loaded, msg)
	pub := priv.Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, msg, sig) {
		t.Error("loaded key (full) does not match original")
	}
}

func TestLoadSigningKey_Invalid(t *testing.T) {
	// Invalid length.
	b64 := base64.StdEncoding.EncodeToString([]byte("too short"))
	_, err := LoadSigningKey(b64)
	if err == nil {
		t.Error("expected error for invalid key length")
	}

	// Invalid base64.
	_, err = LoadSigningKey("!!!not-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestJWKThumbprint(t *testing.T) {
	pub, _, _ := GenerateKeypair()
	tp := JWKThumbprint(pub)
	if tp == "" {
		t.Error("JWKThumbprint returned empty string")
	}

	// Verify it's a valid base64url string.
	decoded, err := B64URLDecode(tp)
	if err != nil {
		t.Fatalf("JWKThumbprint is not valid base64url: %v", err)
	}
	// SHA-256 produces 32 bytes.
	if len(decoded) != 32 {
		t.Errorf("JWKThumbprint decoded length = %d, want 32", len(decoded))
	}

	// Same key should produce same thumbprint.
	tp2 := JWKThumbprint(pub)
	if tp != tp2 {
		t.Error("JWKThumbprint not deterministic")
	}
}

func TestMakeDPoPProof(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	proof := MakeDPoPProof(priv, "POST", "https://example.com/v1/oauth/token", "")

	parts := strings.Split(proof, ".")
	if len(parts) != 3 {
		t.Fatalf("DPoP proof has %d parts, want 3", len(parts))
	}

	// Decode header.
	headerJSON, err := B64URLDecode(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]interface{}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["typ"] != "dpop+jwt" {
		t.Errorf("header typ = %v, want dpop+jwt", header["typ"])
	}
	if header["alg"] != "EdDSA" {
		t.Errorf("header alg = %v, want EdDSA", header["alg"])
	}
	if _, ok := header["jwk"]; !ok {
		t.Error("header missing jwk")
	}

	// Decode payload.
	payloadJSON, err := B64URLDecode(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["htm"] != "POST" {
		t.Errorf("payload htm = %v, want POST", payload["htm"])
	}
	if payload["htu"] != "https://example.com/v1/oauth/token" {
		t.Errorf("payload htu = %v, want https://example.com/v1/oauth/token", payload["htu"])
	}
	if _, ok := payload["jti"]; !ok {
		t.Error("payload missing jti")
	}
	if _, ok := payload["iat"]; !ok {
		t.Error("payload missing iat")
	}

	// Verify signature.
	pub := priv.Public().(ed25519.PublicKey)
	signingInput := []byte(parts[0] + "." + parts[1])
	sigBytes, err := B64URLDecode(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(pub, signingInput, sigBytes) {
		t.Error("DPoP proof signature verification failed")
	}
}

func TestMakeDPoPProof_WithAccessToken(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	accessToken := "test-access-token"
	proof := MakeDPoPProof(priv, "GET", "https://example.com/v1/entities", accessToken)

	parts := strings.Split(proof, ".")
	payloadJSON, _ := B64URLDecode(parts[1])
	var payload map[string]interface{}
	json.Unmarshal(payloadJSON, &payload)

	ath, ok := payload["ath"]
	if !ok {
		t.Fatal("payload missing ath when access_token provided")
	}

	// Verify ath is SHA-256 of access token.
	expected := sha256.Sum256([]byte(accessToken))
	expectedB64 := B64URLEncode(expected[:])
	if ath != expectedB64 {
		t.Errorf("ath = %v, want %v", ath, expectedB64)
	}
}

func TestMakeDPoPProof_UniqueJTI(t *testing.T) {
	_, priv, _ := GenerateKeypair()
	proof1 := MakeDPoPProof(priv, "GET", "https://example.com/test", "")
	proof2 := MakeDPoPProof(priv, "GET", "https://example.com/test", "")

	parts1 := strings.Split(proof1, ".")
	parts2 := strings.Split(proof2, ".")

	p1, _ := B64URLDecode(parts1[1])
	p2, _ := B64URLDecode(parts2[1])

	var payload1, payload2 map[string]interface{}
	json.Unmarshal(p1, &payload1)
	json.Unmarshal(p2, &payload2)

	if payload1["jti"] == payload2["jti"] {
		t.Error("two DPoP proofs have same jti")
	}
}

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE: %v", err)
	}
	if verifier == "" {
		t.Error("verifier is empty")
	}
	if challenge == "" {
		t.Error("challenge is empty")
	}
	if verifier == challenge {
		t.Error("verifier and challenge should differ")
	}

	// Verify challenge is SHA-256 of verifier.
	h := sha256.Sum256([]byte(verifier))
	expectedChallenge := B64URLEncode(h[:])
	if challenge != expectedChallenge {
		t.Errorf("challenge = %q, want SHA256(%q) = %q", challenge, verifier, expectedChallenge)
	}

	// Two calls should produce different values.
	v2, c2, _ := GeneratePKCE()
	if verifier == v2 {
		t.Error("PKCE verifiers should be random")
	}
	if challenge == c2 {
		t.Error("PKCE challenges should differ")
	}
}

func TestDomainFromDID(t *testing.T) {
	tests := []struct {
		did     string
		want    string
		wantErr bool
	}{
		{"did:web:localhost%3A8080:agent:abc", "localhost:8080", false},
		{"did:web:example.com:human:xyz", "example.com", false},
		{"did:web:atap.dev:machine:m1", "atap.dev", false},
		{"invalid", "", true},
		{"a:b", "", true},
	}
	for _, tt := range tests {
		got, err := DomainFromDID(tt.did)
		if (err != nil) != tt.wantErr {
			t.Errorf("DomainFromDID(%q) error = %v, wantErr %v", tt.did, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("DomainFromDID(%q) = %q, want %q", tt.did, got, tt.want)
		}
	}
}
