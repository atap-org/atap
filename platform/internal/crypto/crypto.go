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

// NewApprovalID generates an approval ID using "apr_" + ULID (lowercase).
func NewApprovalID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return "apr_" + strings.ToLower(id.String())
}

// NewKeyID generates a key identifier with the given prefix.
func NewKeyID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("key_%s_%x", prefix, b)
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

// NewClaimID generates a claim ID using "clm_" + 12 hex chars.
func NewClaimID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("clm_%x", b)
}

// NewClaimCode generates a short claim code like "ATAP-7X9K".
// 4 alphanumeric characters, uppercase, easy to read aloud.
func NewClaimCode() string {
	const charset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ" // no I, O (ambiguous)
	b := make([]byte, 4)
	rand.Read(b)
	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[int(b[i])%len(charset)]
	}
	return "ATAP-" + string(code)
}

// EncodePublicKey encodes a public key as base64 standard encoding.
func EncodePublicKey(key ed25519.PublicKey) string {
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
