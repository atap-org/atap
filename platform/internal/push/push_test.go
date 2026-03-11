package push

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/models"
)

// mockFCMClient records calls and can return errors.
type mockFCMClient struct {
	sentMessages []*messaging.Message
	err          error
}

func (m *mockFCMClient) Send(ctx context.Context, message *messaging.Message) (string, error) {
	m.sentMessages = append(m.sentMessages, message)
	if m.err != nil {
		return "", m.err
	}
	return "projects/test/messages/123", nil
}

// mockPushTokenStore returns configurable push tokens.
type mockPushTokenStore struct {
	tokens map[string]*models.PushToken
}

func (m *mockPushTokenStore) GetPushToken(_ context.Context, entityID string) (*models.PushToken, error) {
	pt, ok := m.tokens[entityID]
	if !ok {
		return nil, nil
	}
	return pt, nil
}

func TestSendNotification(t *testing.T) {
	testSignal := &models.Signal{
		ID:      "sig_test123",
		Version: "1",
		Route: models.SignalRoute{
			Origin: "agent://abc123",
			Target: "human://xyz789",
		},
		Signal: models.SignalBody{
			Type: "task.completed",
			Data: json.RawMessage(`{"result":"ok"}`),
		},
	}

	tests := []struct {
		name           string
		entityID       string
		signal         *models.Signal
		tokens         map[string]*models.PushToken
		fcmErr         error
		wantSendCalls  int
		wantTitle      string
		wantBody       string
	}{
		{
			name:     "valid push token sends FCM notification",
			entityID: "entity1",
			signal:   testSignal,
			tokens: map[string]*models.PushToken{
				"entity1": {EntityID: "entity1", Token: "fcm-token-abc", Platform: "android"},
			},
			wantSendCalls: 1,
			wantTitle:     "task.completed",
			wantBody:      "from agent://abc123",
		},
		{
			name:     "no push token registered does nothing",
			entityID: "entity-no-token",
			signal:   testSignal,
			tokens:   map[string]*models.PushToken{},
			wantSendCalls: 0,
		},
		{
			name:     "FCM send error does not propagate",
			entityID: "entity2",
			signal:   testSignal,
			tokens: map[string]*models.PushToken{
				"entity2": {EntityID: "entity2", Token: "fcm-token-xyz", Platform: "ios"},
			},
			fcmErr:        errors.New("FCM unavailable"),
			wantSendCalls: 1,
		},
		{
			name:     "notification content format matches spec",
			entityID: "entity3",
			signal: &models.Signal{
				ID: "sig_format",
				Route: models.SignalRoute{
					Origin: "machine://m1",
					Target: "human://h1",
				},
				Signal: models.SignalBody{
					Type: "alert.security",
				},
			},
			tokens: map[string]*models.PushToken{
				"entity3": {EntityID: "entity3", Token: "fcm-token-fmt", Platform: "android"},
			},
			wantSendCalls: 1,
			wantTitle:     "alert.security",
			wantBody:      "from machine://m1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fcm := &mockFCMClient{err: tt.fcmErr}
			store := &mockPushTokenStore{tokens: tt.tokens}
			log := zerolog.Nop()

			svc := NewPushService(fcm, store, log)
			svc.SendNotification(context.Background(), tt.entityID, tt.signal)

			if len(fcm.sentMessages) != tt.wantSendCalls {
				t.Fatalf("expected %d FCM send calls, got %d", tt.wantSendCalls, len(fcm.sentMessages))
			}

			if tt.wantSendCalls > 0 && tt.wantTitle != "" {
				msg := fcm.sentMessages[0]
				if msg.Notification.Title != tt.wantTitle {
					t.Errorf("title = %q, want %q", msg.Notification.Title, tt.wantTitle)
				}
				if msg.Notification.Body != tt.wantBody {
					t.Errorf("body = %q, want %q", msg.Notification.Body, tt.wantBody)
				}

				// Verify data fields
				if msg.Data["signal_id"] != tt.signal.ID {
					t.Errorf("data.signal_id = %q, want %q", msg.Data["signal_id"], tt.signal.ID)
				}
				if msg.Data["type"] != tt.signal.Signal.Type {
					t.Errorf("data.type = %q, want %q", msg.Data["type"], tt.signal.Signal.Type)
				}
			}
		})
	}
}
