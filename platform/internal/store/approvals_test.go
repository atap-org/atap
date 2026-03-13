package store_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// approvalMockStore is an in-memory store implementing the ApprovalStore contract.
type approvalMockStore struct {
	mu        sync.Mutex
	approvals map[string]*models.Approval
}

func newApprovalMockStore() *approvalMockStore {
	return &approvalMockStore{
		approvals: make(map[string]*models.Approval),
	}
}

func (m *approvalMockStore) CreateApproval(_ context.Context, a *models.Approval) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a.State == "" {
		a.State = models.ApprovalStateRequested
	}
	// Deep-copy so mutations to the original don't affect stored value.
	copy := *a
	m.approvals[a.ID] = &copy
	return nil
}

func (m *approvalMockStore) GetApproval(_ context.Context, id string) (*models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return nil, nil
	}
	copy := *a
	return &copy, nil
}

func (m *approvalMockStore) UpdateApprovalState(_ context.Context, id, state string, respondedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return nil
	}
	a.State = state
	a.RespondedAt = respondedAt
	a.UpdatedAt = time.Now()
	return nil
}

func (m *approvalMockStore) ConsumeApproval(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.approvals[id]
	if !ok {
		return false, nil
	}
	// Only consume one-time (valid_until == nil) approvals in "approved" state.
	if a.State != models.ApprovalStateApproved || a.ValidUntil != nil {
		return false, nil
	}
	a.State = models.ApprovalStateConsumed
	a.UpdatedAt = time.Now()
	return true, nil
}

func (m *approvalMockStore) ListApprovals(_ context.Context, entityDID string, limit, offset int) ([]models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []models.Approval
	for _, a := range m.approvals {
		if a.From == entityDID || a.To == entityDID || a.Via == entityDID {
			results = append(results, *a)
		}
	}
	// Sort by CreatedAt DESC (bubble sort — small test data).
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].CreatedAt.After(results[j-1].CreatedAt); j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	if offset >= len(results) {
		return []models.Approval{}, nil
	}
	results = results[offset:]
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// allDescendants recursively collects IDs of all descendants of parentID.
func (m *approvalMockStore) allDescendants(parentID string) []string {
	var ids []string
	for _, a := range m.approvals {
		if a.Parent == parentID {
			ids = append(ids, a.ID)
			ids = append(ids, m.allDescendants(a.ID)...)
		}
	}
	return ids
}

func (m *approvalMockStore) GetChildApprovals(_ context.Context, parentID string) ([]models.Approval, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.allDescendants(parentID)
	var results []models.Approval
	for _, id := range ids {
		if a, ok := m.approvals[id]; ok {
			results = append(results, *a)
		}
	}
	return results, nil
}

func (m *approvalMockStore) RevokeWithChildren(_ context.Context, parentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Revoke the parent itself.
	if a, ok := m.approvals[parentID]; ok {
		a.State = models.ApprovalStateRevoked
		a.UpdatedAt = time.Now()
	}
	// Revoke all descendants.
	ids := m.allDescendants(parentID)
	for _, id := range ids {
		if a, ok := m.approvals[id]; ok {
			a.State = models.ApprovalStateRevoked
			a.UpdatedAt = time.Now()
		}
	}
	return nil
}

// ============================================================
// HELPERS
// ============================================================

func makeApproval(id, from, to string) *models.Approval {
	payload, _ := json.Marshal(map[string]string{"action": "test"})
	return &models.Approval{
		AtapApproval: "1",
		ID:           id,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		From:         from,
		To:           to,
		Subject: models.ApprovalSubject{
			Type:       "dev.atap.test.action",
			Label:      "Test Action",
			Reversible: false,
			Payload:    json.RawMessage(payload),
		},
		Signatures: map[string]string{},
	}
}

// ============================================================
// CONTRACT TESTS
// ============================================================

func TestApprovalStore_CreateAndGet(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	a := makeApproval("apr_01", "did:web:atap.app:agent:from01", "did:web:atap.app:agent:to01")

	t.Run("CreateApproval stores approval with default state", func(t *testing.T) {
		if err := s.CreateApproval(ctx, a); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
	})

	t.Run("GetApproval retrieves approval with all fields", func(t *testing.T) {
		got, err := s.GetApproval(ctx, "apr_01")
		if err != nil {
			t.Fatalf("GetApproval: %v", err)
		}
		if got == nil {
			t.Fatal("GetApproval returned nil for existing approval")
		}
		if got.ID != "apr_01" {
			t.Errorf("ID = %q, want apr_01", got.ID)
		}
		if got.From != "did:web:atap.app:agent:from01" {
			t.Errorf("From = %q, want did:web:atap.app:agent:from01", got.From)
		}
		if got.To != "did:web:atap.app:agent:to01" {
			t.Errorf("To = %q, want did:web:atap.app:agent:to01", got.To)
		}
		if got.State != models.ApprovalStateRequested {
			t.Errorf("State = %q, want %q", got.State, models.ApprovalStateRequested)
		}
		if got.Subject.Type != "dev.atap.test.action" {
			t.Errorf("Subject.Type = %q, want dev.atap.test.action", got.Subject.Type)
		}
	})

	t.Run("GetApproval returns nil for unknown ID", func(t *testing.T) {
		got, err := s.GetApproval(ctx, "apr_nonexistent")
		if err != nil {
			t.Fatalf("GetApproval: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil for unknown ID, got %+v", got)
		}
	})
}

func TestApprovalStore_UpdateApprovalState(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	a := makeApproval("apr_state01", "did:web:atap.app:agent:fromS", "did:web:atap.app:agent:toS")
	if err := s.CreateApproval(ctx, a); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}

	t.Run("UpdateApprovalState transitions to approved", func(t *testing.T) {
		now := time.Now().UTC()
		if err := s.UpdateApprovalState(ctx, "apr_state01", models.ApprovalStateApproved, &now); err != nil {
			t.Fatalf("UpdateApprovalState: %v", err)
		}

		got, err := s.GetApproval(ctx, "apr_state01")
		if err != nil {
			t.Fatalf("GetApproval: %v", err)
		}
		if got.State != models.ApprovalStateApproved {
			t.Errorf("State = %q, want %q", got.State, models.ApprovalStateApproved)
		}
		if got.RespondedAt == nil {
			t.Error("RespondedAt should be set after UpdateApprovalState with timestamp")
		}
	})

	t.Run("UpdateApprovalState transitions to declined", func(t *testing.T) {
		if err := s.UpdateApprovalState(ctx, "apr_state01", models.ApprovalStateDeclined, nil); err != nil {
			t.Fatalf("UpdateApprovalState: %v", err)
		}

		got, err := s.GetApproval(ctx, "apr_state01")
		if err != nil {
			t.Fatalf("GetApproval: %v", err)
		}
		if got.State != models.ApprovalStateDeclined {
			t.Errorf("State = %q, want %q", got.State, models.ApprovalStateDeclined)
		}
	})
}

func TestApprovalStore_ListApprovals(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	entity := "did:web:atap.app:agent:lister"

	a1 := makeApproval("apr_list01", entity, "did:web:atap.app:agent:other1")
	a1.CreatedAt = time.Now().UTC().Add(-2 * time.Second)
	a1.UpdatedAt = a1.CreatedAt

	a2 := makeApproval("apr_list02", entity, "did:web:atap.app:agent:other2")
	a2.CreatedAt = time.Now().UTC().Add(-1 * time.Second)
	a2.UpdatedAt = a2.CreatedAt

	a3 := makeApproval("apr_list03", "did:web:atap.app:agent:other3", entity)
	a3.CreatedAt = time.Now().UTC()
	a3.UpdatedAt = a3.CreatedAt

	for _, a := range []*models.Approval{a1, a2, a3} {
		if err := s.CreateApproval(ctx, a); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
	}

	t.Run("ListApprovals returns all approvals for entity", func(t *testing.T) {
		results, err := s.ListApprovals(ctx, entity, 100, 0)
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 approvals, got %d", len(results))
		}
		// Should be ordered DESC by created_at — newest first.
		if results[0].ID != "apr_list03" {
			t.Errorf("results[0].ID = %q, want apr_list03", results[0].ID)
		}
	})

	t.Run("ListApprovals respects limit", func(t *testing.T) {
		results, err := s.ListApprovals(ctx, entity, 2, 0)
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 approvals (limit=2), got %d", len(results))
		}
	})

	t.Run("ListApprovals respects offset", func(t *testing.T) {
		results, err := s.ListApprovals(ctx, entity, 100, 2)
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 approval (offset=2), got %d", len(results))
		}
	})
}

func TestApprovalStore_GetChildApprovals(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	parent := makeApproval("apr_parent", "did:web:atap.app:agent:p", "did:web:atap.app:agent:q")

	child := makeApproval("apr_child", "did:web:atap.app:agent:p", "did:web:atap.app:agent:q")
	child.Parent = "apr_parent"

	grandchild := makeApproval("apr_grandchild", "did:web:atap.app:agent:p", "did:web:atap.app:agent:q")
	grandchild.Parent = "apr_child"

	for _, a := range []*models.Approval{parent, child, grandchild} {
		if err := s.CreateApproval(ctx, a); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
	}

	t.Run("GetChildApprovals returns all descendants recursively", func(t *testing.T) {
		children, err := s.GetChildApprovals(ctx, "apr_parent")
		if err != nil {
			t.Fatalf("GetChildApprovals: %v", err)
		}
		if len(children) != 2 {
			t.Fatalf("expected 2 descendants (child + grandchild), got %d", len(children))
		}
		ids := map[string]bool{}
		for _, c := range children {
			ids[c.ID] = true
		}
		if !ids["apr_child"] {
			t.Error("expected apr_child in descendants")
		}
		if !ids["apr_grandchild"] {
			t.Error("expected apr_grandchild in descendants")
		}
		// Parent itself should NOT be included.
		if ids["apr_parent"] {
			t.Error("parent should not be in its own descendants list")
		}
	})

	t.Run("GetChildApprovals returns empty for leaf node", func(t *testing.T) {
		children, err := s.GetChildApprovals(ctx, "apr_grandchild")
		if err != nil {
			t.Fatalf("GetChildApprovals: %v", err)
		}
		if len(children) != 0 {
			t.Errorf("expected 0 children for leaf node, got %d", len(children))
		}
	})
}

func TestApprovalStore_RevokeWithChildren(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	root := makeApproval("apr_root", "did:web:atap.app:agent:r1", "did:web:atap.app:agent:r2")
	child := makeApproval("apr_rchild", "did:web:atap.app:agent:r1", "did:web:atap.app:agent:r2")
	child.Parent = "apr_root"
	grandchild := makeApproval("apr_rgrand", "did:web:atap.app:agent:r1", "did:web:atap.app:agent:r2")
	grandchild.Parent = "apr_rchild"

	for _, a := range []*models.Approval{root, child, grandchild} {
		if err := s.CreateApproval(ctx, a); err != nil {
			t.Fatalf("CreateApproval: %v", err)
		}
	}

	t.Run("RevokeWithChildren marks root and all descendants as revoked", func(t *testing.T) {
		if err := s.RevokeWithChildren(ctx, "apr_root"); err != nil {
			t.Fatalf("RevokeWithChildren: %v", err)
		}

		for _, id := range []string{"apr_root", "apr_rchild", "apr_rgrand"} {
			got, err := s.GetApproval(ctx, id)
			if err != nil {
				t.Fatalf("GetApproval(%s): %v", id, err)
			}
			if got.State != models.ApprovalStateRevoked {
				t.Errorf("approval %s: State = %q, want %q", id, got.State, models.ApprovalStateRevoked)
			}
		}
	})
}

func TestApprovalStore_ConsumeApproval(t *testing.T) {
	s := newApprovalMockStore()
	ctx := context.Background()

	// One-time approval: valid_until is nil.
	oneTime := makeApproval("apr_onetime", "did:web:atap.app:agent:ot1", "did:web:atap.app:agent:ot2")
	// oneTime.ValidUntil is nil by default from makeApproval.
	if err := s.CreateApproval(ctx, oneTime); err != nil {
		t.Fatalf("CreateApproval: %v", err)
	}
	// Transition to approved.
	now := time.Now().UTC()
	if err := s.UpdateApprovalState(ctx, "apr_onetime", models.ApprovalStateApproved, &now); err != nil {
		t.Fatalf("UpdateApprovalState: %v", err)
	}

	t.Run("ConsumeApproval first call returns true", func(t *testing.T) {
		consumed, err := s.ConsumeApproval(ctx, "apr_onetime")
		if err != nil {
			t.Fatalf("ConsumeApproval: %v", err)
		}
		if !consumed {
			t.Error("first ConsumeApproval should return true")
		}
		got, _ := s.GetApproval(ctx, "apr_onetime")
		if got.State != models.ApprovalStateConsumed {
			t.Errorf("State = %q, want %q", got.State, models.ApprovalStateConsumed)
		}
	})

	t.Run("ConsumeApproval second call returns false (idempotent)", func(t *testing.T) {
		consumed, err := s.ConsumeApproval(ctx, "apr_onetime")
		if err != nil {
			t.Fatalf("ConsumeApproval: %v", err)
		}
		if consumed {
			t.Error("second ConsumeApproval should return false (already consumed)")
		}
	})

	// Persistent approval: valid_until is set — ConsumeApproval must not consume it.
	future := time.Now().UTC().Add(24 * time.Hour)
	persistent := makeApproval("apr_persist", "did:web:atap.app:agent:ps1", "did:web:atap.app:agent:ps2")
	persistent.ValidUntil = &future
	if err := s.CreateApproval(ctx, persistent); err != nil {
		t.Fatalf("CreateApproval persistent: %v", err)
	}
	if err := s.UpdateApprovalState(ctx, "apr_persist", models.ApprovalStateApproved, &now); err != nil {
		t.Fatalf("UpdateApprovalState persistent: %v", err)
	}

	t.Run("ConsumeApproval on persistent approval returns false", func(t *testing.T) {
		consumed, err := s.ConsumeApproval(ctx, "apr_persist")
		if err != nil {
			t.Fatalf("ConsumeApproval: %v", err)
		}
		if consumed {
			t.Error("ConsumeApproval on persistent approval should return false")
		}
		got, _ := s.GetApproval(ctx, "apr_persist")
		if got.State != models.ApprovalStateApproved {
			t.Errorf("persistent approval State = %q, want %q", got.State, models.ApprovalStateApproved)
		}
	})
}
