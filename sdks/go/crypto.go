package atap

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/8upio/atap/sdks/go/internal/uuidgen"
)

// B64URLEncode encodes data as base64url without padding.
func B64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// B64URLDecode decodes a base64url string (with or without padding).
func B64URLDecode(s string) ([]byte, error) {
	// Strip padding if present.
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

// GenerateKeypair generates a new Ed25519 keypair.
func GenerateKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate keypair: %w", err)
	}
	return pub, priv, nil
}

// LoadSigningKey loads an Ed25519 private key from a base64 string.
// It accepts either a 32-byte seed or a 64-byte full key.
func LoadSigningKey(b64 string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		// Try base64url
		raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(b64, "="))
		if err != nil {
			return nil, fmt.Errorf("decode private key: %w", err)
		}
	}
	switch len(raw) {
	case 64:
		return ed25519.PrivateKey(raw), nil
	case 32:
		return ed25519.NewKeyFromSeed(raw), nil
	default:
		return nil, fmt.Errorf("invalid private key length: %d bytes (expected 32 or 64)", len(raw))
	}
}

// JWKThumbprint computes the JWK thumbprint (RFC 7638) for an Ed25519 public key.
func JWKThumbprint(pub ed25519.PublicKey) string {
	x := B64URLEncode([]byte(pub))
	// Canonical JSON with sorted keys.
	canonical := fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`, x)
	h := sha256.Sum256([]byte(canonical))
	return B64URLEncode(h[:])
}

// MakeDPoPProof creates a DPoP proof JWT (RFC 9449).
//
// The url parameter should use https://{platformDomain}/path for the htu claim.
// If accessToken is non-empty, an ath (access token hash) claim is included.
func MakeDPoPProof(privKey ed25519.PrivateKey, method, url string, accessToken string) string {
	pub := privKey.Public().(ed25519.PublicKey)
	x := B64URLEncode([]byte(pub))

	header := map[string]interface{}{
		"typ": "dpop+jwt",
		"alg": "EdDSA",
		"jwk": map[string]string{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   x,
		},
	}

	payload := map[string]interface{}{
		"jti": uuidgen.New(),
		"htm": method,
		"htu": url,
		"iat": time.Now().Unix(),
	}

	if accessToken != "" {
		h := sha256.Sum256([]byte(accessToken))
		payload["ath"] = B64URLEncode(h[:])
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := B64URLEncode(headerJSON)
	payloadB64 := B64URLEncode(payloadJSON)

	signingInput := []byte(headerB64 + "." + payloadB64)
	sig := ed25519.Sign(privKey, signingInput)
	sigB64 := B64URLEncode(sig)

	return headerB64 + "." + payloadB64 + "." + sigB64
}

// GeneratePKCE generates a PKCE code verifier and S256 challenge.
func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate PKCE: %w", err)
	}
	verifier = B64URLEncode(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = B64URLEncode(h[:])
	return verifier, challenge, nil
}

// DomainFromDID extracts the platform domain from a DID.
//
// Example: did:web:localhost%3A8080:agent:abc -> localhost:8080
func DomainFromDID(did string) (string, error) {
	parts := strings.Split(did, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid DID format: %s", did)
	}
	domain := parts[2]
	domain = strings.ReplaceAll(domain, "%3A", ":")
	return domain, nil
}
