package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/atap-dev/atap/platform/internal/didcomm"
	"github.com/atap-dev/atap/platform/internal/models"
)

// fanOutRateLimit is the maximum number of fan-out approval requests a single source DID
// may send to the same org DID within a 1-hour sliding window (spec MSG-06).
const fanOutRateLimit = 10

// ============================================================
// POST /v1/approvals
// ============================================================

// CreateApproval handles POST /v1/approvals.
//
// Creates an approval request and dispatches a DIDComm notification to the target.
// If the target entity is an org, the message is fan-out to all org delegates (up to 50).
//
// Per-source rate limiting prevents a single source DID from flooding an org's
// delegates. The rate limit is 10 fan-out requests per hour per (source, org) pair.
func (h *Handler) CreateApproval(c *fiber.Ctx) error {
	entity, ok := c.Locals("entity").(*models.Entity)
	if !ok || entity == nil {
		return problem(c, fiber.StatusUnauthorized, "unauthorized", "Unauthorized",
			"no authenticated entity in context")
	}

	var req struct {
		From    string                 `json:"from"`
		To      string                 `json:"to"`
		Via     string                 `json:"via,omitempty"`
		Subject models.ApprovalSubject `json:"subject"`
	}
	if err := c.BodyParser(&req); err != nil {
		return problem(c, fiber.StatusBadRequest, "bad-request", "Bad Request", "invalid JSON body")
	}
	if req.From == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "from is required")
	}
	if req.To == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "to is required")
	}
	if req.Subject.Type == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "subject.type is required")
	}
	if req.Subject.Label == "" {
		return problem(c, fiber.StatusBadRequest, "validation", "Validation Error", "subject.label is required")
	}
	if req.Subject.Payload == nil {
		req.Subject.Payload = json.RawMessage("{}")
	}

	now := time.Now().UTC()
	approvalID := "apr_" + ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()

	approval := &models.Approval{
		AtapApproval: "1",
		ID:           approvalID,
		CreatedAt:    now,
		From:         req.From,
		To:           req.To,
		Via:          req.Via,
		Subject:      req.Subject,
		Signatures:   map[string]string{},
		State:        models.ApprovalStateRequested,
	}

	// Look up the target entity to determine if fan-out is needed.
	toEntity, err := h.entityStore.GetEntityByDID(c.Context(), req.To)
	if err != nil {
		h.log.Error().Err(err).Str("to", req.To).Msg("failed to look up target entity")
		return problem(c, fiber.StatusInternalServerError, "internal", "Internal Server Error",
			"failed to look up target entity")
	}

	// Dispatch DIDComm approval request message.
	msgBody := map[string]interface{}{
		"approval_id": approvalID,
		"from":        req.From,
		"to":          req.To,
		"subject":     req.Subject,
	}
	if req.Via != "" {
		msgBody["via"] = req.Via
	}

	baseMsg := &didcomm.PlaintextMessage{
		ID:   "msg_" + ulid.MustNew(ulid.Timestamp(now), rand.Reader).String(),
		Type: "https://atap.dev/approval/1.0/request",
		From: req.From,
		To:   []string{req.To},
		Body: msgBody,
	}

	if toEntity != nil && toEntity.Type == models.EntityTypeOrg {
		// Org fan-out: check per-source rate limit before dispatching.
		limited, limitResp := h.checkFanOutRateLimitAndRespond(c, req.From, req.To)
		if limited {
			return limitResp
		}

		// Fan-out to org delegates (up to 50).
		if h.orgDelegateStore != nil {
			delegates, err := h.orgDelegateStore.GetOrgDelegates(c.Context(), req.To, 50)
			if err != nil {
				h.log.Error().Err(err).Str("org_did", req.To).Msg("failed to get org delegates")
				// Non-fatal: fall back to direct dispatch to the org DID.
			} else if len(delegates) > 0 {
				// Dispatch concurrently to all delegates.
				go func() {
					for _, delegate := range delegates {
						delegateMsg := &didcomm.PlaintextMessage{
							ID:   "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String(),
							Type: "https://atap.dev/approval/1.0/request",
							From: req.From,
							To:   []string{delegate.DID},
							Body: msgBody,
						}
						if qErr := h.dispatchDIDCommMessageTo(delegateMsg, delegate.DID); qErr != nil {
							h.log.Error().Err(qErr).
								Str("delegate_did", delegate.DID).
								Str("approval_id", approvalID).
								Msg("fan-out: failed to dispatch to delegate")
						}
					}
				}()
				// Return immediately after launching goroutine.
				return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
					"id":         approval.ID,
					"state":      approval.State,
					"created_at": approval.CreatedAt,
					"fan_out":    len(delegates),
				})
			}
		}
	}

	// Direct dispatch (non-org or no delegates found).
	if err := h.dispatchDIDCommMessage(baseMsg); err != nil {
		h.log.Warn().Err(err).Str("approval_id", approvalID).Msg("failed to dispatch DIDComm approval request (non-fatal)")
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"id":         approval.ID,
		"state":      approval.State,
		"created_at": approval.CreatedAt,
	})
}

// checkFanOutRateLimit checks and increments the per-source-per-org fan-out rate limit counter.
// Returns an RFC 7807 429 problem response if the rate limit is exceeded.
// Uses Redis INCR with 1-hour TTL. If Redis is unavailable, the check is skipped (best-effort).
func (h *Handler) checkFanOutRateLimit(ctx context.Context, sourceDID, orgDID string) error {
	if h.redis == nil {
		return nil
	}

	rateKey := fmt.Sprintf("fanout:rate:%s:%s", sourceDID, orgDID)

	// INCR atomically increments the counter and returns the new value.
	count, err := h.redis.Incr(ctx, rateKey).Result()
	if err != nil {
		// Redis unavailable — best-effort; allow the request.
		h.log.Warn().Err(err).Str("key", rateKey).Msg("fanout rate limit: Redis INCR failed (skipping check)")
		return nil
	}

	// On the first increment (count == 1), set the expiry to 1 hour.
	// Use NX flag so we don't reset TTL on subsequent requests.
	if count == 1 {
		h.redis.Expire(ctx, rateKey, time.Hour)
	}

	if count > fanOutRateLimit {
		return &fanOutRateLimitError{}
	}

	return nil
}

// fanOutRateLimitError is a sentinel error type for rate limit exceeded.
// It carries the RFC 7807 response details.
type fanOutRateLimitError struct{}

func (e *fanOutRateLimitError) Error() string {
	return "fan-out rate limit exceeded"
}

// dispatchDIDCommMessageTo serializes and queues a DIDComm message for a specific recipient DID.
func (h *Handler) dispatchDIDCommMessageTo(msg *didcomm.PlaintextMessage, recipientDID string) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("dispatchDIDCommMessageTo: marshal: %w", err)
	}
	msgID := "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	qmsg := &models.DIDCommMessage{
		ID:           msgID,
		RecipientDID: recipientDID,
		SenderDID:    msg.From,
		MessageType:  msg.Type,
		Payload:      payload,
		State:        "pending",
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.messageStore.QueueMessage(context.Background(), qmsg); err != nil {
		return fmt.Errorf("dispatchDIDCommMessageTo: queue: %w", err)
	}
	return nil
}

// respondWithFanOutRateLimitError writes a 429 RFC 7807 response for the fan-out rate limit.
func respondWithFanOutRateLimitError(c *fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(models.ProblemDetail{
		Type:     "https://atap.dev/errors/rate-limit-exceeded",
		Title:    "Rate limit exceeded",
		Status:   fiber.StatusTooManyRequests,
		Detail:   "Too many fan-out requests to this organization. Try again later.",
		Instance: c.Path(),
	}, mimeApplicationProblemJSON)
}

// isFanOutRateLimitError checks if an error is a fan-out rate limit error.
func isFanOutRateLimitError(err error) bool {
	_, ok := err.(*fanOutRateLimitError)
	return ok
}

// checkFanOutRateLimitAndRespond checks the rate limit and writes a response if exceeded.
// Returns true if the rate limit was exceeded (and the caller should return).
func (h *Handler) checkFanOutRateLimitAndRespond(c *fiber.Ctx, sourceDID, orgDID string) (bool, error) {
	if err := h.checkFanOutRateLimit(c.Context(), sourceDID, orgDID); err != nil {
		if isFanOutRateLimitError(err) {
			return true, respondWithFanOutRateLimitError(c)
		}
		return false, nil
	}
	return false, nil
}
