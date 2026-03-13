package approval_test

import (
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/approval"
	"github.com/atap-dev/atap/platform/internal/models"
)

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		{models.ApprovalStateRequested, models.ApprovalStateApproved},
		{models.ApprovalStateRequested, models.ApprovalStateDeclined},
		{models.ApprovalStateRequested, models.ApprovalStateExpired},
		{models.ApprovalStateRequested, models.ApprovalStateRejected},
		{models.ApprovalStateApproved, models.ApprovalStateConsumed},
		{models.ApprovalStateApproved, models.ApprovalStateRevoked},
	}
	for _, c := range cases {
		if err := approval.ValidateTransition(c.from, c.to); err != nil {
			t.Errorf("expected %s->%s to be valid, got: %v", c.from, c.to, err)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	cases := []struct {
		from, to string
	}{
		// Self-transitions
		{models.ApprovalStateApproved, models.ApprovalStateApproved},
		// Terminal states have no valid outgoing transitions
		{models.ApprovalStateDeclined, models.ApprovalStateRequested},
		{models.ApprovalStateDeclined, models.ApprovalStateApproved},
		{models.ApprovalStateExpired, models.ApprovalStateRequested},
		{models.ApprovalStateExpired, models.ApprovalStateApproved},
		{models.ApprovalStateConsumed, models.ApprovalStateRequested},
		{models.ApprovalStateConsumed, models.ApprovalStateApproved},
		{models.ApprovalStateRevoked, models.ApprovalStateRequested},
		{models.ApprovalStateRevoked, models.ApprovalStateApproved},
		{models.ApprovalStateRejected, models.ApprovalStateRequested},
		{models.ApprovalStateRejected, models.ApprovalStateApproved},
	}
	for _, c := range cases {
		if err := approval.ValidateTransition(c.from, c.to); err == nil {
			t.Errorf("expected %s->%s to be invalid, got nil error", c.from, c.to)
		}
	}
}

func TestIsTerminalState(t *testing.T) {
	terminal := []string{
		models.ApprovalStateDeclined,
		models.ApprovalStateExpired,
		models.ApprovalStateRejected,
		models.ApprovalStateConsumed,
		models.ApprovalStateRevoked,
	}
	nonTerminal := []string{
		models.ApprovalStateRequested,
		models.ApprovalStateApproved,
	}

	for _, s := range terminal {
		if !approval.IsTerminalState(s) {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if approval.IsTerminalState(s) {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

func TestClampValidUntil(t *testing.T) {
	maxTTL := 24 * time.Hour

	// validUntil within TTL -> returned unchanged
	future := time.Now().Add(1 * time.Hour)
	result := approval.ClampValidUntil(&future, maxTTL)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Equal(future) {
		t.Errorf("expected %v, got %v", future, *result)
	}

	// validUntil beyond TTL -> clamped to now+maxTTL
	farFuture := time.Now().Add(48 * time.Hour)
	result = approval.ClampValidUntil(&farFuture, maxTTL)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should be approximately now+maxTTL (within 5 seconds tolerance)
	expected := time.Now().Add(maxTTL)
	diff := result.Sub(expected)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("clamped value %v not close to now+maxTTL %v", *result, expected)
	}
	if result.After(farFuture) {
		t.Errorf("clamped result %v should not exceed original %v", *result, farFuture)
	}
}

func TestClampValidUntilNil(t *testing.T) {
	maxTTL := 24 * time.Hour
	result := approval.ClampValidUntil(nil, maxTTL)
	if result != nil {
		t.Errorf("expected nil for one-time approval, got %v", result)
	}
}
