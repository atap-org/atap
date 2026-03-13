// Package approval provides the domain logic for the ATAP approval engine:
// JWS signing/verification, lifecycle state machine, and template fetch.
package approval

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// approvalWithoutSignatures builds a map representation of the approval excluding
// the signatures field. This is the canonical payload for JCS/JWS signing per spec §8.8.
// IMPORTANT: Does NOT modify the original Approval struct.
func approvalWithoutSignatures(a *models.Approval) (map[string]any, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal approval: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal approval to map: %w", err)
	}
	delete(m, "signatures")
	return m, nil
}

// SignApproval produces a JWS Compact Serialization with detached payload per spec §8.8.
// The returned string has the format: header..signature (empty middle segment = detached).
// keyID must be a fully-qualified DID URL, e.g. "did:web:example.com:entities:abc#key-0".
func SignApproval(a *models.Approval, privateKey ed25519.PrivateKey, keyID string) (string, error) {
	m, err := approvalWithoutSignatures(a)
	if err != nil {
		return "", fmt.Errorf("sign approval: %w", err)
	}

	payload, err := crypto.CanonicalJSON(m)
	if err != nil {
		return "", fmt.Errorf("sign approval: canonical JSON: %w", err)
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.EdDSA, Key: privateKey},
		(&jose.SignerOptions{}).WithHeader("kid", keyID),
	)
	if err != nil {
		return "", fmt.Errorf("sign approval: create signer: %w", err)
	}

	jws, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("sign approval: sign: %w", err)
	}

	compact, err := jws.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("sign approval: compact serialize: %w", err)
	}

	// Detach payload: split header.payload.signature -> header..signature
	parts := strings.Split(compact, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("sign approval: unexpected compact JWS format")
	}
	return parts[0] + ".." + parts[2], nil
}

// VerifyApprovalSignature verifies a detached JWS compact signature against an approval.
// Per APR-12, it first extracts the kid from the JWS protected header and validates it
// matches expectedKID before performing the cryptographic verification.
func VerifyApprovalSignature(a *models.Approval, jwsToken string, expectedKID string, resolvedPubKey ed25519.PublicKey) error {
	// Step 1 (APR-12): Extract and validate kid from JWS protected header.
	parts := strings.Split(jwsToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("verify approval: malformed JWS compact: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("verify approval: decode JWS header: %w", err)
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return fmt.Errorf("verify approval: parse JWS header: %w", err)
	}

	headerKID, ok := header["kid"].(string)
	if !ok {
		return fmt.Errorf("verify approval: JWS header missing kid field")
	}
	if headerKID != expectedKID {
		return fmt.Errorf("kid mismatch: JWS header has %q, expected %q", headerKID, expectedKID)
	}

	// Step 2: Recompute canonical payload.
	m, err := approvalWithoutSignatures(a)
	if err != nil {
		return fmt.Errorf("verify approval: %w", err)
	}
	payload, err := crypto.CanonicalJSON(m)
	if err != nil {
		return fmt.Errorf("verify approval: canonical JSON: %w", err)
	}

	// Step 3: Re-attach payload (go-jose requires non-detached JWS for parsing).
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	attached := parts[0] + "." + encodedPayload + "." + parts[2]

	// Step 4: Parse the re-attached JWS.
	parsed, err := jose.ParseSigned(attached, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return fmt.Errorf("verify approval: parse JWS: %w", err)
	}

	// Step 5: Verify signature against resolved public key.
	if _, err := parsed.Verify(resolvedPubKey); err != nil {
		return fmt.Errorf("verify approval: signature verification failed: %w", err)
	}

	return nil
}
