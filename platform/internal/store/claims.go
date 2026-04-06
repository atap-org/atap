package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateClaim inserts a new claim record.
func (s *Store) CreateClaim(ctx context.Context, cl *models.Claim) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO claims (id, code, agent_id, agent_name, description, scopes, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		cl.ID, cl.Code, cl.AgentID, cl.AgentName, cl.Description,
		cl.Scopes, cl.Status, cl.CreatedAt, cl.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create claim: %w", err)
	}
	return nil
}

// GetClaimByCode retrieves a claim by its short code (e.g. "ATAP-7X9K").
func (s *Store) GetClaimByCode(ctx context.Context, code string) (*models.Claim, error) {
	cl := &models.Claim{}
	var redeemedBy *string
	var redeemedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, code, agent_id, agent_name, description, scopes,
			status, redeemed_by, created_at, redeemed_at, expires_at
		FROM claims WHERE code = $1`, code).Scan(
		&cl.ID, &cl.Code, &cl.AgentID, &cl.AgentName, &cl.Description, &cl.Scopes,
		&cl.Status, &redeemedBy, &cl.CreatedAt, &redeemedAt, &cl.ExpiresAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get claim by code: %w", err)
	}
	if redeemedBy != nil {
		cl.RedeemedBy = *redeemedBy
	}
	cl.RedeemedAt = redeemedAt
	return cl, nil
}

// RedeemClaim atomically transitions a claim from 'pending' to 'redeemed'
// and sets the human entity ID. Returns (true, nil) on success,
// (false, nil) if already redeemed or not pending.
func (s *Store) RedeemClaim(ctx context.Context, code, humanEntityID string) (bool, error) {
	var returnedID string
	err := s.pool.QueryRow(ctx, `
		UPDATE claims
		SET status = 'redeemed', redeemed_by = $1, redeemed_at = NOW()
		WHERE code = $2 AND status = 'pending' AND expires_at > NOW()
		RETURNING id`, humanEntityID, code).Scan(&returnedID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("redeem claim: %w", err)
	}
	return true, nil
}

// SetEntityPrincipalDID updates the principal_did on an agent entity
// after a claim is redeemed.
func (s *Store) SetEntityPrincipalDID(ctx context.Context, entityID, principalDID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE entities SET principal_did = $1, updated_at = NOW()
		WHERE id = $2`, principalDID, entityID)
	if err != nil {
		return fmt.Errorf("set principal did: %w", err)
	}
	return nil
}

// GetEntityByEmail finds a human entity that has been associated with a given email
// via the claim flow. We look up by the Redis-stored session, not by a DB column,
// so this is a helper for the claim approve handler that looks up by entity ID.
// (Email is not stored on the entity — it's an attestation.)

// CleanupExpiredClaims marks expired pending claims.
func (s *Store) CleanupExpiredClaims(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE claims SET status = 'expired'
		WHERE status = 'pending' AND expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired claims: %w", err)
	}
	return tag.RowsAffected(), nil
}
