package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateApproval inserts a new approval into the database. The full Approval
// struct (excluding server-side json:"-" fields) is serialized as JSONB into
// the document column. State defaults to "requested" if empty.
func (s *Store) CreateApproval(ctx context.Context, a *models.Approval) error {
	if a.State == "" {
		a.State = models.ApprovalStateRequested
	}

	doc, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal approval document: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO approvals (id, state, from_did, to_did, via_did, parent_id,
			document, created_at, valid_until, responded_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		a.ID, a.State, a.From, a.To,
		nullableString(a.Via), nullableString(a.Parent),
		doc, a.CreatedAt, a.ValidUntil, a.RespondedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert approval: %w", err)
	}
	return nil
}

// GetApproval retrieves an approval by ID. Returns nil, nil if not found.
// Server-side fields (State, RespondedAt, UpdatedAt) are read from their
// dedicated columns and overlaid onto the deserialized document.
func (s *Store) GetApproval(ctx context.Context, id string) (*models.Approval, error) {
	var doc []byte
	var state string
	var respondedAt *time.Time
	var updatedAt time.Time

	err := s.pool.QueryRow(ctx, `
		SELECT document, state, responded_at, updated_at
		FROM approvals WHERE id = $1`, id).Scan(&doc, &state, &respondedAt, &updatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get approval: %w", err)
	}

	a := &models.Approval{}
	if err := json.Unmarshal(doc, a); err != nil {
		return nil, fmt.Errorf("unmarshal approval document: %w", err)
	}

	// Overlay server-side fields from dedicated columns.
	a.State = state
	a.RespondedAt = respondedAt
	a.UpdatedAt = updatedAt

	return a, nil
}

// UpdateApprovalState transitions an approval to a new state and optionally
// sets responded_at (e.g., when approved or declined).
func (s *Store) UpdateApprovalState(ctx context.Context, id, state string, respondedAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE approvals
		SET state = $2, responded_at = $3, updated_at = NOW()
		WHERE id = $1`, id, state, respondedAt)
	if err != nil {
		return fmt.Errorf("update approval state: %w", err)
	}
	return nil
}

// ConsumeApproval atomically transitions a one-time approval (valid_until IS NULL)
// from "approved" to "consumed". Returns (true, nil) if the approval was consumed,
// (false, nil) if it was already consumed or is not a one-time approval.
// This atomic check-and-set prevents race conditions on concurrent consume attempts.
func (s *Store) ConsumeApproval(ctx context.Context, id string) (consumed bool, err error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE approvals
		SET state = 'consumed', updated_at = NOW()
		WHERE id = $1 AND state = 'approved' AND valid_until IS NULL`, id)
	if err != nil {
		return false, fmt.Errorf("consume approval: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListApprovals returns approvals involving the given entity DID (as from, to, or via).
// Results are ordered by created_at DESC with limit/offset pagination.
func (s *Store) ListApprovals(ctx context.Context, entityDID string, limit, offset int) ([]models.Approval, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT document, state, responded_at, updated_at
		FROM approvals
		WHERE from_did = $1 OR to_did = $1 OR via_did = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, entityDID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	return scanApprovalRows(rows)
}

// GetChildApprovals returns all descendants of a parent approval using a
// recursive CTE to traverse the full approval chain (spec APR-11).
func (s *Store) GetChildApprovals(ctx context.Context, parentID string) ([]models.Approval, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE descendants AS (
			SELECT id FROM approvals WHERE parent_id = $1
			UNION ALL
			SELECT a.id FROM approvals a JOIN descendants d ON a.parent_id = d.id
		)
		SELECT document, state, responded_at, updated_at
		FROM approvals
		WHERE id IN (SELECT id FROM descendants)`, parentID)
	if err != nil {
		return nil, fmt.Errorf("get child approvals: %w", err)
	}
	defer rows.Close()

	return scanApprovalRows(rows)
}

// RevokeWithChildren atomically marks the given approval and all descendants
// as "revoked" using a recursive CTE (spec APR-11).
func (s *Store) RevokeWithChildren(ctx context.Context, parentID string) error {
	_, err := s.pool.Exec(ctx, `
		WITH RECURSIVE descendants AS (
			SELECT id FROM approvals WHERE id = $1
			UNION ALL
			SELECT a.id FROM approvals a JOIN descendants d ON a.parent_id = d.id
		)
		UPDATE approvals
		SET state = 'revoked', updated_at = NOW()
		WHERE id IN (SELECT id FROM descendants)`, parentID)
	if err != nil {
		return fmt.Errorf("revoke with children: %w", err)
	}
	return nil
}

// CleanupExpiredApprovals marks unanswered requested approvals as expired if
// they were created more than 24 hours ago (spec §8.16). Returns rows affected.
func (s *Store) CleanupExpiredApprovals(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE approvals
		SET state = 'expired', updated_at = NOW()
		WHERE state = 'requested' AND created_at < NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired approvals: %w", err)
	}
	return tag.RowsAffected(), nil
}

// scanApprovalRows is a shared helper for reading rows of (document, state,
// responded_at, updated_at) into a slice of Approval structs.
func scanApprovalRows(rows pgx.Rows) ([]models.Approval, error) {
	var approvals []models.Approval
	for rows.Next() {
		var doc []byte
		var state string
		var respondedAt *time.Time
		var updatedAt time.Time

		if err := rows.Scan(&doc, &state, &respondedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan approval row: %w", err)
		}

		a := models.Approval{}
		if err := json.Unmarshal(doc, &a); err != nil {
			return nil, fmt.Errorf("unmarshal approval document: %w", err)
		}
		a.State = state
		a.RespondedAt = respondedAt
		a.UpdatedAt = updatedAt

		approvals = append(approvals, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("approvals rows error: %w", err)
	}
	return approvals, nil
}
