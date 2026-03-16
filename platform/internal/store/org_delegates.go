package store

import (
	"context"
	"fmt"

	"github.com/atap-dev/atap/platform/internal/models"
)

const maxOrgDelegateLimit = 50

// GetOrgDelegates returns up to limit entities whose principal_did matches orgDID
// and whose type is 'human', 'agent', or 'machine' (i.e. org-type entities are excluded).
//
// If limit exceeds 50, it is capped at 50 per spec Section 7.5.
// Returns a non-nil empty slice when no delegates are found.
func (s *Store) GetOrgDelegates(ctx context.Context, orgDID string, limit int) ([]models.Entity, error) {
	if limit <= 0 || limit > maxOrgDelegateLimit {
		limit = maxOrgDelegateLimit
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(did, ''), type, COALESCE(name, '')
		FROM entities
		WHERE principal_did = $1
		  AND type IN ('human', 'agent', 'machine')
		LIMIT $2`,
		orgDID, limit)
	if err != nil {
		return nil, fmt.Errorf("get org delegates: %w", err)
	}
	defer rows.Close()

	delegates := make([]models.Entity, 0)
	for rows.Next() {
		var e models.Entity
		if err := rows.Scan(&e.ID, &e.DID, &e.Type, &e.Name); err != nil {
			return nil, fmt.Errorf("scan org delegate: %w", err)
		}
		delegates = append(delegates, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate org delegates: %w", err)
	}

	return delegates, nil
}
