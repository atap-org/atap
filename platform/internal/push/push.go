package push

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/messaging"
	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/models"
)

// FCMClient is the interface for Firebase Cloud Messaging operations.
type FCMClient interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

// PushTokenStore is the interface for looking up push tokens.
type PushTokenStore interface {
	GetPushToken(ctx context.Context, entityID string) (*models.PushToken, error)
}

// PushService handles sending push notifications via FCM.
type PushService struct {
	fcmClient FCMClient
	store     PushTokenStore
	log       zerolog.Logger
}

// NewPushService creates a new PushService.
func NewPushService(fcmClient FCMClient, store PushTokenStore, log zerolog.Logger) *PushService {
	return &PushService{
		fcmClient: fcmClient,
		store:     store,
		log:       log,
	}
}

// SendNotification sends a push notification for a signal to an entity.
// If no push token is registered, this is a no-op.
// Errors are logged but not propagated (push is best-effort / fire-and-forget).
func (p *PushService) SendNotification(ctx context.Context, entityID string, sig *models.Signal) {
	token, err := p.store.GetPushToken(ctx, entityID)
	if err != nil {
		p.log.Error().Err(err).Str("entity_id", entityID).Msg("failed to get push token")
		return
	}
	if token == nil {
		return
	}

	msg := &messaging.Message{
		Token: token.Token,
		Notification: &messaging.Notification{
			Title: sig.Signal.Type,
			Body:  fmt.Sprintf("from %s", sig.Route.Origin),
		},
		Data: map[string]string{
			"signal_id": sig.ID,
			"type":      sig.Signal.Type,
		},
	}

	if _, err := p.fcmClient.Send(ctx, msg); err != nil {
		p.log.Error().Err(err).
			Str("entity_id", entityID).
			Str("signal_id", sig.ID).
			Msg("failed to send push notification")
	}
}
