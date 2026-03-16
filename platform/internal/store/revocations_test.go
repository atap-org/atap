package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// revocationMockStore is an in-memory store implementing the RevocationStore contract.
type revocationMockStore struct {
	revocations map[string]*models.Revocation
}

func newRevocationMockStore() *revocationMockStore {
	return &revocationMockStore{
		revocations: make(map[string]*models.Revocation),
	}
}

func (m *revocationMockStore) CreateRevocation(_ context.Context, r *models.Revocation) error {
	if _, exists := m.revocations[r.ID]; exists {
		return fmt.Errorf("revocation with id %q already exists", r.ID)
	}
	// Check unique constraint on approval_id
	for _, existing := range m.revocations {
		if existing.ApprovalID == r.ApprovalID {
			return fmt.Errorf("revocation for approval_id %q already exists", r.ApprovalID)
		}
	}
	cp := *r
	m.revocations[r.ID] = &cp
	return nil
}

func (m *revocationMockStore) ListRevocations(_ context.Context, approverDID string) ([]models.Revocation, error) {
	now := time.Now().UTC()
	var results []models.Revocation
	for _, r := range m.revocations {
		if r.ApproverDID == approverDID && r.ExpiresAt.After(now) {
			results = append(results, *r)
		}
	}
	return results, nil
}

func (m *revocationMockStore) CleanupExpiredRevocations(_ context.Context) (int64, error) {
	now := time.Now().UTC()
	var count int64
	for id, r := range m.revocations {
		if r.ExpiresAt.Before(now) {
			delete(m.revocations, id)
			count++
		}
	}
	return count, nil
}

// ============================================================
// TESTS
// ============================================================

func TestRevocations_CreateAndList(t *testing.T) {
	s := newRevocationMockStore()
	ctx := context.Background()

	approverDID := "did:web:atap.app:human:approver01"
	now := time.Now().UTC()
	expires := now.Add(60 * time.Minute)

	r := &models.Revocation{
		ID:          "rev_test01",
		ApprovalID:  "apr_test01",
		ApproverDID: approverDID,
		RevokedAt:   now,
		ExpiresAt:   expires,
	}

	t.Run("CreateRevocation stores a revocation", func(t *testing.T) {
		if err := s.CreateRevocation(ctx, r); err != nil {
			t.Fatalf("CreateRevocation: %v", err)
		}
	})

	t.Run("ListRevocations returns revocation by approver DID", func(t *testing.T) {
		results, err := s.ListRevocations(ctx, approverDID)
		if err != nil {
			t.Fatalf("ListRevocations: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 revocation, got %d", len(results))
		}
		if results[0].ID != "rev_test01" {
			t.Errorf("ID = %q, want rev_test01", results[0].ID)
		}
		if results[0].ApprovalID != "apr_test01" {
			t.Errorf("ApprovalID = %q, want apr_test01", results[0].ApprovalID)
		}
		if results[0].ApproverDID != approverDID {
			t.Errorf("ApproverDID = %q, want %q", results[0].ApproverDID, approverDID)
		}
	})
}

func TestRevocations_ExpiredEntriesExcluded(t *testing.T) {
	s := newRevocationMockStore()
	ctx := context.Background()

	approverDID := "did:web:atap.app:human:approver02"
	now := time.Now().UTC()

	// Active revocation
	active := &models.Revocation{
		ID:          "rev_active",
		ApprovalID:  "apr_active",
		ApproverDID: approverDID,
		RevokedAt:   now.Add(-30 * time.Minute),
		ExpiresAt:   now.Add(30 * time.Minute), // still valid
	}

	// Expired revocation (expires_at in the past)
	expired := &models.Revocation{
		ID:          "rev_expired",
		ApprovalID:  "apr_expired",
		ApproverDID: approverDID,
		RevokedAt:   now.Add(-90 * time.Minute),
		ExpiresAt:   now.Add(-30 * time.Minute), // already expired
	}

	if err := s.CreateRevocation(ctx, active); err != nil {
		t.Fatalf("CreateRevocation active: %v", err)
	}
	if err := s.CreateRevocation(ctx, expired); err != nil {
		t.Fatalf("CreateRevocation expired: %v", err)
	}

	t.Run("ListRevocations excludes expired entries", func(t *testing.T) {
		results, err := s.ListRevocations(ctx, approverDID)
		if err != nil {
			t.Fatalf("ListRevocations: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 active revocation, got %d", len(results))
		}
		if results[0].ID != "rev_active" {
			t.Errorf("expected rev_active, got %q", results[0].ID)
		}
	})
}

func TestRevocations_CleanupExpired(t *testing.T) {
	s := newRevocationMockStore()
	ctx := context.Background()

	approverDID := "did:web:atap.app:human:approver03"
	now := time.Now().UTC()

	active := &models.Revocation{
		ID:          "rev_keep",
		ApprovalID:  "apr_keep",
		ApproverDID: approverDID,
		RevokedAt:   now,
		ExpiresAt:   now.Add(60 * time.Minute),
	}
	expired1 := &models.Revocation{
		ID:          "rev_old1",
		ApprovalID:  "apr_old1",
		ApproverDID: approverDID,
		RevokedAt:   now.Add(-2 * time.Hour),
		ExpiresAt:   now.Add(-1 * time.Hour),
	}
	expired2 := &models.Revocation{
		ID:          "rev_old2",
		ApprovalID:  "apr_old2",
		ApproverDID: approverDID,
		RevokedAt:   now.Add(-3 * time.Hour),
		ExpiresAt:   now.Add(-2 * time.Hour),
	}

	for _, r := range []*models.Revocation{active, expired1, expired2} {
		if err := s.CreateRevocation(ctx, r); err != nil {
			t.Fatalf("CreateRevocation: %v", err)
		}
	}

	t.Run("CleanupExpiredRevocations removes expired entries", func(t *testing.T) {
		n, err := s.CleanupExpiredRevocations(ctx)
		if err != nil {
			t.Fatalf("CleanupExpiredRevocations: %v", err)
		}
		if n != 2 {
			t.Errorf("expected 2 removed, got %d", n)
		}

		// Active entry should remain
		remaining, err := s.ListRevocations(ctx, approverDID)
		if err != nil {
			t.Fatalf("ListRevocations after cleanup: %v", err)
		}
		if len(remaining) != 1 {
			t.Errorf("expected 1 remaining after cleanup, got %d", len(remaining))
		}
		if remaining[0].ID != "rev_keep" {
			t.Errorf("expected rev_keep to remain, got %q", remaining[0].ID)
		}
	})
}

func TestRevocations_DuplicateApprovalID(t *testing.T) {
	s := newRevocationMockStore()
	ctx := context.Background()

	now := time.Now().UTC()
	r1 := &models.Revocation{
		ID:          "rev_dup1",
		ApprovalID:  "apr_dup",
		ApproverDID: "did:web:atap.app:human:approver04",
		RevokedAt:   now,
		ExpiresAt:   now.Add(60 * time.Minute),
	}
	r2 := &models.Revocation{
		ID:          "rev_dup2",
		ApprovalID:  "apr_dup", // same approval_id
		ApproverDID: "did:web:atap.app:human:approver04",
		RevokedAt:   now,
		ExpiresAt:   now.Add(60 * time.Minute),
	}

	if err := s.CreateRevocation(ctx, r1); err != nil {
		t.Fatalf("CreateRevocation first: %v", err)
	}

	t.Run("CreateRevocation with duplicate approval_id returns error", func(t *testing.T) {
		err := s.CreateRevocation(ctx, r2)
		if err == nil {
			t.Error("expected error for duplicate approval_id, got nil")
		}
	})
}
