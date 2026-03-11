package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/atap-dev/atap/platform/internal/models"
)

// RegisterPushToken handles POST /v1/entities/:entityId/push-token -- authenticated.
func (h *Handler) RegisterPushToken(c *fiber.Ctx) error {
	entity := c.Locals("entity").(*models.Entity)
	entityID := c.Params("entityId")

	// Own-entity enforcement
	if entity.ID != entityID {
		return problem(c, 403, "forbidden", "Cannot register push token for another entity", "")
	}

	var req struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
	}

	if req.Token == "" {
		return problem(c, 400, "invalid_request", "Token is required", "")
	}
	if req.Platform != "android" && req.Platform != "ios" {
		return problem(c, 400, "invalid_request", "Platform must be 'android' or 'ios'", "")
	}

	if err := h.pushTokenStore.UpsertPushToken(c.Context(), entityID, req.Token, req.Platform); err != nil {
		h.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to upsert push token")
		return problem(c, 500, "store_error", "Failed to register push token", "")
	}

	return c.SendStatus(204)
}
