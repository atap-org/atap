package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gowebpki/jcs"
	"github.com/oklog/ulid/v2"
)

// GenerateKeyPair creates a new Ed25519 keypair.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// Sign signs data with an Ed25519 private key.
func Sign(privateKey ed25519.PrivateKey, data []byte) []byte {
	return ed25519.Sign(privateKey, data)
}

// Verify verifies an Ed25519 signature.
func Verify(publicKey ed25519.PublicKey, data, sig []byte) bool {
	return ed25519.Verify(publicKey, data, sig)
}

// DeriveHumanID derives a human entity ID from an Ed25519 public key.
// Formula: lowercase(base32(sha256(pubkey))[:16])
func DeriveHumanID(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return strings.ToLower(encoded[:16])
}

// NewEntityID generates a random entity ID using ULID (lowercase).
func NewEntityID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return strings.ToLower(id.String())
}

// NewChannelID generates a channel ID with "chn_" prefix and 128-bit entropy (32 hex chars).
func NewChannelID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("chn_%x", b)
}

// NewKeyID generates a key identifier with the given prefix.
func NewKeyID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("key_%s_%x", prefix, b)
}

// NewToken generates a bearer token: "atap_" + 32 random bytes base64url.
// Returns the token string and its SHA-256 hash (for storage).
func NewToken() (token string, hash []byte) {
	raw := make([]byte, 32)
	rand.Read(raw)
	token = "atap_" + base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(token))
	hash = h[:]
	return
}

// HashToken returns the SHA-256 hash of a token (for lookup).
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CanonicalJSON produces RFC 8785 (JCS) compliant canonical JSON for signing.
// Keys are sorted lexicographically, no extra whitespace, floats per ECMAScript spec.
func CanonicalJSON(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for canonical JSON: %w", err)
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("JCS transform: %w", err)
	}
	return canonical, nil
}

// SignablePayload creates the signable payload from route + signal blocks.
// Format: canonical(route) + "." + canonical(signal)
func SignablePayload(route, signal interface{}) ([]byte, error) {
	r, err := CanonicalJSON(route)
	if err != nil {
		return nil, fmt.Errorf("canonical route: %w", err)
	}
	s, err := CanonicalJSON(signal)
	if err != nil {
		return nil, fmt.Errorf("canonical signal: %w", err)
	}
	result := make([]byte, 0, len(r)+1+len(s))
	result = append(result, r...)
	result = append(result, '.')
	result = append(result, s...)
	return result, nil
}

// EncodePublicKey encodes a public key as base64 standard encoding.
func EncodePublicKey(key ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

// EncodePrivateKey encodes a private key as base64 standard encoding.
func EncodePrivateKey(key ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodePublicKey decodes a base64-encoded public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, want %d", len(b), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}
