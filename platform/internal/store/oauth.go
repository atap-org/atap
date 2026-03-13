package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// OAUTH TOKEN STORE
// ============================================================

// CreateOAuthToken inserts a new OAuth token record into the oauth_tokens table.
func (s *Store) CreateOAuthToken(ctx context.Context, token *models.OAuthToken) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_tokens (id, entity_id, token_type, scope, dpop_jkt, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.ID, token.EntityID, token.TokenType, token.Scope, token.DPoPJKT,
		token.ExpiresAt, token.CreatedAt)
	if err != nil {
		return fmt.Errorf("create oauth token: %w", err)
	}
	return nil
}

// GetOAuthToken retrieves an active (non-expired, non-revoked) OAuth token by its JTI.
// Returns nil if the token does not exist, is expired, or has been revoked.
func (s *Store) GetOAuthToken(ctx context.Context, tokenID string) (*models.OAuthToken, error) {
	t := &models.OAuthToken{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, entity_id, token_type, scope, dpop_jkt, expires_at, revoked_at, created_at
		FROM oauth_tokens
		WHERE id = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenID).Scan(
		&t.ID, &t.EntityID, &t.TokenType, &t.Scope, &t.DPoPJKT,
		&t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get oauth token: %w", err)
	}
	return t, nil
}

// RevokeOAuthToken sets revoked_at to the current timestamp for the given token ID.
func (s *Store) RevokeOAuthToken(ctx context.Context, tokenID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE oauth_tokens SET revoked_at = NOW() WHERE id = $1`,
		tokenID)
	if err != nil {
		return fmt.Errorf("revoke oauth token: %w", err)
	}
	return nil
}

// CleanupExpiredTokens removes tokens that expired more than 24 hours ago.
// Returns the number of rows deleted.
func (s *Store) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM oauth_tokens WHERE expires_at < NOW() - interval '1 day'`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ============================================================
// OAUTH AUTH CODE STORE
// ============================================================

// CreateAuthCode inserts a new authorization code into the oauth_auth_codes table.
func (s *Store) CreateAuthCode(ctx context.Context, code *models.OAuthAuthCode) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_auth_codes (code, entity_id, redirect_uri, scope, code_challenge, dpop_jkt, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		code.Code, code.EntityID, code.RedirectURI, code.Scope, code.CodeChallenge,
		code.DPoPJKT, code.ExpiresAt, code.CreatedAt)
	if err != nil {
		return fmt.Errorf("create auth code: %w", err)
	}
	return nil
}

// RedeemAuthCode atomically marks the authorization code as used (single-use) and returns it.
// Returns nil if the code does not exist, has already been used, or has expired.
// Uses UPDATE ... RETURNING for atomic single-use enforcement (prevents race conditions).
func (s *Store) RedeemAuthCode(ctx context.Context, code string) (*models.OAuthAuthCode, error) {
	c := &models.OAuthAuthCode{}
	err := s.pool.QueryRow(ctx, `
		UPDATE oauth_auth_codes
		SET used_at = NOW()
		WHERE code = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING code, entity_id, redirect_uri, scope, code_challenge, dpop_jkt, expires_at, used_at, created_at`,
		code).Scan(
		&c.Code, &c.EntityID, &c.RedirectURI, &c.Scope, &c.CodeChallenge,
		&c.DPoPJKT, &c.ExpiresAt, &c.UsedAt, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redeem auth code: %w", err)
	}
	return c, nil
}
