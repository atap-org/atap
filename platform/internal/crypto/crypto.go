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

// NewSignalID generates a signal ID with "sig_" prefix and ULID (lowercase).
func NewSignalID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return "sig_" + strings.ToLower(id.String())
}

// NewDeliveryAttemptID generates a delivery attempt ID using ULID (lowercase).
func NewDeliveryAttemptID() string {
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

// MaxTimestampSkew is the maximum allowed difference between client timestamp
// and server time for signed request verification.
const MaxTimestampSkew = 5 * time.Minute

// SignRequest signs an HTTP request for Ed25519 authentication.
// It produces the value for the Authorization header.
// The signed payload is: method + " " + path + " " + timestamp (RFC3339).
func SignRequest(privateKey ed25519.PrivateKey, keyID, method, path string, ts time.Time) string {
	timestamp := ts.UTC().Format(time.RFC3339)
	payload := method + " " + path + " " + timestamp
	sig := ed25519.Sign(privateKey, []byte(payload))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return fmt.Sprintf(`Signature keyId="%s",algorithm="ed25519",headers="(request-target) x-atap-timestamp",signature="%s"`, keyID, sigB64)
}

// VerifyRequest verifies an Ed25519 signed request.
// Returns the keyID extracted from the Authorization header if valid, or an error.
// The caller is responsible for looking up the public key by keyID.
func VerifyRequest(publicKey ed25519.PublicKey, authHeader, method, path, timestamp string) error {
	// Parse Authorization header
	keyID, sigB64, err := ParseSignatureHeader(authHeader)
	if err != nil {
		return err
	}
	_ = keyID // caller already looked up the key

	// Verify timestamp is within skew
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	diff := time.Since(ts)
	if diff < 0 {
		diff = -diff
	}
	if diff > MaxTimestampSkew {
		return fmt.Errorf("timestamp skew too large: %v", diff)
	}

	// Verify signature
	payload := method + " " + path + " " + timestamp
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(publicKey, []byte(payload), sig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// ParseSignatureHeader extracts keyId and signature from an Authorization header
// of the form: Signature keyId="...",algorithm="ed25519",headers="...",signature="..."
func ParseSignatureHeader(header string) (keyID, signature string, err error) {
	if !strings.HasPrefix(header, "Signature ") {
		return "", "", fmt.Errorf("invalid Authorization format, expected Signature scheme")
	}
	params := header[len("Signature "):]

	// Parse key-value pairs
	fields := make(map[string]string)
	for _, part := range splitParams(params) {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx < 0 {
			continue
		}
		key := part[:eqIdx]
		val := strings.Trim(part[eqIdx+1:], `"`)
		fields[key] = val
	}

	keyID, ok := fields["keyId"]
	if !ok || keyID == "" {
		return "", "", fmt.Errorf("missing keyId in Signature header")
	}
	signature, ok = fields["signature"]
	if !ok || signature == "" {
		return "", "", fmt.Errorf("missing signature in Signature header")
	}
	return keyID, signature, nil
}

// splitParams splits comma-separated params, respecting quoted values.
func splitParams(s string) []string {
	var parts []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
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
