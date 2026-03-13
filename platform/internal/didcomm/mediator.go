package didcomm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// jweRecipientPeek is a minimal struct for extracting the recipient KID
// from a JWE JSON serialization without decrypting.
type jweRecipientPeek struct {
	Recipients []struct {
		Header struct {
			KID string `json:"kid"`
		} `json:"header"`
	} `json:"recipients"`
	Protected string `json:"protected"`
}

// ExtractRecipientKID parses JWE JSON bytes and returns the first recipient's KID.
// The KID is a DID fragment like: did:web:atap.app:agent:01xxx#key-x25519-1
// Returns an error if no recipients are present or the KID is missing.
func ExtractRecipientKID(jweBytes []byte) (string, error) {
	var peek jweRecipientPeek
	if err := json.Unmarshal(jweBytes, &peek); err != nil {
		return "", fmt.Errorf("extract recipient KID: unmarshal JWE: %w", err)
	}
	if len(peek.Recipients) == 0 {
		return "", errors.New("extract recipient KID: JWE has no recipients")
	}
	kid := peek.Recipients[0].Header.KID
	if kid == "" {
		return "", errors.New("extract recipient KID: recipients[0].header.kid is missing")
	}
	return kid, nil
}

// jweProtectedPeek is a minimal struct for extracting the skid from the protected header.
type jweProtectedPeek struct {
	SKID string `json:"skid"`
}

// ExtractSenderKID parses JWE JSON bytes, decodes the protected header (base64url),
// and returns the skid field. Returns an error if the field is absent or unparseable.
func ExtractSenderKID(jweBytes []byte) (string, error) {
	var peek jweRecipientPeek
	if err := json.Unmarshal(jweBytes, &peek); err != nil {
		return "", fmt.Errorf("extract sender KID: unmarshal JWE: %w", err)
	}
	if peek.Protected == "" {
		return "", errors.New("extract sender KID: JWE missing protected header")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(peek.Protected)
	if err != nil {
		return "", fmt.Errorf("extract sender KID: decode protected header: %w", err)
	}
	var header jweProtectedPeek
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", fmt.Errorf("extract sender KID: parse protected header: %w", err)
	}
	if header.SKID == "" {
		return "", errors.New("extract sender KID: skid field missing in protected header")
	}
	return header.SKID, nil
}

// ValidateRecipientDomain validates that the DID in the recipient KID belongs to this server.
// The KID is a DID fragment like: did:web:atap.app:agent:01xxx#key-x25519-1
// The DID method is did:web, so the domain is the third segment after "did:web:".
//
// This prevents the server from accepting messages for foreign DIDs (anti-forwarding).
// Returns true if the DID's domain component matches platformDomain.
func ValidateRecipientDomain(recipientKID, platformDomain string) bool {
	// KID format: did:web:{domain}:{path...}#{fragment}
	// Strip fragment first.
	did := recipientKID
	if idx := strings.Index(did, "#"); idx >= 0 {
		did = did[:idx]
	}

	// Parse did:web:{domain}
	parts := strings.SplitN(did, ":", 4)
	if len(parts) < 3 {
		return false
	}
	if parts[0] != "did" || parts[1] != "web" {
		return false
	}

	// parts[2] is the domain (may include port as %3A per did:web spec, but we keep it simple).
	domain := parts[2]
	return domain == platformDomain
}
