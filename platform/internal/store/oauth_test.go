package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// oauthMockStore is an in-memory store implementing OAuthTokenStore for testing
// the store contract behavior without a real database.
type oauthMockStore struct {
	tokens map[string]*models.OAuthToken
	codes  map[string]*models.OAuthAuthCode
}

func newOAuthMockStore() *oauthMockStore {
	return &oauthMockStore{
		tokens: make(map[string]*models.OAuthToken),
		codes:  make(map[string]*models.OAuthAuthCode),
	}
}

func (m *oauthMockStore) CreateOAuthToken(_ context.Context, token *models.OAuthToken) error {
	m.tokens[token.ID] = token
	return nil
}

func (m *oauthMockStore) GetOAuthToken(_ context.Context, tokenID string) (*models.OAuthToken, error) {
	t, ok := m.tokens[tokenID]
	if !ok {
		return nil, nil
	}
	// Simulate the DB query: WHERE revoked_at IS NULL AND expires_at > NOW()
	if t.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	if t.RevokedAt != nil {
		return nil, nil
	}
	return t, nil
}

func (m *oauthMockStore) RevokeOAuthToken(_ context.Context, tokenID string) error {
	if t, ok := m.tokens[tokenID]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (m *oauthMockStore) CreateAuthCode(_ context.Context, code *models.OAuthAuthCode) error {
	m.codes[code.Code] = code
	return nil
}

func (m *oauthMockStore) RedeemAuthCode(_ context.Context, code string) (*models.OAuthAuthCode, error) {
	c, ok := m.codes[code]
	if !ok {
		return nil, nil
	}
	// Simulate atomic single-use: WHERE used_at IS NULL AND expires_at > NOW()
	if c.UsedAt != nil {
		return nil, nil
	}
	if c.ExpiresAt.Before(time.Now()) {
		return nil, nil
	}
	now := time.Now()
	c.UsedAt = &now
	return c, nil
}

func (m *oauthMockStore) CleanupExpiredTokens(_ context.Context) (int64, error) {
	var count int64
	now := time.Now()
	for id, t := range m.tokens {
		if t.ExpiresAt.Before(now.Add(-24 * time.Hour)) {
			delete(m.tokens, id)
			count++
		}
	}
	return count, nil
}

// ============================================================
// CONTRACT TESTS
// These tests document and verify the expected contract for the
// OAuthTokenStore interface (used by both mock and real store).
// ============================================================

func TestOAuthStore_CreateAndGetToken(t *testing.T) {
	s := newOAuthMockStore()

	now := time.Now().UTC()
	token := &models.OAuthToken{
		ID:        "test-token-jti-001",
		EntityID:  "entity-001",
		TokenType: "access",
		Scope:     []string{"atap:inbox", "atap:send"},
		DPoPJKT:   "test-thumbprint-abc123",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}

	t.Run("CreateOAuthToken stores token with correct fields", func(t *testing.T) {
		if err := s.CreateOAuthToken(context.Background(), token); err != nil {
			t.Fatalf("CreateOAuthToken: %v", err)
		}
	})

	t.Run("GetOAuthToken retrieves non-expired non-revoked token", func(t *testing.T) {
		got, err := s.GetOAuthToken(context.Background(), "test-token-jti-001")
		if err != nil {
			t.Fatalf("GetOAuthToken: %v", err)
		}
		if got == nil {
			t.Fatal("GetOAuthToken: expected token, got nil")
		}
		if got.ID != token.ID {
			t.Errorf("ID = %q, want %q", got.ID, token.ID)
		}
		if got.EntityID != token.EntityID {
			t.Errorf("EntityID = %q, want %q", got.EntityID, token.EntityID)
		}
		if got.DPoPJKT != token.DPoPJKT {
			t.Errorf("DPoPJKT = %q, want %q", got.DPoPJKT, token.DPoPJKT)
		}
		if len(got.Scope) != 2 {
			t.Errorf("Scope length = %d, want 2", len(got.Scope))
		}
	})

	t.Run("GetOAuthToken returns nil for nonexistent token", func(t *testing.T) {
		got, err := s.GetOAuthToken(context.Background(), "nonexistent-token")
		if err != nil {
			t.Fatalf("GetOAuthToken: unexpected error: %v", err)
		}
		if got != nil {
			t.Error("expected nil for nonexistent token")
		}
	})
}

func TestOAuthStore_GetToken_ExpiredToken(t *testing.T) {
	s := newOAuthMockStore()

	expiredToken := &models.OAuthToken{
		ID:        "expired-token-001",
		EntityID:  "entity-001",
		TokenType: "access",
		Scope:     []string{"atap:inbox"},
		DPoPJKT:   "thumbprint-xyz",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := s.CreateOAuthToken(context.Background(), expiredToken); err != nil {
		t.Fatalf("setup: CreateOAuthToken: %v", err)
	}

	got, err := s.GetOAuthToken(context.Background(), "expired-token-001")
	if err != nil {
		t.Fatalf("GetOAuthToken: unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for expired token, got token")
	}
}

func TestOAuthStore_GetToken_RevokedToken(t *testing.T) {
	s := newOAuthMockStore()

	token := &models.OAuthToken{
		ID:        "revoke-me-token",
		EntityID:  "entity-001",
		TokenType: "access",
		Scope:     []string{"atap:inbox"},
		DPoPJKT:   "thumbprint-rev",
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateOAuthToken(context.Background(), token); err != nil {
		t.Fatalf("setup: CreateOAuthToken: %v", err)
	}

	if err := s.RevokeOAuthToken(context.Background(), "revoke-me-token"); err != nil {
		t.Fatalf("RevokeOAuthToken: %v", err)
	}

	got, err := s.GetOAuthToken(context.Background(), "revoke-me-token")
	if err != nil {
		t.Fatalf("GetOAuthToken: unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for revoked token, got token")
	}
}

func TestOAuthStore_CreateAndRedeemAuthCode(t *testing.T) {
	s := newOAuthMockStore()

	code := &models.OAuthAuthCode{
		Code:          "test-auth-code-abc123",
		EntityID:      "entity-001",
		RedirectURI:   "atap://callback",
		Scope:         []string{"atap:revoke"},
		CodeChallenge: "challenge-abc",
		DPoPJKT:       "thumbprint-human",
		ExpiresAt:     time.Now().UTC().Add(10 * time.Minute),
		CreatedAt:     time.Now().UTC(),
	}

	t.Run("CreateAuthCode stores code", func(t *testing.T) {
		if err := s.CreateAuthCode(context.Background(), code); err != nil {
			t.Fatalf("CreateAuthCode: %v", err)
		}
	})

	t.Run("RedeemAuthCode returns code on first use", func(t *testing.T) {
		got, err := s.RedeemAuthCode(context.Background(), "test-auth-code-abc123")
		if err != nil {
			t.Fatalf("RedeemAuthCode: %v", err)
		}
		if got == nil {
			t.Fatal("RedeemAuthCode: expected code, got nil")
		}
		if got.Code != code.Code {
			t.Errorf("Code = %q, want %q", got.Code, code.Code)
		}
		if got.DPoPJKT != code.DPoPJKT {
			t.Errorf("DPoPJKT = %q, want %q", got.DPoPJKT, code.DPoPJKT)
		}
	})

	t.Run("RedeemAuthCode returns nil on second use (single-use)", func(t *testing.T) {
		got, err := s.RedeemAuthCode(context.Background(), "test-auth-code-abc123")
		if err != nil {
			t.Fatalf("RedeemAuthCode: unexpected error: %v", err)
		}
		if got != nil {
			t.Error("expected nil for already-used code")
		}
	})
}

func TestOAuthStore_RedeemAuthCode_ExpiredCode(t *testing.T) {
	s := newOAuthMockStore()

	expiredCode := &models.OAuthAuthCode{
		Code:          "expired-code-001",
		EntityID:      "entity-001",
		RedirectURI:   "atap://callback",
		Scope:         []string{"atap:revoke"},
		CodeChallenge: "challenge-exp",
		DPoPJKT:       "thumbprint-exp",
		ExpiresAt:     time.Now().UTC().Add(-1 * time.Minute),
		CreatedAt:     time.Now().UTC().Add(-15 * time.Minute),
	}
	if err := s.CreateAuthCode(context.Background(), expiredCode); err != nil {
		t.Fatalf("setup: CreateAuthCode: %v", err)
	}

	got, err := s.RedeemAuthCode(context.Background(), "expired-code-001")
	if err != nil {
		t.Fatalf("RedeemAuthCode: unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for expired code")
	}
}
