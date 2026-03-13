package api

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Discovery handles GET /.well-known/atap.json
// Returns the server discovery document per the ATAP protocol specification.
//
// SRV-01: Server discovery endpoint.
// SRV-02: trust_level field publishes the server's self-assessed trust level (L1 = DV TLS).
// SRV-03: max_approval_ttl published as 86400 seconds (enforcement deferred to Phase 3).
func (h *Handler) Discovery(c *fiber.Ctx) error {
	domain := h.config.PlatformDomain
	return c.JSON(fiber.Map{
		"domain":           domain,
		"api_base":         fmt.Sprintf("https://%s/v1", domain),
		"didcomm_endpoint": fmt.Sprintf("https://%s/v1/didcomm", domain),
		"claim_types":      []string{},
		"max_approval_ttl": 86400,
		"trust_level":      1,
		"oauth": fiber.Map{
			"token_endpoint":     fmt.Sprintf("https://%s/v1/oauth/token", domain),
			"authorize_endpoint": fmt.Sprintf("https://%s/v1/oauth/authorize", domain),
			"grant_types":        []string{"client_credentials", "authorization_code"},
			"dpop_required":      true,
		},
	})
}
