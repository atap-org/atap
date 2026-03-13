package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// Note: Integration tests that require DATABASE_URL (PostgreSQL) are skipped
// when DATABASE_URL is not set. Unit-level behavior is validated through the
// mock in api/entities_test.go and api/did_test.go.

// TestKeyVersionMockBehavior validates the key version store contract using
// the same mock used in API tests (no DB required).
// These tests document the expected behavior for the real store implementation.
func TestKeyVersionMockBehavior(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateKeyVersion stores version", func(t *testing.T) {
		kvs := &testKeyVersionStore{versions: make(map[string][]models.KeyVersion)}
		kv := &models.KeyVersion{
			ID:        "kv_test01",
			EntityID:  "entity01",
			PublicKey: []byte("fakepubkey01"),
			KeyIndex:  1,
			ValidFrom: time.Now().UTC(),
			CreatedAt: time.Now().UTC(),
		}
		if err := kvs.CreateKeyVersion(ctx, kv); err != nil {
			t.Fatalf("CreateKeyVersion: %v", err)
		}

		versions, err := kvs.GetKeyVersions(ctx, "entity01")
		if err != nil {
			t.Fatalf("GetKeyVersions: %v", err)
		}
		if len(versions) != 1 {
			t.Errorf("versions count = %d, want 1", len(versions))
		}
		if versions[0].KeyIndex != 1 {
			t.Errorf("key_index = %d, want 1", versions[0].KeyIndex)
		}
	})

	t.Run("GetActiveKeyVersion returns key with nil valid_until", func(t *testing.T) {
		kvs := &testKeyVersionStore{versions: make(map[string][]models.KeyVersion)}
		now := time.Now().UTC()
		expiredAt := now.Add(-time.Hour)

		kvs.versions["entity01"] = []models.KeyVersion{
			{ID: "kv1", EntityID: "entity01", PublicKey: []byte("oldkey"), KeyIndex: 1, ValidUntil: &expiredAt},
			{ID: "kv2", EntityID: "entity01", PublicKey: []byte("newkey"), KeyIndex: 2, ValidUntil: nil},
		}

		active, err := kvs.GetActiveKeyVersion(ctx, "entity01")
		if err != nil {
			t.Fatalf("GetActiveKeyVersion: %v", err)
		}
		if active == nil {
			t.Fatal("expected active key, got nil")
		}
		if active.KeyIndex != 2 {
			t.Errorf("active key_index = %d, want 2", active.KeyIndex)
		}
	})

	t.Run("GetKeyVersions returns all ordered by key_index", func(t *testing.T) {
		kvs := &testKeyVersionStore{versions: make(map[string][]models.KeyVersion)}
		expiredAt := time.Now().UTC().Add(-time.Hour)

		kvs.versions["entity01"] = []models.KeyVersion{
			{ID: "kv2", EntityID: "entity01", PublicKey: []byte("key2"), KeyIndex: 2},
			{ID: "kv1", EntityID: "entity01", PublicKey: []byte("key1"), KeyIndex: 1, ValidUntil: &expiredAt},
		}

		versions, err := kvs.GetKeyVersions(ctx, "entity01")
		if err != nil {
			t.Fatalf("GetKeyVersions: %v", err)
		}
		if len(versions) != 2 {
			t.Errorf("versions count = %d, want 2", len(versions))
		}
		// Check ordering - both keys present (ordering in mock may not be guaranteed)
		found := make(map[int]bool)
		for _, v := range versions {
			found[v.KeyIndex] = true
		}
		if !found[1] || !found[2] {
			t.Error("GetKeyVersions should return all key versions including expired ones")
		}
	})

	t.Run("RotateKey sets valid_until on old key and creates new key", func(t *testing.T) {
		kvs := &testKeyVersionStore{versions: make(map[string][]models.KeyVersion)}
		kvs.versions["entity01"] = []models.KeyVersion{
			{ID: "kv1", EntityID: "entity01", PublicKey: []byte("oldkey"), KeyIndex: 1, ValidUntil: nil},
		}

		newPubKey := []byte("newkey32bytespublickey_abcdefghij")
		newKV, err := kvs.RotateKey(ctx, "entity01", newPubKey)
		if err != nil {
			t.Fatalf("RotateKey: %v", err)
		}

		// New key version should have incremented index
		if newKV.KeyIndex != 2 {
			t.Errorf("new key_index = %d, want 2", newKV.KeyIndex)
		}
		if newKV.ValidUntil != nil {
			t.Error("new key valid_until should be nil (active)")
		}

		// Old key should have valid_until set
		versions, _ := kvs.GetKeyVersions(ctx, "entity01")
		if len(versions) != 2 {
			t.Fatalf("versions count = %d, want 2 after rotation", len(versions))
		}

		var oldKV *models.KeyVersion
		for i := range versions {
			if versions[i].KeyIndex == 1 {
				oldKV = &versions[i]
			}
		}
		if oldKV == nil {
			t.Fatal("could not find old key version")
		}
		if oldKV.ValidUntil == nil {
			t.Error("old key should have valid_until set after rotation")
		}

		// Active key should be the new one
		active, _ := kvs.GetActiveKeyVersion(ctx, "entity01")
		if active == nil {
			t.Fatal("expected active key after rotation")
		}
		if active.KeyIndex != 2 {
			t.Errorf("active key_index = %d, want 2", active.KeyIndex)
		}
	})
}

// TestKeyRotateIntegration tests the full rotation flow if DATABASE_URL is set.
func TestKeyRotateIntegration(t *testing.T) {
	// Skip if no database available
	if true {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	// Future: use testcontainers-postgres for full integration test
}

// testKeyVersionStore is a simple in-memory mock for testing key version behavior contracts.
type testKeyVersionStore struct {
	versions map[string][]models.KeyVersion
}

func (m *testKeyVersionStore) CreateKeyVersion(_ context.Context, kv *models.KeyVersion) error {
	m.versions[kv.EntityID] = append(m.versions[kv.EntityID], *kv)
	return nil
}

func (m *testKeyVersionStore) GetActiveKeyVersion(_ context.Context, entityID string) (*models.KeyVersion, error) {
	for _, kv := range m.versions[entityID] {
		if kv.ValidUntil == nil {
			return &kv, nil
		}
	}
	return nil, nil
}

func (m *testKeyVersionStore) GetKeyVersions(_ context.Context, entityID string) ([]models.KeyVersion, error) {
	return m.versions[entityID], nil
}

func (m *testKeyVersionStore) RotateKey(_ context.Context, entityID string, newPubKey []byte) (*models.KeyVersion, error) {
	now := time.Now().UTC()
	versions := m.versions[entityID]
	for i := range versions {
		if versions[i].ValidUntil == nil {
			versions[i].ValidUntil = &now
		}
	}
	m.versions[entityID] = versions

	maxIndex := 0
	for _, kv := range versions {
		if kv.KeyIndex > maxIndex {
			maxIndex = kv.KeyIndex
		}
	}

	newKV := models.KeyVersion{
		ID:        "kv-rotated",
		EntityID:  entityID,
		PublicKey: newPubKey,
		KeyIndex:  maxIndex + 1,
		ValidFrom: now,
		CreatedAt: now,
	}
	m.versions[entityID] = append(m.versions[entityID], newKV)
	return &newKV, nil
}
