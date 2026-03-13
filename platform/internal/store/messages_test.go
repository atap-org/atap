package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/models"
)

// messageMockStore is an in-memory store implementing the MessageStore contract for testing.
type messageMockStore struct {
	messages map[string]*models.DIDCommMessage
}

func newMessageMockStore() *messageMockStore {
	return &messageMockStore{
		messages: make(map[string]*models.DIDCommMessage),
	}
}

func (m *messageMockStore) QueueMessage(_ context.Context, msg *models.DIDCommMessage) error {
	m.messages[msg.ID] = msg
	return nil
}

func (m *messageMockStore) GetPendingMessages(_ context.Context, recipientDID string, limit int) ([]models.DIDCommMessage, error) {
	var results []models.DIDCommMessage
	for _, msg := range m.messages {
		if msg.RecipientDID == recipientDID && msg.State == "pending" {
			results = append(results, *msg)
		}
	}
	// Sort by created_at ascending (simulate DB ORDER BY created_at ASC)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].CreatedAt.Before(results[j-1].CreatedAt); j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (m *messageMockStore) MarkDelivered(_ context.Context, messageIDs []string) error {
	now := time.Now()
	for _, id := range messageIDs {
		if msg, ok := m.messages[id]; ok && msg.State == "pending" {
			msg.State = "delivered"
			msg.DeliveredAt = &now
		}
	}
	return nil
}

func (m *messageMockStore) CleanupExpiredMessages(_ context.Context) (int64, error) {
	now := time.Now()
	var count int64
	for id, msg := range m.messages {
		if msg.ExpiresAt != nil && msg.ExpiresAt.Before(now) && msg.State == "pending" {
			delete(m.messages, id)
			count++
		}
	}
	return count, nil
}

// ============================================================
// CONTRACT TESTS
// These tests verify the expected contract for the MessageStore interface.
// ============================================================

func TestMessageStore_QueueAndGetPendingMessages(t *testing.T) {
	s := newMessageMockStore()
	ctx := context.Background()

	now := time.Now().UTC()
	msg1 := &models.DIDCommMessage{
		ID:           "msg_01",
		RecipientDID: "did:web:atap.app:agent:recipient01",
		SenderDID:    "did:web:atap.app:agent:sender01",
		MessageType:  "https://atap.dev/protocols/basic/1.0/ping",
		Payload:      []byte(`{"hello":"world"}`),
		State:        "pending",
		CreatedAt:    now,
	}
	msg2 := &models.DIDCommMessage{
		ID:           "msg_02",
		RecipientDID: "did:web:atap.app:agent:recipient01",
		SenderDID:    "did:web:atap.app:agent:sender01",
		MessageType:  "https://atap.dev/protocols/basic/1.0/ping",
		Payload:      []byte(`{"hello":"world2"}`),
		State:        "pending",
		CreatedAt:    now.Add(time.Second),
	}

	t.Run("QueueMessage stores message with state pending", func(t *testing.T) {
		if err := s.QueueMessage(ctx, msg1); err != nil {
			t.Fatalf("QueueMessage: %v", err)
		}
		if err := s.QueueMessage(ctx, msg2); err != nil {
			t.Fatalf("QueueMessage: %v", err)
		}
	})

	t.Run("GetPendingMessages retrieves messages in created_at order", func(t *testing.T) {
		msgs, err := s.GetPendingMessages(ctx, "did:web:atap.app:agent:recipient01", 100)
		if err != nil {
			t.Fatalf("GetPendingMessages: %v", err)
		}
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		// First message should be msg_01 (earlier created_at)
		if msgs[0].ID != "msg_01" {
			t.Errorf("msgs[0].ID = %q, want msg_01", msgs[0].ID)
		}
		if msgs[1].ID != "msg_02" {
			t.Errorf("msgs[1].ID = %q, want msg_02", msgs[1].ID)
		}
	})

	t.Run("GetPendingMessages respects limit", func(t *testing.T) {
		msgs, err := s.GetPendingMessages(ctx, "did:web:atap.app:agent:recipient01", 1)
		if err != nil {
			t.Fatalf("GetPendingMessages: %v", err)
		}
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message (limit=1), got %d", len(msgs))
		}
	})

	t.Run("GetPendingMessages returns empty for unknown DID", func(t *testing.T) {
		msgs, err := s.GetPendingMessages(ctx, "did:web:atap.app:agent:unknown", 100)
		if err != nil {
			t.Fatalf("GetPendingMessages: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected 0 messages for unknown DID, got %d", len(msgs))
		}
	})
}

func TestMessageStore_MarkDelivered(t *testing.T) {
	s := newMessageMockStore()
	ctx := context.Background()

	msg := &models.DIDCommMessage{
		ID:           "msg_deliver_01",
		RecipientDID: "did:web:atap.app:agent:recip02",
		SenderDID:    "did:web:atap.app:agent:sender02",
		MessageType:  "https://atap.dev/protocols/basic/1.0/ping",
		Payload:      []byte(`{"test":true}`),
		State:        "pending",
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.QueueMessage(ctx, msg); err != nil {
		t.Fatalf("setup: QueueMessage: %v", err)
	}

	t.Run("MarkDelivered changes state to delivered", func(t *testing.T) {
		if err := s.MarkDelivered(ctx, []string{"msg_deliver_01"}); err != nil {
			t.Fatalf("MarkDelivered: %v", err)
		}

		// Message should no longer appear in GetPendingMessages
		msgs, err := s.GetPendingMessages(ctx, "did:web:atap.app:agent:recip02", 100)
		if err != nil {
			t.Fatalf("GetPendingMessages after deliver: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected 0 pending messages after MarkDelivered, got %d", len(msgs))
		}

		// Verify state and delivered_at were set
		stored := s.messages["msg_deliver_01"]
		if stored.State != "delivered" {
			t.Errorf("state = %q, want delivered", stored.State)
		}
		if stored.DeliveredAt == nil {
			t.Error("delivered_at should be set after MarkDelivered")
		}
	})
}

func TestMessageStore_CleanupExpiredMessages(t *testing.T) {
	s := newMessageMockStore()
	ctx := context.Background()

	past := time.Now().UTC().Add(-1 * time.Hour)
	expiredMsg := &models.DIDCommMessage{
		ID:           "msg_expired_01",
		RecipientDID: "did:web:atap.app:agent:recip03",
		Payload:      []byte(`{}`),
		State:        "pending",
		CreatedAt:    time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:    &past,
	}
	future := time.Now().UTC().Add(1 * time.Hour)
	activeMsg := &models.DIDCommMessage{
		ID:           "msg_active_01",
		RecipientDID: "did:web:atap.app:agent:recip03",
		Payload:      []byte(`{}`),
		State:        "pending",
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    &future,
	}

	if err := s.QueueMessage(ctx, expiredMsg); err != nil {
		t.Fatalf("setup: QueueMessage expired: %v", err)
	}
	if err := s.QueueMessage(ctx, activeMsg); err != nil {
		t.Fatalf("setup: QueueMessage active: %v", err)
	}

	count, err := s.CleanupExpiredMessages(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredMessages: %v", err)
	}
	if count != 1 {
		t.Errorf("CleanupExpiredMessages returned count = %d, want 1", count)
	}

	// Active message should still be pending
	msgs, err := s.GetPendingMessages(ctx, "did:web:atap.app:agent:recip03", 100)
	if err != nil {
		t.Fatalf("GetPendingMessages after cleanup: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("expected 1 pending message after cleanup, got %d", len(msgs))
	}
	if msgs[0].ID != "msg_active_01" {
		t.Errorf("remaining message ID = %q, want msg_active_01", msgs[0].ID)
	}
}
