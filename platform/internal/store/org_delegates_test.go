package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
	"github.com/atap-dev/atap/platform/internal/store"
)

// newTestStore connects to a test database.
// Skips the test if DATABASE_URL is not set.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration tests")
	}
	s, err := store.New(databaseURL)
	if err != nil {
		t.Fatalf("connect store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// createOrgDelegateEntity creates a minimal entity for testing org delegate queries.
func createOrgDelegateEntity(t *testing.T, s *store.Store, id, entityType, did, principalDID string) {
	t.Helper()
	e := &models.Entity{
		ID:           id,
		Type:         entityType,
		DID:          did,
		PrincipalDID: principalDID,
		Name:         "test-" + id,
		TrustLevel:   0,
		Registry:     "test",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.CreateEntity(context.Background(), e); err != nil {
		t.Fatalf("createOrgDelegateEntity %q: %v", id, err)
	}
	t.Cleanup(func() {
		_ = s.DeleteEntity(context.Background(), id)
	})
}

// TestOrgDelegate_Empty verifies GetOrgDelegates returns an empty (non-nil) slice
// when the org has no delegates.
func TestOrgDelegate_Empty(t *testing.T) {
	s := newTestStore(t)

	orgID := "test-org-empty-01"
	orgDID := "did:web:atap.app:org:" + orgID
	createOrgDelegateEntity(t, s, orgID, models.EntityTypeOrg, orgDID, "")

	delegates, err := s.GetOrgDelegates(context.Background(), orgDID, 50)
	if err != nil {
		t.Fatalf("GetOrgDelegates: %v", err)
	}
	if delegates == nil {
		t.Error("GetOrgDelegates returned nil, want empty slice")
	}
	if len(delegates) != 0 {
		t.Errorf("GetOrgDelegates returned %d delegates, want 0", len(delegates))
	}
}

// TestOrgDelegate_Normal verifies that entities with principal_did matching the org DID
// are returned, and that the org itself is excluded.
func TestOrgDelegate_Normal(t *testing.T) {
	s := newTestStore(t)

	orgID := "test-org-normal-01"
	orgDID := "did:web:atap.app:org:" + orgID
	createOrgDelegateEntity(t, s, orgID, models.EntityTypeOrg, orgDID, "")

	// Create 3 delegate members
	for i := 1; i <= 3; i++ {
		memberID := fmt.Sprintf("test-member-normal-%02d", i)
		memberDID := "did:web:atap.app:human:" + memberID
		createOrgDelegateEntity(t, s, memberID, models.EntityTypeHuman, memberDID, orgDID)
	}

	// Create an unrelated entity to ensure it is not returned
	unrelatedID := "test-unrelated-normal-01"
	createOrgDelegateEntity(t, s, unrelatedID, models.EntityTypeHuman,
		"did:web:atap.app:human:"+unrelatedID, "did:web:atap.app:org:other-org")

	delegates, err := s.GetOrgDelegates(context.Background(), orgDID, 50)
	if err != nil {
		t.Fatalf("GetOrgDelegates: %v", err)
	}
	if len(delegates) != 3 {
		t.Errorf("GetOrgDelegates returned %d delegates, want 3", len(delegates))
	}

	// Verify org entity is not in the result set
	for _, d := range delegates {
		if d.DID == orgDID {
			t.Error("GetOrgDelegates returned the org entity itself; want members only")
		}
	}
}

// TestOrgDelegate_ExcludesOrgItself verifies that only members (human/agent/machine) are
// returned, not the org entity whose DID is the principal_did.
func TestOrgDelegate_ExcludesOrgItself(t *testing.T) {
	s := newTestStore(t)

	orgID := "test-org-excl-01"
	orgDID := "did:web:atap.app:org:" + orgID
	createOrgDelegateEntity(t, s, orgID, models.EntityTypeOrg, orgDID, "")

	// An org-type entity that happens to have this org as principal_did should be excluded.
	subOrgID := "test-suborg-excl-01"
	createOrgDelegateEntity(t, s, subOrgID, models.EntityTypeOrg,
		"did:web:atap.app:org:"+subOrgID, orgDID)

	humanID := "test-human-excl-01"
	createOrgDelegateEntity(t, s, humanID, models.EntityTypeHuman,
		"did:web:atap.app:human:"+humanID, orgDID)

	delegates, err := s.GetOrgDelegates(context.Background(), orgDID, 50)
	if err != nil {
		t.Fatalf("GetOrgDelegates: %v", err)
	}

	// Only the human entity should be returned (org type excluded by query)
	if len(delegates) != 1 {
		t.Errorf("GetOrgDelegates returned %d delegates, want 1 (human only)", len(delegates))
	}
	if len(delegates) == 1 && delegates[0].Type != models.EntityTypeHuman {
		t.Errorf("delegate type = %q, want %q", delegates[0].Type, models.EntityTypeHuman)
	}
}

// TestOrgDelegate_CapAt50 verifies that when there are more than 50 matching entities,
// GetOrgDelegates returns exactly 50 (server-side enforcement).
func TestOrgDelegate_CapAt50(t *testing.T) {
	s := newTestStore(t)

	orgID := "test-org-cap-01"
	orgDID := "did:web:atap.app:org:" + orgID
	createOrgDelegateEntity(t, s, orgID, models.EntityTypeOrg, orgDID, "")

	// Create 60 members
	for i := 1; i <= 60; i++ {
		memberID := fmt.Sprintf("test-member-cap-%03d", i)
		memberDID := fmt.Sprintf("did:web:atap.app:agent:%s", memberID)
		createOrgDelegateEntity(t, s, memberID, models.EntityTypeAgent, memberDID, orgDID)
	}

	// Request more than 50 — should be capped server-side
	delegates, err := s.GetOrgDelegates(context.Background(), orgDID, 100)
	if err != nil {
		t.Fatalf("GetOrgDelegates: %v", err)
	}
	if len(delegates) != 50 {
		t.Errorf("GetOrgDelegates with limit=100 and 60 members returned %d, want 50 (cap)", len(delegates))
	}
}
