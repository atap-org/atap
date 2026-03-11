package push

import (
	"context"

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
// This is a no-op stub -- tests should fail.
func (p *PushService) SendNotification(ctx context.Context, entityID string, sig *models.Signal) {
	// TODO: implement
}
