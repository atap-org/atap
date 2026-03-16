package store

import (
	"context"
	"fmt"

	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateRevocation inserts a new revocation entry into the database.
// Returns an error if a revocation for the same approval_id already exists
// (enforced by UNIQUE constraint on approval_id column).
func (s *Store) CreateRevocation(ctx context.Context, r *models.Revocation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO revocations (id, approval_id, approver_did, revoked_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		r.ID, r.ApprovalID, r.ApproverDID, r.RevokedAt, r.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert revocation: %w", err)
	}
	return nil
}

// ListRevocations returns all non-expired revocations for the given approver DID.
// Results are ordered by revoked_at DESC.
func (s *Store) ListRevocations(ctx context.Context, approverDID string) ([]models.Revocation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, approval_id, approver_did, revoked_at, expires_at
		FROM revocations
		WHERE approver_did = $1 AND expires_at > NOW()
		ORDER BY revoked_at DESC`,
		approverDID)
	if err != nil {
		return nil, fmt.Errorf("list revocations: %w", err)
	}
	defer rows.Close()

	var revocations []models.Revocation
	for rows.Next() {
		var r models.Revocation
		if err := rows.Scan(&r.ID, &r.ApprovalID, &r.ApproverDID, &r.RevokedAt, &r.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan revocation row: %w", err)
		}
		revocations = append(revocations, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("revocations rows error: %w", err)
	}
	return revocations, nil
}

// CleanupExpiredRevocations deletes revocation entries where expires_at < NOW().
// Returns the number of rows deleted.
func (s *Store) CleanupExpiredRevocations(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM revocations WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired revocations: %w", err)
	}
	return tag.RowsAffected(), nil
}
