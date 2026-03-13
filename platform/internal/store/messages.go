package store

import (
	"context"
	"fmt"

	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// DIDCOMM MESSAGE QUEUE
// Implements didcomm.MessageStore on *Store.
// ============================================================

// QueueMessage inserts a DIDComm message into the queue with state "pending".
func (s *Store) QueueMessage(ctx context.Context, msg *models.DIDCommMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO didcomm_messages
			(id, recipient_did, sender_did, message_type, payload, state, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7)`,
		msg.ID,
		msg.RecipientDID,
		nullableString(msg.SenderDID),
		nullableString(msg.MessageType),
		msg.Payload,
		msg.CreatedAt,
		msg.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("queue message: %w", err)
	}
	return nil
}

// GetPendingMessages returns up to limit pending messages for the given recipient DID,
// ordered by created_at ascending (oldest first).
func (s *Store) GetPendingMessages(ctx context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, recipient_did, COALESCE(sender_did, ''), COALESCE(message_type, ''),
			payload, state, created_at, expires_at, delivered_at
		FROM didcomm_messages
		WHERE recipient_did = $1 AND state = 'pending'
		ORDER BY created_at ASC
		LIMIT $2`,
		recipientDID, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending messages: %w", err)
	}
	defer rows.Close()

	var msgs []models.DIDCommMessage
	for rows.Next() {
		var m models.DIDCommMessage
		if err := rows.Scan(
			&m.ID, &m.RecipientDID, &m.SenderDID, &m.MessageType,
			&m.Payload, &m.State, &m.CreatedAt, &m.ExpiresAt, &m.DeliveredAt,
		); err != nil {
			return nil, fmt.Errorf("get pending messages: scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get pending messages: rows: %w", err)
	}
	return msgs, nil
}

// MarkDelivered sets state to "delivered" and delivered_at = NOW() for the given message IDs.
// Only messages with state "pending" are affected.
func (s *Store) MarkDelivered(ctx context.Context, messageIDs []string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE didcomm_messages
		SET state = 'delivered', delivered_at = NOW()
		WHERE id = ANY($1) AND state = 'pending'`,
		messageIDs)
	if err != nil {
		return fmt.Errorf("mark delivered: %w", err)
	}
	return nil
}

// CleanupExpiredMessages deletes pending messages past their expires_at.
// Returns the count of deleted messages.
func (s *Store) CleanupExpiredMessages(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM didcomm_messages
		WHERE expires_at < NOW() AND state = 'pending'`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired messages: %w", err)
	}
	return tag.RowsAffected(), nil
}
