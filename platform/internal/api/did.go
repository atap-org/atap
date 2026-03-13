package api

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/crypto"
)

const mimeApplicationDIDLDJSON = "application/did+ld+json"

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
		return problem(c, 404, "not-found", "Not Found",
			fmt.Sprintf("entity %q not found", entityID))
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
