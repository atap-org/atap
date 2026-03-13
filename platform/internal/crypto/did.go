package crypto

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/mr-tron/base58"

	"github.com/atap-dev/atap/platform/internal/models"
)

// BuildDID constructs a did:web DID for an entity.
// Format: did:web:{domain}:{entityType}:{entityID}
func BuildDID(domain, entityType, entityID string) string {
	return fmt.Sprintf("did:web:%s:%s:%s", domain, entityType, entityID)
}

// EncodePublicKeyMultibase encodes an Ed25519 public key in multibase format.
// Uses base58btc encoding with "z" prefix, as required by Ed25519VerificationKey2020.
func EncodePublicKeyMultibase(pub ed25519.PublicKey) string {
	return "z" + base58.Encode(pub)
}

// BuildDIDDocument constructs a W3C DID Document for an entity.
// Includes all key versions (for rotation history) with only the active key
// referenced in authentication and assertionMethod.
func BuildDIDDocument(entity *models.Entity, keyVersions []models.KeyVersion, domain string) *models.DIDDocument {
	doc := &models.DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
			"https://atap.dev/ns/v1",
		},
		ID:       entity.DID,
		ATAPType: entity.Type,
	}

	// Only agents have a principal
	if entity.Type == models.EntityTypeAgent && entity.PrincipalDID != "" {
		doc.ATAPPrincipal = entity.PrincipalDID
	}

	// Build verification methods from all key versions
	var activeMethods []string
	for _, kv := range keyVersions {
		vmID := fmt.Sprintf("%s#key-%d", entity.DID, kv.KeyIndex)
		vm := models.VerificationMethod{
			ID:                 vmID,
			Type:               "Ed25519VerificationKey2020",
			Controller:         entity.DID,
			PublicKeyMultibase: EncodePublicKeyMultibase(ed25519.PublicKey(kv.PublicKey)),
		}
		doc.VerificationMethod = append(doc.VerificationMethod, vm)

		// Only active key (valid_until IS NULL) goes in authentication/assertionMethod
		if kv.ValidUntil == nil {
			activeMethods = append(activeMethods, vmID)
		}
	}

	doc.Authentication = activeMethods
	doc.AssertionMethod = activeMethods

	return doc
}

// nowUTC returns the current UTC time (used in tests for validity period fixtures).
func nowUTC() time.Time {
	return time.Now().UTC()
}
