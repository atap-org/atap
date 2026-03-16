package api

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

const (
	mimeApplicationDIDLDJSON = "application/did+ld+json"
	mimeApplicationDIDJSON   = "application/did+json"
)

// ResolveDID handles GET /:type/:id/did.json
// Returns a W3C DID Document for the given entity at the standard did:web resolution path.
// Content-Type is application/did+ld+json per the DID spec.
func (h *Handler) ResolveDID(c *fiber.Ctx) error {
	entityType := c.Params("type")
	entityID := c.Params("id")

	// Only accept valid entity types -- return 404 for anything else
	if !validEntityTypes[entityType] {
		return problem(c, 404, "not-found", "Not Found",
			fmt.Sprintf("unknown entity type %q", entityType))
	}

	// Look up entity by ID
	entity, err := h.entityStore.GetEntity(c.Context(), entityID)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to get entity for DID resolution")
		return problem(c, 500, "internal", "Internal Server Error", "failed to resolve DID")
	}
	if entity == nil {
		// Per W3C DID Core spec and PRV-03: return 410 Gone (not 404).
		// 410 indicates the entity existed but was deactivated/deleted.
		// For v1.0, we return 410 for any missing entity at the DID resolution path,
		// since only registered entities should be resolved.
		return c.Status(410).JSON(fiber.Map{
			"deactivated": true,
			"error":       "did-deactivated",
			"message":     fmt.Sprintf("DID document for %q/%q has been deactivated", entityType, entityID),
		})
	}

	// Type must match the URL parameter (prevents cross-type resolution)
	if entity.Type != entityType {
		return problem(c, 404, "not-found", "Not Found",
			fmt.Sprintf("entity %q not found", entityID))
	}

	// Fetch all key versions for the entity (including rotated keys)
	keyVersions, err := h.keyVersionStore.GetKeyVersions(c.Context(), entityID)
	if err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to get key versions for DID resolution")
		return problem(c, 500, "internal", "Internal Server Error", "failed to resolve DID")
	}

	// Build the DID Document
	doc := crypto.BuildDIDDocument(entity, keyVersions, h.config.PlatformDomain)

	// Marshal manually so we can set Content-Type: application/did+ld+json
	// (Fiber's c.JSON defaults to application/json -- must NOT use it here per DID spec)
	docJSON, err := json.Marshal(doc)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal DID document")
		return problem(c, 500, "internal", "Internal Server Error", "failed to serialize DID document")
	}

	c.Set(fiber.HeaderContentType, mimeApplicationDIDLDJSON)
	return c.Status(200).Send(docJSON)
}

// ResolveServerDID handles GET /server/platform/did.json
// Returns the server's own DID Document for did:web:{domain}:server:platform.
// The server DID acts as the trusted "via" participant for DIDComm routing (MSG-03).
// Content-Type is application/did+json.
func (h *Handler) ResolveServerDID(c *fiber.Ctx) error {
	serverDID := fmt.Sprintf("did:web:%s:server:platform", h.config.PlatformDomain)

	// Ed25519 signing key verification method
	ed25519PubKey := h.platformKey.Public().(ed25519.PublicKey)
	ed25519VM := models.VerificationMethod{
		ID:                 serverDID + "#key-ed25519-1",
		Type:               "Ed25519VerificationKey2020",
		Controller:         serverDID,
		PublicKeyMultibase: crypto.EncodePublicKeyMultibase(ed25519PubKey),
	}

	doc := &models.DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
			"https://atap.dev/ns/v1",
		},
		ID:                 serverDID,
		VerificationMethod: []models.VerificationMethod{ed25519VM},
		Authentication:     []string{serverDID + "#key-ed25519-1"},
		AssertionMethod:    []string{serverDID + "#key-ed25519-1"},
		ATAPType:           "platform",
	}

	// Add X25519 keyAgreement and DIDCommMessaging service if server has X25519 key
	if h.platformX25519Key != nil {
		x25519VMID := serverDID + "#key-x25519-1"
		x25519VM := models.VerificationMethod{
			ID:                 x25519VMID,
			Type:               "X25519KeyAgreementKey2020",
			Controller:         serverDID,
			PublicKeyMultibase: crypto.EncodeX25519PublicKeyMultibase(h.platformX25519Key.PublicKey().Bytes()),
		}
		doc.VerificationMethod = append(doc.VerificationMethod, x25519VM)
		doc.KeyAgreement = []string{x25519VMID}

		doc.Service = []models.DIDService{{
			ID:   serverDID + "#didcomm",
			Type: "DIDCommMessaging",
			ServiceEndpoint: models.DIDServiceEndpoint{
				URI:         fmt.Sprintf("https://%s/v1/didcomm", h.config.PlatformDomain),
				Accept:      []string{"didcomm/v2"},
				RoutingKeys: []string{},
			},
		}}
	}

	docJSON, err := json.Marshal(doc)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to marshal server DID document")
		return problem(c, 500, "internal", "Internal Server Error", "failed to serialize server DID document")
	}

	c.Set(fiber.HeaderContentType, mimeApplicationDIDJSON)
	return c.Status(200).Send(docJSON)
}
