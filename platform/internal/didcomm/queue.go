package didcomm

import (
	"context"

	"github.com/atap-dev/atap/platform/internal/models"
)

// MessageStore defines the data access contract for DIDComm offline message delivery.
// It is implemented by *store.Store for production and by in-memory mocks for testing.
type MessageStore interface {
	// QueueMessage inserts a new message into the queue with state "pending".
	QueueMessage(ctx context.Context, msg *models.DIDCommMessage) error

	// GetPendingMessages returns up to limit messages for the given recipient DID
	// with state "pending", ordered by created_at ascending.
	GetPendingMessages(ctx context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error)

	// MarkDelivered updates state to "delivered" and sets delivered_at for the given message IDs.
	// Only messages with state "pending" are affected.
	MarkDelivered(ctx context.Context, messageIDs []string) error

	// CleanupExpiredMessages deletes messages past their expires_at with state "pending".
	// Returns the number of deleted messages.
	CleanupExpiredMessages(ctx context.Context) (int64, error)
}
