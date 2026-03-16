package store_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// credentialMockStore is an in-memory store implementing the credential store contract.
type credentialMockStore struct {
	encKeys     map[string][]byte
	credentials map[string]*models.Credential
	statusLists map[string]*models.CredentialStatusList
}

func newCredentialMockStore() *credentialMockStore {
	return &credentialMockStore{
		encKeys:     make(map[string][]byte),
		credentials: make(map[string]*models.Credential),
		statusLists: make(map[string]*models.CredentialStatusList),
	}
}

func (m *credentialMockStore) CreateEncKey(_ context.Context, entityID string, key []byte) error {
	cp := make([]byte, len(key))
	copy(cp, key)
	m.encKeys[entityID] = cp
	return nil
}

func (m *credentialMockStore) GetEncKey(_ context.Context, entityID string) ([]byte, error) {
	key, ok := m.encKeys[entityID]
	if !ok {
		return nil, nil
	}
	cp := make([]byte, len(key))
	copy(cp, key)
	return cp, nil
}

func (m *credentialMockStore) DeleteEncKey(_ context.Context, entityID string) error {
	delete(m.encKeys, entityID)
	return nil
}

func (m *credentialMockStore) CreateCredential(_ context.Context, cred *models.Credential) error {
	cp := *cred
	ct := make([]byte, len(cred.CredentialCT))
	copy(ct, cred.CredentialCT)
	cp.CredentialCT = ct
	m.credentials[cred.ID] = &cp
	return nil
}

func (m *credentialMockStore) GetCredentials(_ context.Context, entityID string) ([]models.Credential, error) {
	var results []models.Credential
	for _, c := range m.credentials {
		if c.EntityID == entityID {
			results = append(results, *c)
		}
	}
	return results, nil
}

func (m *credentialMockStore) RevokeCredential(_ context.Context, id string) error {
	c, ok := m.credentials[id]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	c.RevokedAt = &now
	return nil
}

func (m *credentialMockStore) GetStatusList(_ context.Context, listID string) (*models.CredentialStatusList, error) {
	sl, ok := m.statusLists[listID]
	if !ok {
		return nil, nil
	}
	cp := *sl
	bits := make([]byte, len(sl.Bits))
	copy(bits, sl.Bits)
	cp.Bits = bits
	return &cp, nil
}

func (m *credentialMockStore) UpdateStatusListBit(_ context.Context, listID string, index int) error {
	sl, ok := m.statusLists[listID]
	if !ok {
		return nil
	}
	sl.Bits[index/8] |= (1 << (7 - uint(index%8)))
	sl.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *credentialMockStore) GetNextStatusIndex(_ context.Context, listID string) (int, error) {
	sl, ok := m.statusLists[listID]
	if !ok {
		return 0, nil
	}
	idx := sl.NextIndex
	sl.NextIndex++
	return idx, nil
}

// ============================================================
// TESTS
// ============================================================

func TestEncKey_CreateAndGet(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	entityID := "entity_enckey01"
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	t.Run("CreateEncKey stores key", func(t *testing.T) {
		if err := s.CreateEncKey(ctx, entityID, key); err != nil {
			t.Fatalf("CreateEncKey: %v", err)
		}
	})

	t.Run("GetEncKey retrieves stored key", func(t *testing.T) {
		got, err := s.GetEncKey(ctx, entityID)
		if err != nil {
			t.Fatalf("GetEncKey: %v", err)
		}
		if len(got) != 32 {
			t.Errorf("key length = %d, want 32", len(got))
		}
		for i, b := range key {
			if got[i] != b {
				t.Errorf("key[%d] = %d, want %d", i, got[i], b)
				break
			}
		}
	})
}

func TestEncKey_DeleteKey(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	entityID := "entity_enckey02"
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	if err := s.CreateEncKey(ctx, entityID, key); err != nil {
		t.Fatalf("CreateEncKey: %v", err)
	}

	t.Run("DeleteEncKey removes key", func(t *testing.T) {
		if err := s.DeleteEncKey(ctx, entityID); err != nil {
			t.Fatalf("DeleteEncKey: %v", err)
		}

		got, err := s.GetEncKey(ctx, entityID)
		if err != nil {
			t.Fatalf("GetEncKey after delete: %v", err)
		}
		if got != nil {
			t.Error("expected nil after DeleteEncKey, got non-nil")
		}
	})
}

func TestEncKey_GetNonExistent(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	t.Run("GetEncKey returns nil for unknown entity", func(t *testing.T) {
		got, err := s.GetEncKey(ctx, "no_such_entity")
		if err != nil {
			t.Fatalf("GetEncKey: %v", err)
		}
		if got != nil {
			t.Error("expected nil for non-existent entity, got non-nil")
		}
	})
}

func TestCredential_CreateAndGet(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	entityID := "entity_cred01"
	now := time.Now().UTC()
	fakeCT := []byte("encrypted-jwt-payload-bytes")

	cred := &models.Credential{
		ID:           models.CredentialIDPrefix + "01TESTULID00000001",
		EntityID:     entityID,
		Type:         "ATAPEmailVerification",
		StatusIndex:  0,
		StatusListID: "1",
		CredentialCT: fakeCT,
		IssuedAt:     now,
	}

	t.Run("CreateCredential inserts credential", func(t *testing.T) {
		if err := s.CreateCredential(ctx, cred); err != nil {
			t.Fatalf("CreateCredential: %v", err)
		}
	})

	t.Run("GetCredentials returns credentials for entity", func(t *testing.T) {
		creds, err := s.GetCredentials(ctx, entityID)
		if err != nil {
			t.Fatalf("GetCredentials: %v", err)
		}
		if len(creds) != 1 {
			t.Fatalf("expected 1 credential, got %d", len(creds))
		}
		if creds[0].ID != cred.ID {
			t.Errorf("ID = %q, want %q", creds[0].ID, cred.ID)
		}
		if creds[0].Type != "ATAPEmailVerification" {
			t.Errorf("Type = %q, want ATAPEmailVerification", creds[0].Type)
		}
		if len(creds[0].CredentialCT) != len(fakeCT) {
			t.Errorf("CredentialCT length = %d, want %d", len(creds[0].CredentialCT), len(fakeCT))
		}
	})
}

func TestCredential_Revoke(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	entityID := "entity_cred02"
	now := time.Now().UTC()
	cred := &models.Credential{
		ID:           models.CredentialIDPrefix + "01TESTULID00000002",
		EntityID:     entityID,
		Type:         "ATAPPhoneVerification",
		StatusIndex:  1,
		StatusListID: "1",
		CredentialCT: []byte("fake-ct"),
		IssuedAt:     now,
	}

	if err := s.CreateCredential(ctx, cred); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	t.Run("RevokeCredential sets revoked_at", func(t *testing.T) {
		if err := s.RevokeCredential(ctx, cred.ID); err != nil {
			t.Fatalf("RevokeCredential: %v", err)
		}

		creds, err := s.GetCredentials(ctx, entityID)
		if err != nil {
			t.Fatalf("GetCredentials after revoke: %v", err)
		}
		if len(creds) == 0 {
			t.Fatal("expected credentials after revoke")
		}
		if creds[0].RevokedAt == nil {
			t.Error("expected RevokedAt to be set after revoke")
		}
	})
}

func TestStatusList_GetAndUpdate(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	// Seed a status list
	bits := make([]byte, 16384)
	now := time.Now().UTC()
	s.statusLists["1"] = &models.CredentialStatusList{
		ID:        "1",
		Bits:      bits,
		NextIndex: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("GetStatusList returns raw bits", func(t *testing.T) {
		sl, err := s.GetStatusList(ctx, "1")
		if err != nil {
			t.Fatalf("GetStatusList: %v", err)
		}
		if sl == nil {
			t.Fatal("expected status list, got nil")
		}
		if len(sl.Bits) != 16384 {
			t.Errorf("bits length = %d, want 16384", len(sl.Bits))
		}
	})

	t.Run("UpdateStatusListBit sets bit at index", func(t *testing.T) {
		if err := s.UpdateStatusListBit(ctx, "1", 42); err != nil {
			t.Fatalf("UpdateStatusListBit: %v", err)
		}

		sl, err := s.GetStatusList(ctx, "1")
		if err != nil {
			t.Fatalf("GetStatusList after bit set: %v", err)
		}

		// Bit 42: byte index 42/8=5, bit offset 7-(42%8)=7-2=5
		byteIdx := 42 / 8
		bitMask := byte(1 << (7 - uint(42%8)))
		if sl.Bits[byteIdx]&bitMask == 0 {
			t.Error("expected bit 42 to be set")
		}
	})
}

func TestStatusList_GetNextStatusIndex(t *testing.T) {
	s := newCredentialMockStore()
	ctx := context.Background()

	bits := make([]byte, 16384)
	now := time.Now().UTC()
	s.statusLists["1"] = &models.CredentialStatusList{
		ID:        "1",
		Bits:      bits,
		NextIndex: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	t.Run("GetNextStatusIndex atomically increments and returns old value", func(t *testing.T) {
		idx0, err := s.GetNextStatusIndex(ctx, "1")
		if err != nil {
			t.Fatalf("GetNextStatusIndex (first): %v", err)
		}
		if idx0 != 0 {
			t.Errorf("first index = %d, want 0", idx0)
		}

		idx1, err := s.GetNextStatusIndex(ctx, "1")
		if err != nil {
			t.Fatalf("GetNextStatusIndex (second): %v", err)
		}
		if idx1 != 1 {
			t.Errorf("second index = %d, want 1", idx1)
		}

		idx2, err := s.GetNextStatusIndex(ctx, "1")
		if err != nil {
			t.Fatalf("GetNextStatusIndex (third): %v", err)
		}
		if idx2 != 2 {
			t.Errorf("third index = %d, want 2", idx2)
		}
	})
}
