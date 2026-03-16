package store_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// MOCK APPROVAL STORE
// ============================================================

type approvalMockStore struct {
	mu        sync.Mutex
	approvals map[string]*models.Approval
}

func newApprovalMockStore() *approvalMockStore {
	return &approvalMockStore{
		approvals: make(map[string]*models.Approval),
	}
}

func (m *approvalMockStore) CreateApproval(_ context.Context, apr *models.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *apr
	cp.State = models.ApprovalStateRequested
	cp.UpdatedAt = time.Now().UTC()
	m.approvals[apr.ID] = &cp
	return nil
}

func (m *approvalMockStore) GetApprovals(_ context.Context, entityDID string) ([]models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Approval
	for _, a := range m.approvals {
		if a.To == entityDID {
			results = append(results, *a)
		}
	}
	// Sort by created_at DESC (newest first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].CreatedAt.After(results[i].CreatedAt) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	return results, nil
}

func (m *approvalMockStore) UpdateApprovalState(_ context.Context, id, newState, responderSignature string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apr, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	if apr.State != models.ApprovalStateRequested {
		return false, nil
	}
	apr.State = newState
	now := time.Now().UTC()
	apr.RespondedAt = &now
	apr.UpdatedAt = now
	if apr.Signatures == nil {
		apr.Signatures = make(map[string]string)
	}
	apr.Signatures["to"] = responderSignature
	return true, nil
}

func (m *approvalMockStore) RevokeApproval(_ context.Context, id, entityDID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	apr, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	if apr.To != entityDID {
		return false, nil
	}
	if apr.State != models.ApprovalStateRequested && apr.State != models.ApprovalStateApproved {
		return false, nil
	}
	apr.State = models.ApprovalStateRevoked
	apr.UpdatedAt = time.Now().UTC()
	return true, nil
}

// ============================================================
// TESTS
// ============================================================

func TestApproval_CreateAndGet(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	now := time.Now().UTC()
	apr := &models.Approval{
		AtapApproval: "1",
		ID:           "apr_test01",
		CreatedAt:    now,
		From:         "did:web:atap.app:agent:from01",
		To:           "did:web:atap.app:human:to01",
		Subject: models.ApprovalSubject{
			Type:    "com.example.test",
			Label:   "Test Approval",
			Payload: json.RawMessage(`{}`),
		},
		Signatures: map[string]string{},
		State:      models.ApprovalStateRequested,
	}

	t.Run("CreateApproval stores approval", func(t *testing.T) {
		if err := s.CreateApproval(ctx, apr); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
	})

	t.Run("GetApprovals retrieves by to_did", func(t *testing.T) {
		results, err := s.GetApprovals(ctx, "did:web:atap.app:human:to01")
		if err != nil {
			t.Fatalf("GetApprovals: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 approval, got %d", len(results))
		}
		if results[0].ID != "apr_test01" {
			t.Errorf("ID = %q, want %q", results[0].ID, "apr_test01")
		}
		if results[0].State != models.ApprovalStateRequested {
			t.Errorf("State = %q, want %q", results[0].State, models.ApprovalStateRequested)
		}
	})

	t.Run("GetApprovals returns empty for unknown DID", func(t *testing.T) {
		results, err := s.GetApprovals(ctx, "did:web:atap.app:human:unknown")
		if err != nil {
			t.Fatalf("GetApprovals: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 approvals for unknown DID, got %d", len(results))
		}
	})
}

func TestApproval_UpdateState_FirstResponseWins(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	now := time.Now().UTC()
	apr := &models.Approval{
		AtapApproval: "1",
		ID:           "apr_frw01",
		CreatedAt:    now,
		From:         "did:web:atap.app:agent:from01",
		To:           "did:web:atap.app:human:to01",
		Subject: models.ApprovalSubject{
			Type:    "com.example.test",
			Label:   "First Response Wins",
			Payload: json.RawMessage(`{}`),
		},
		Signatures: map[string]string{},
		State:      models.ApprovalStateRequested,
	}

	if err := s.CreateApproval(ctx, apr); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	t.Run("first UpdateApprovalState returns true", func(t *testing.T) {
		updated, err := s.UpdateApprovalState(ctx, "apr_frw01", models.ApprovalStateApproved, "jws-sig-1")
		if err != nil {
			t.Fatalf("UpdateApprovalState: %v", err)
		}
		if !updated {
			t.Error("expected updated=true on first call")
		}
	})

	t.Run("second UpdateApprovalState returns false (first-response-wins)", func(t *testing.T) {
		updated, err := s.UpdateApprovalState(ctx, "apr_frw01", models.ApprovalStateDeclined, "jws-sig-2")
		if err != nil {
			t.Fatalf("UpdateApprovalState: %v", err)
		}
		if updated {
			t.Error("expected updated=false on second call (first-response-wins)")
		}
	})

	t.Run("state remains approved after second call", func(t *testing.T) {
		results, err := s.GetApprovals(ctx, "did:web:atap.app:human:to01")
		if err != nil {
			t.Fatalf("GetApprovals: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected approval")
		}
		if results[0].State != models.ApprovalStateApproved {
			t.Errorf("State = %q, want %q", results[0].State, models.ApprovalStateApproved)
		}
	})
}

func TestApproval_Revoke(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	now := time.Now().UTC()
	apr := &models.Approval{
		AtapApproval: "1",
		ID:           "apr_rev01",
		CreatedAt:    now,
		From:         "did:web:atap.app:agent:from01",
		To:           "did:web:atap.app:human:to01",
		Subject: models.ApprovalSubject{
			Type:    "com.example.test",
			Label:   "Revoke Test",
			Payload: json.RawMessage(`{}`),
		},
		Signatures: map[string]string{},
		State:      models.ApprovalStateRequested,
	}

	if err := s.CreateApproval(ctx, apr); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	t.Run("RevokeApproval returns true for requested state", func(t *testing.T) {
		updated, err := s.RevokeApproval(ctx, "apr_rev01", "did:web:atap.app:human:to01")
		if err != nil {
			t.Fatalf("RevokeApproval: %v", err)
		}
		if !updated {
			t.Error("expected updated=true for revoke of requested approval")
		}
	})

	t.Run("RevokeApproval returns false for already-revoked", func(t *testing.T) {
		updated, err := s.RevokeApproval(ctx, "apr_rev01", "did:web:atap.app:human:to01")
		if err != nil {
			t.Fatalf("RevokeApproval: %v", err)
		}
		if updated {
			t.Error("expected updated=false for already-revoked approval")
		}
	})

	t.Run("RevokeApproval returns false for wrong owner", func(t *testing.T) {
		// Create another approval
		apr2 := &models.Approval{
			AtapApproval: "1",
			ID:           "apr_rev02",
			CreatedAt:    now,
			From:         "did:web:atap.app:agent:from01",
			To:           "did:web:atap.app:human:to01",
			Subject: models.ApprovalSubject{
				Type:    "com.example.test",
				Label:   "Wrong Owner",
				Payload: json.RawMessage(`{}`),
			},
			Signatures: map[string]string{},
			State:      models.ApprovalStateRequested,
		}
		if err := s.CreateApproval(ctx, apr2); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}

		updated, err := s.RevokeApproval(ctx, "apr_rev02", "did:web:atap.app:human:wrong-owner")
		if err != nil {
			t.Fatalf("RevokeApproval: %v", err)
		}
		if updated {
			t.Error("expected updated=false for wrong owner DID")
		}
	})
}
