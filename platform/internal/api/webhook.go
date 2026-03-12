package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/atap-dev/atap/platform/internal/crypto"
	"github.com/atap-dev/atap/platform/internal/models"
)

// WebhookJob represents a webhook delivery job.
type WebhookJob struct {
	SignalID   string
	EntityID   string
	WebhookURL string
	Payload    []byte
	Attempt    int
}

// retryDelays defines the exponential backoff schedule for webhook retries.
var retryDelays = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	30 * time.Second,
	5 * time.Minute,
	30 * time.Minute,
}

// WebhookWorker delivers signals to webhook endpoints with retry.
type WebhookWorker struct {
	queue       chan WebhookJob
	httpClient  *http.Client
	store       WebhookStore
	signalStore SignalStore
	platformKey ed25519.PrivateKey
	log         zerolog.Logger
}

// NewWebhookWorker creates a new WebhookWorker with a buffered job queue.
func NewWebhookWorker(store WebhookStore, signalStore SignalStore, platformKey ed25519.PrivateKey, log zerolog.Logger) *WebhookWorker {
	return &WebhookWorker{
		queue: make(chan WebhookJob, 1000),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		store:       store,
		signalStore: signalStore,
		platformKey: platformKey,
		log:         log.With().Str("component", "webhook-worker").Logger(),
	}
}

// Start launches N worker goroutines that process webhook jobs.
func (w *WebhookWorker) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		go w.worker(ctx)
	}
	w.log.Info().Int("workers", workers).Msg("webhook worker started")
}

// Enqueue adds a webhook job to the queue. Drops the job if the queue is full.
func (w *WebhookWorker) Enqueue(job WebhookJob) {
	select {
	case w.queue <- job:
	default:
		w.log.Warn().
			Str("signal_id", job.SignalID).
			Str("webhook_url", job.WebhookURL).
			Msg("webhook queue full, dropping job")
	}
}

// worker reads jobs from the queue and delivers them.
func (w *WebhookWorker) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.queue:
			w.deliver(ctx, job)
		}
	}
}

// deliver attempts to deliver a webhook job.
func (w *WebhookWorker) deliver(ctx context.Context, job WebhookJob) {
	// Sign payload with platform Ed25519 key
	sig := crypto.Sign(w.platformKey, job.Payload)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Create HTTP POST request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.WebhookURL, bytes.NewReader(job.Payload))
	if err != nil {
		w.log.Error().Err(err).Str("signal_id", job.SignalID).Msg("failed to create webhook request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ATAP-Signature", sigB64)
	req.Header.Set("X-ATAP-Signal-ID", job.SignalID)

	// Execute request
	resp, err := w.httpClient.Do(req)

	attempt := &models.DeliveryAttempt{
		ID:         crypto.NewDeliveryAttemptID(),
		SignalID:   job.SignalID,
		WebhookURL: job.WebhookURL,
		Attempt:    job.Attempt,
		CreatedAt:  time.Now().UTC(),
	}

	if err != nil {
		attempt.Error = err.Error()
	} else {
		attempt.StatusCode = resp.StatusCode
		resp.Body.Close()
	}

	isSuccess := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300

	if isSuccess {
		// Delivery succeeded
		if storeErr := w.store.UpdateSignalDeliveryStatus(ctx, job.SignalID, models.DeliveryDelivered); storeErr != nil {
			w.log.Error().Err(storeErr).Str("signal_id", job.SignalID).Msg("failed to update delivery status")
		}
		w.log.Info().Str("signal_id", job.SignalID).Int("status", resp.StatusCode).Msg("webhook delivered")
	} else if job.Attempt < len(retryDelays) {
		// Schedule retry
		nextRetry := time.Now().UTC().Add(retryDelays[job.Attempt])
		attempt.NextRetryAt = &nextRetry

		w.log.Warn().
			Str("signal_id", job.SignalID).
			Int("attempt", job.Attempt).
			Time("next_retry", nextRetry).
			Msg("webhook delivery failed, scheduling retry")
	} else {
		// Max retries exhausted
		if storeErr := w.store.UpdateSignalDeliveryStatus(ctx, job.SignalID, models.DeliveryFailed); storeErr != nil {
			w.log.Error().Err(storeErr).Str("signal_id", job.SignalID).Msg("failed to update delivery status to failed")
		}
		w.log.Error().
			Str("signal_id", job.SignalID).
			Int("attempts", job.Attempt+1).
			Msg("webhook delivery permanently failed")
	}

	// Save delivery attempt
	if storeErr := w.store.SaveDeliveryAttempt(ctx, attempt); storeErr != nil {
		w.log.Error().Err(storeErr).Str("signal_id", job.SignalID).Msg("failed to save delivery attempt")
	}
}

// StartRetryPoller starts a background goroutine that polls for pending retries
// and re-enqueues them.
func (w *WebhookWorker) StartRetryPoller(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.pollRetries(ctx)
			}
		}
	}()
	w.log.Info().Dur("interval", interval).Msg("webhook retry poller started")
}

// pollRetries fetches pending retries and re-enqueues them.
func (w *WebhookWorker) pollRetries(ctx context.Context) {
	attempts, err := w.store.GetPendingRetries(ctx, time.Now().UTC())
	if err != nil {
		w.log.Error().Err(err).Msg("failed to get pending retries")
		return
	}

	for _, a := range attempts {
		sig, err := w.signalStore.GetSignal(ctx, a.SignalID)
		if err != nil {
			w.log.Warn().Err(err).Str("signal_id", a.SignalID).Msg("signal not found for retry, skipping")
			continue
		}
		if sig == nil {
			w.log.Warn().Str("signal_id", a.SignalID).Msg("signal not found for retry, skipping")
			continue
		}
		payload, err := json.Marshal(sig)
		if err != nil {
			w.log.Error().Err(err).Str("signal_id", a.SignalID).Msg("failed to marshal signal for retry")
			continue
		}
		w.Enqueue(WebhookJob{
			SignalID:   a.SignalID,
			WebhookURL: a.WebhookURL,
			Attempt:    a.Attempt + 1,
			Payload:    payload,
		})
	}

	if len(attempts) > 0 {
		w.log.Info().Int("count", len(attempts)).Msg("re-enqueued pending retries")
	}
}

// StartCleanupJob starts a background goroutine that cleans up old delivery attempts.
func (w *WebhookWorker) StartCleanupJob(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				olderThan := time.Now().UTC().Add(-24 * time.Hour)
				count, err := w.store.CleanupDeliveryAttempts(ctx, olderThan)
				if err != nil {
					w.log.Error().Err(err).Msg("failed to cleanup delivery attempts")
					continue
				}
				if count > 0 {
					w.log.Info().Int64("deleted", count).Msg("cleaned up old delivery attempts")
				}
			}
		}
	}()
	w.log.Info().Dur("interval", interval).Msg("webhook cleanup job started")
}

// formatWebhookResponse is a helper for JSON response from SetWebhook.
type webhookResponse struct {
	WebhookURL string    `json:"webhook_url"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// channelListResponse wraps a slice of channels for list responses.
type channelListResponse struct {
	Channels []*models.Channel `json:"channels"`
}

// channelInboundFromPayload wraps an external payload into an ATAP signal.
func channelInboundFromPayload(ch *models.Channel, payload json.RawMessage) *models.Signal {
	now := time.Now().UTC()

	var origin, source string
	var trustLevel int

	if ch.Type == models.ChannelTypeTrusted {
		origin = fmt.Sprintf("agent://%s", ch.TrusteeID)
		source = models.SignalSourceAgent
		trustLevel = 0 // signer trust level would come from entity lookup, default 0
	} else {
		origin = fmt.Sprintf("external://%s", ch.ID)
		source = models.SignalSourceExternal
		trustLevel = 0
	}

	return &models.Signal{
		ID:      crypto.NewSignalID(),
		Version: "1",
		TS:      now,
		Route: models.SignalRoute{
			Origin:  origin,
			Target:  fmt.Sprintf("agent://%s", ch.EntityID),
			Channel: ch.ID,
		},
		Trust: models.SignalTrust{
			Level: trustLevel,
		},
		Signal: models.SignalBody{
			Type: "channel.inbound",
			Data: payload,
		},
		Context: models.SignalContext{
			Source:   source,
			Priority: models.PriorityNormal,
		},
		TargetEntityID: ch.EntityID,
		DeliveryStatus: models.DeliveryPending,
		CreatedAt:      now,
	}
}
