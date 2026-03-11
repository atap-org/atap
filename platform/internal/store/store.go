package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atap-dev/atap/platform/internal/models"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// ============================================================
// ENTITIES
// ============================================================

func (s *Store) CreateEntity(ctx context.Context, e *models.Entity) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO entities (id, type, uri, public_key_ed25519, public_key_x25519, key_id,
			name, description, trust_level, delivery_pref, webhook_url,
			owner_id, org_id, attestations, token_hash, registry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $17)`,
		e.ID, e.Type, e.URI, e.PublicKeyEd25519, e.PublicKeyX25519, e.KeyID,
		e.Name, e.Description, e.TrustLevel, e.DeliveryPref, e.WebhookURL,
		nilIfEmpty(e.OwnerID), nilIfEmpty(e.OrgID), e.Attestations, e.TokenHash,
		e.Registry, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert entity: %w", err)
	}
	return nil
}

func (s *Store) GetEntity(ctx context.Context, id string) (*models.Entity, error) {
	e := &models.Entity{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, uri, public_key_ed25519, key_id, name, description,
			trust_level, delivery_pref, webhook_url, push_token, push_platform,
			owner_id, org_id, attestations, registry, created_at, updated_at
		FROM entities WHERE id = $1 AND deleted_at IS NULL`, id).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name, &e.Description,
		&e.TrustLevel, &e.DeliveryPref, &e.WebhookURL, &e.PushToken, &e.PushPlatform,
		&e.OwnerID, &e.OrgID, &e.Attestations, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	return e, nil
}

func (s *Store) GetEntityByTokenHash(ctx context.Context, hash []byte) (*models.Entity, error) {
	e := &models.Entity{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, uri, public_key_ed25519, key_id, name, description,
			trust_level, delivery_pref, webhook_url, push_token, push_platform,
			owner_id, org_id, attestations, registry, created_at, updated_at
		FROM entities WHERE token_hash = $1 AND deleted_at IS NULL`, hash).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name, &e.Description,
		&e.TrustLevel, &e.DeliveryPref, &e.WebhookURL, &e.PushToken, &e.PushPlatform,
		&e.OwnerID, &e.OrgID, &e.Attestations, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity by token: %w", err)
	}
	return e, nil
}

func (s *Store) UpdatePushToken(ctx context.Context, entityID, token, platform string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE entities SET push_token = $2, push_platform = $3, updated_at = NOW()
		WHERE id = $1`, entityID, token, platform)
	return err
}

// ============================================================
// SIGNALS
// ============================================================

func (s *Store) SaveSignal(ctx context.Context, sig *models.Signal) error {
	data, _ := json.Marshal(sig.Signal.Data)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO signals (id, version, origin, target, reply_to, channel_id,
			thread_id, ref_id, content_type, data, source_type, idempotency_key,
			tags, ttl, priority, encrypted, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		sig.ID, sig.Version, sig.Route.Origin, sig.Route.Target,
		nilIfEmpty(sig.Route.ReplyTo), nilIfEmpty(sig.Route.Channel),
		nilIfEmpty(sig.Route.Thread), nilIfEmpty(sig.Route.Ref),
		sig.Signal.Type, data,
		contextSource(sig.Context), contextIdempotency(sig.Context),
		contextTags(sig.Context), contextTTL(sig.Context),
		contextPriority(sig.Context), sig.Signal.Encrypted,
		sig.CreatedAt, sig.ExpiresAt)
	if err != nil {
		return fmt.Errorf("save signal: %w", err)
	}
	return nil
}

func (s *Store) GetSignalsForEntity(ctx context.Context, entityURI string, afterID string, limit int) ([]models.Signal, error) {
	var rows pgx.Rows
	var err error

	if afterID != "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, version, origin, target, reply_to, channel_id, thread_id, ref_id,
				content_type, data, encrypted, source_type, tags, ttl, priority, created_at
			FROM signals
			WHERE target = $1 AND id > $2
				AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY id ASC LIMIT $3`, entityURI, afterID, limit)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, version, origin, target, reply_to, channel_id, thread_id, ref_id,
				content_type, data, encrypted, source_type, tags, ttl, priority, created_at
			FROM signals
			WHERE target = $1
				AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY created_at DESC LIMIT $2`, entityURI, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query signals: %w", err)
	}
	defer rows.Close()

	var signals []models.Signal
	for rows.Next() {
		var sig models.Signal
		var data json.RawMessage
		var replyTo, channelID, threadID, refID, sourceType *string
		var tags []string
		var ttl *int

		err := rows.Scan(&sig.ID, &sig.Version, &sig.Route.Origin, &sig.Route.Target,
			&replyTo, &channelID, &threadID, &refID,
			&sig.Signal.Type, &data, &sig.Signal.Encrypted,
			&sourceType, &tags, &ttl, &sig.Context.Priority, &sig.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan signal: %w", err)
		}

		sig.Signal.Data = data
		sig.TS = sig.CreatedAt.UTC().Format(time.RFC3339)
		if replyTo != nil { sig.Route.ReplyTo = *replyTo }
		if channelID != nil { sig.Route.Channel = *channelID }
		if threadID != nil { sig.Route.Thread = *threadID }
		if refID != nil { sig.Route.Ref = *refID }
		sig.Context = &models.SignalContext{Priority: sig.Context.Priority}
		if sourceType != nil { sig.Context.Source = *sourceType }
		if tags != nil { sig.Context.Tags = tags }
		if ttl != nil { sig.Context.TTL = ttl }

		signals = append(signals, sig)
	}
	return signals, nil
}

// ============================================================
// CHANNELS
// ============================================================

func (s *Store) CreateChannel(ctx context.Context, ch *models.Channel) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO channels (id, entity_id, webhook_url, label, tags, expires_at, rate_limit, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ch.ID, ch.EntityID, ch.WebhookURL, ch.Label, ch.Tags, ch.ExpiresAt, ch.RateLimit, ch.CreatedAt)
	return err
}

func (s *Store) GetChannel(ctx context.Context, id string) (*models.Channel, error) {
	ch := &models.Channel{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, entity_id, webhook_url, label, tags, expires_at, active, signal_count, created_at
		FROM channels WHERE id = $1`, id).Scan(
		&ch.ID, &ch.EntityID, &ch.WebhookURL, &ch.Label, &ch.Tags,
		&ch.ExpiresAt, &ch.Active, &ch.SignalCount, &ch.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

func (s *Store) GetChannelByWebhookURL(ctx context.Context, url string) (*models.Channel, error) {
	ch := &models.Channel{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, entity_id, webhook_url, label, tags, expires_at, active, signal_count, created_at
		FROM channels WHERE webhook_url = $1`, url).Scan(
		&ch.ID, &ch.EntityID, &ch.WebhookURL, &ch.Label, &ch.Tags,
		&ch.ExpiresAt, &ch.Active, &ch.SignalCount, &ch.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

func (s *Store) ListChannels(ctx context.Context, entityID string) ([]models.Channel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entity_id, webhook_url, label, tags, expires_at, active, signal_count, last_signal_at, created_at
		FROM channels WHERE entity_id = $1 ORDER BY created_at DESC`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var ch models.Channel
		err := rows.Scan(&ch.ID, &ch.EntityID, &ch.WebhookURL, &ch.Label, &ch.Tags,
			&ch.ExpiresAt, &ch.Active, &ch.SignalCount, &ch.LastSignalAt, &ch.CreatedAt)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, nil
}

func (s *Store) IncrementChannelCount(ctx context.Context, channelID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE channels SET signal_count = signal_count + 1, last_signal_at = NOW()
		WHERE id = $1`, channelID)
	return err
}

func (s *Store) RevokeChannel(ctx context.Context, channelID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE channels SET active = FALSE, revoked_at = NOW() WHERE id = $1`, channelID)
	return err
}

// ============================================================
// HELPERS
// ============================================================

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func contextSource(c *models.SignalContext) *string {
	if c == nil { return nil }
	return nilIfEmpty(c.Source)
}

func contextIdempotency(c *models.SignalContext) *string {
	if c == nil { return nil }
	return nilIfEmpty(c.Idempotency)
}

func contextTags(c *models.SignalContext) []string {
	if c == nil { return nil }
	return c.Tags
}

func contextTTL(c *models.SignalContext) *int {
	if c == nil { return nil }
	return c.TTL
}

func contextPriority(c *models.SignalContext) int {
	if c == nil { return 1 }
	if c.Priority == 0 { return 1 }
	return c.Priority
}
