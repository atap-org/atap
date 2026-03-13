package approval

import (
	"fmt"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// validTransitions defines the allowed state transitions per spec §8.3.
// Terminal states (declined, expired, rejected, consumed, revoked) are not present
// as keys, meaning no transitions are allowed FROM those states.
var validTransitions = map[string][]string{
	models.ApprovalStateRequested: {
		models.ApprovalStateApproved,
		models.ApprovalStateDeclined,
		models.ApprovalStateExpired,
		models.ApprovalStateRejected,
	},
	models.ApprovalStateApproved: {
		models.ApprovalStateConsumed,
		models.ApprovalStateRevoked,
	},
}

// ValidateTransition checks if transitioning from currentState to nextState is allowed.
// Returns nil if valid, error if the transition is not permitted.
func ValidateTransition(currentState, nextState string) error {
	allowed, ok := validTransitions[currentState]
	if !ok {
		return fmt.Errorf("invalid transition from %s to %s: %s is a terminal state with no valid transitions",
			currentState, nextState, currentState)
	}
	for _, s := range allowed {
		if s == nextState {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %s to %s", currentState, nextState)
}

// IsTerminalState returns true if the given state has no valid outgoing transitions.
// Terminal states: declined, expired, rejected, consumed, revoked.
func IsTerminalState(state string) bool {
	_, hasTransitions := validTransitions[state]
	return !hasTransitions
}

// ClampValidUntil enforces the server's max approval TTL policy.
// If validUntil is nil (one-time approval), it is returned as nil.
// Otherwise, returns min(validUntil, now+maxTTL).
func ClampValidUntil(validUntil *time.Time, maxTTL time.Duration) *time.Time {
	if validUntil == nil {
		return nil
	}
	ceiling := time.Now().Add(maxTTL)
	if validUntil.After(ceiling) {
		return &ceiling
	}
	return validUntil
}
