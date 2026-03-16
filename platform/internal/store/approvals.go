package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateApproval inserts a new approval record into the approvals table.
func (s *Store) CreateApproval(ctx context.Context, apr *models.Approval) error {
	doc, err := json.Marshal(apr)
	if err != nil {
		return fmt.Errorf("create approval: marshal document: %w", err)
	}

	var viaDID, parentID *string
	if apr.Via != "" {
		viaDID = &apr.Via
	}
	if apr.Parent != "" {
		parentID = &apr.Parent
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO approvals (id, state, from_did, to_did, via_did, parent_id, document, created_at, valid_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		apr.ID,
		models.ApprovalStateRequested,
		apr.From,
		apr.To,
		viaDID,
		parentID,
		doc,
		apr.CreatedAt,
		apr.ValidUntil,
	)
	if err != nil {
		return fmt.Errorf("create approval: %w", err)
	}
	return nil
}

// GetApprovals returns all approvals addressed to the given entity DID, ordered by created_at DESC.
func (s *Store) GetApprovals(ctx context.Context, entityDID string) ([]models.Approval, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, state, from_did, to_did, via_did, parent_id, document, created_at, valid_until, responded_at, updated_at
		FROM approvals
		WHERE to_did = $1
		ORDER BY created_at DESC`, entityDID)
	if err != nil {
		return nil, fmt.Errorf("get approvals: %w", err)
	}
	defer rows.Close()

	var approvals []models.Approval
	for rows.Next() {
		var (
			id, state, fromDID, toDID string
			viaDID, parentID          *string
			document                  []byte
			createdAt                 time.Time
			validUntil, respondedAt   *time.Time
			updatedAt                 time.Time
		)
		if err := rows.Scan(&id, &state, &fromDID, &toDID, &viaDID, &parentID, &document, &createdAt, &validUntil, &respondedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("get approvals: scan: %w", err)
		}

		var apr models.Approval
		if err := json.Unmarshal(document, &apr); err != nil {
			return nil, fmt.Errorf("get approvals: unmarshal document: %w", err)
		}

		// Overlay server-side fields from columns
		apr.State = state
		apr.RespondedAt = respondedAt
		apr.UpdatedAt = updatedAt

		approvals = append(approvals, apr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get approvals: rows: %w", err)
	}
	return approvals, nil
}

// UpdateApprovalState atomically transitions an approval's state from 'requested' to the new state.
// Returns (true, nil) if the update succeeded, (false, nil) if the approval was already responded to.
// This implements first-response-wins semantics.
func (s *Store) UpdateApprovalState(ctx context.Context, id, newState, responderSignature string) (bool, error) {
	var returnedID string
	err := s.pool.QueryRow(ctx, `
		UPDATE approvals
		SET state = $1,
		    responded_at = NOW(),
		    updated_at = NOW(),
		    document = document || jsonb_build_object('signatures', COALESCE(document->'signatures', '{}'::jsonb) || jsonb_build_object('to', $3::text))
		WHERE id = $2 AND state = 'requested'
		RETURNING id`, newState, id, responderSignature).Scan(&returnedID)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("update approval state: %w", err)
	}
	return true, nil
}

// RevokeApproval sets an approval's state to 'revoked' if it is currently 'requested' or 'approved'
// and belongs to the given entity DID.
// Returns (true, nil) if revoked, (false, nil) if not found or not in a revocable state.
func (s *Store) RevokeApproval(ctx context.Context, id, entityDID string) (bool, error) {
	var returnedID string
	err := s.pool.QueryRow(ctx, `
		UPDATE approvals
		SET state = 'revoked', updated_at = NOW()
		WHERE id = $1 AND to_did = $2 AND state IN ('requested', 'approved')
		RETURNING id`, id, entityDID).Scan(&returnedID)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("revoke approval: %w", err)
	}
	return true, nil
}
