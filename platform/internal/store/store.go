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

// ErrDuplicateSignal is returned when a signal with the same idempotency key already exists.
var ErrDuplicateSignal = fmt.Errorf("duplicate signal")

// Store provides PostgreSQL data access.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a new Store with a pgx connection pool.
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

// Close closes the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// ============================================================
// ENTITIES
// ============================================================

// CreateEntity inserts a new entity into the database.
func (s *Store) CreateEntity(ctx context.Context, e *models.Entity) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO entities (id, type, uri, public_key_ed25519, key_id,
			name, trust_level, registry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		e.ID, e.Type, e.URI, e.PublicKeyEd25519, e.KeyID,
		e.Name, e.TrustLevel, e.Registry, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert entity: %w", err)
	}
	return nil
}

// GetEntity retrieves an entity by ID.
func (s *Store) GetEntity(ctx context.Context, id string) (*models.Entity, error) {
	e := &models.Entity{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, uri, public_key_ed25519, key_id, name,
			trust_level, registry, created_at, updated_at
		FROM entities WHERE id = $1`, id).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	return e, nil
}

// GetEntityByKeyID retrieves an entity by its Ed25519 key ID.
func (s *Store) GetEntityByKeyID(ctx context.Context, keyID string) (*models.Entity, error) {
	e := &models.Entity{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, uri, public_key_ed25519, key_id, name,
			trust_level, registry, created_at, updated_at
		FROM entities WHERE key_id = $1`, keyID).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity by key_id: %w", err)
	}
	return e, nil
}

// ============================================================
// CLAIMS
// ============================================================

// ErrClaimNotAvailable is returned when a claim cannot be redeemed (not found or already redeemed).
var ErrClaimNotAvailable = fmt.Errorf("claim not available")

// scanClaim scans a row into a Claim struct.
func scanClaim(row pgx.Row) (*models.Claim, error) {
	c := &models.Claim{}
	err := row.Scan(
		&c.ID, &c.Code, &c.CreatorID, &c.RedeemedBy, &c.Status,
		&c.CreatedAt, &c.RedeemedAt, &c.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

const claimColumns = `id, code, creator_id, redeemed_by, status, created_at, redeemed_at, expires_at`

// CreateClaim inserts a new claim.
func (s *Store) CreateClaim(ctx context.Context, c *models.Claim) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO claims (id, code, creator_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.Code, c.CreatorID, c.Status, c.CreatedAt, c.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert claim: %w", err)
	}
	return nil
}

// GetClaimByCode retrieves a claim by its code. Returns nil, nil if not found.
func (s *Store) GetClaimByCode(ctx context.Context, code string) (*models.Claim, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+claimColumns+` FROM claims WHERE code = $1`, code)
	c, err := scanClaim(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get claim by code: %w", err)
	}
	return c, nil
}

// RedeemClaim marks a pending claim as redeemed by the given entity.
// Returns ErrClaimNotAvailable if the claim does not exist or is not pending.
func (s *Store) RedeemClaim(ctx context.Context, code string, redeemedBy string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE claims SET status = 'redeemed', redeemed_by = $2, redeemed_at = NOW()
		WHERE code = $1 AND status = 'pending'`, code, redeemedBy)
	if err != nil {
		return fmt.Errorf("redeem claim: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrClaimNotAvailable
	}
	return nil
}

// ============================================================
// DELEGATIONS
// ============================================================

// scanDelegation scans a row into a Delegation struct.
func scanDelegation(row pgx.Row) (*models.Delegation, error) {
	d := &models.Delegation{}
	err := row.Scan(
		&d.ID, &d.DelegatorID, &d.DelegateID, &d.Scope,
		&d.CreatedAt, &d.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

const delegationColumns = `id, delegator_id, delegate_id, scope, created_at, revoked_at`

// CreateDelegation inserts a new delegation.
func (s *Store) CreateDelegation(ctx context.Context, d *models.Delegation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO delegations (id, delegator_id, delegate_id, scope, created_at)
		VALUES ($1, $2, $3, $4, $5)`,
		d.ID, d.DelegatorID, d.DelegateID, d.Scope, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert delegation: %w", err)
	}
	return nil
}

// GetDelegationsByDelegator retrieves all active delegations where the entity is the delegator.
func (s *Store) GetDelegationsByDelegator(ctx context.Context, delegatorID string) ([]*models.Delegation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+delegationColumns+` FROM delegations
		WHERE delegator_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC`, delegatorID)
	if err != nil {
		return nil, fmt.Errorf("get delegations by delegator: %w", err)
	}
	defer rows.Close()

	var delegations []*models.Delegation
	for rows.Next() {
		d, err := scanDelegation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan delegation: %w", err)
		}
		delegations = append(delegations, d)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate delegations: %w", rows.Err())
	}
	return delegations, nil
}

// GetDelegationsByDelegate retrieves all active delegations where the entity is the delegate.
func (s *Store) GetDelegationsByDelegate(ctx context.Context, delegateID string) ([]*models.Delegation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+delegationColumns+` FROM delegations
		WHERE delegate_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC`, delegateID)
	if err != nil {
		return nil, fmt.Errorf("get delegations by delegate: %w", err)
	}
	defer rows.Close()

	var delegations []*models.Delegation
	for rows.Next() {
		d, err := scanDelegation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan delegation: %w", err)
		}
		delegations = append(delegations, d)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate delegations: %w", rows.Err())
	}
	return delegations, nil
}

// ============================================================
// PUSH TOKENS
// ============================================================

// UpsertPushToken creates or updates a push token for an entity.
func (s *Store) UpsertPushToken(ctx context.Context, entityID, token, platform string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO push_tokens (entity_id, token, platform, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (entity_id) DO UPDATE SET token = $2, platform = $3, updated_at = NOW()`,
		entityID, token, platform)
	if err != nil {
		return fmt.Errorf("upsert push token: %w", err)
	}
	return nil
}

// GetPushToken retrieves the push token for an entity. Returns nil, nil if not found.
func (s *Store) GetPushToken(ctx context.Context, entityID string) (*models.PushToken, error) {
	pt := &models.PushToken{}
	err := s.pool.QueryRow(ctx, `
		SELECT entity_id, token, platform, created_at, updated_at
		FROM push_tokens WHERE entity_id = $1`, entityID).Scan(
		&pt.EntityID, &pt.Token, &pt.Platform, &pt.CreatedAt, &pt.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get push token: %w", err)
	}
	return pt, nil
}

// DeletePushToken removes the push token for an entity.
func (s *Store) DeletePushToken(ctx context.Context, entityID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM push_tokens WHERE entity_id = $1`, entityID)
	if err != nil {
		return fmt.Errorf("delete push token: %w", err)
	}
	return nil
}

// ============================================================
// SIGNALS
// ============================================================

// scanSignal scans a row into a Signal struct.
func scanSignal(row pgx.Row) (*models.Signal, error) {
	s := &models.Signal{}
	var tagsJSON []byte
	var data []byte
	var idempotencyKey *string
	err := row.Scan(
		&s.ID, &s.Version, &s.TS,
		&s.Route.Origin, &s.Route.Target, &s.TargetEntityID,
		&s.Route.ReplyTo, &s.Route.Channel, &s.Route.Thread, &s.Route.Ref,
		&s.Trust.Level, &s.Trust.Signer, &s.Trust.SignerKeyID, &s.Trust.Signature,
		&s.Signal.Type, &s.Signal.Encrypted, &data,
		&s.Context.Source, &idempotencyKey, &tagsJSON, &s.Context.TTL, &s.Context.Priority,
		&s.DeliveryStatus, &s.ExpiresAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if idempotencyKey != nil {
		s.Context.Idempotency = *idempotencyKey
	}
	if data != nil {
		s.Signal.Data = json.RawMessage(data)
	}
	if tagsJSON != nil {
		_ = json.Unmarshal(tagsJSON, &s.Context.Tags)
	}
	return s, nil
}

// signalColumns is the column list for signal queries.
const signalColumns = `id, version, ts,
	origin, target, target_entity_id, reply_to, channel_id, thread_id, ref_id,
	trust_level, signer, signer_key_id, signature,
	signal_type, encrypted, data,
	source_type, idempotency_key, tags, ttl, priority,
	delivery_status, expires_at, created_at`

// SaveSignal inserts a new signal. Computes expires_at from TS + TTL.
// Returns ErrDuplicateSignal if idempotency_key conflicts.
func (s *Store) SaveSignal(ctx context.Context, sig *models.Signal) error {
	// Compute expires_at
	ttl := models.DefaultSignalTTL
	if sig.Context.TTL > 0 {
		ttl = time.Duration(sig.Context.TTL) * time.Second
	}
	expiresAt := sig.TS.Add(ttl)
	sig.ExpiresAt = &expiresAt

	// Marshal tags to JSON
	tagsJSON, err := json.Marshal(sig.Context.Tags)
	if err != nil {
		return fmt.Errorf("marshal signal tags: %w", err)
	}

	// Marshal data
	var data []byte
	if sig.Signal.Data != nil {
		data = []byte(sig.Signal.Data)
	}

	// Treat empty idempotency key as NULL to avoid spurious unique conflicts
	var idempotencyKey interface{}
	if sig.Context.Idempotency != "" {
		idempotencyKey = sig.Context.Idempotency
	}

	ct, err := s.pool.Exec(ctx, `
		INSERT INTO signals (id, version, ts,
			origin, target, target_entity_id, reply_to, channel_id, thread_id, ref_id,
			trust_level, signer, signer_key_id, signature,
			signal_type, encrypted, data,
			source_type, idempotency_key, tags, ttl, priority,
			delivery_status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`,
		sig.ID, sig.Version, sig.TS,
		sig.Route.Origin, sig.Route.Target, sig.TargetEntityID,
		sig.Route.ReplyTo, sig.Route.Channel, sig.Route.Thread, sig.Route.Ref,
		sig.Trust.Level, sig.Trust.Signer, sig.Trust.SignerKeyID, sig.Trust.Signature,
		sig.Signal.Type, sig.Signal.Encrypted, data,
		sig.Context.Source, idempotencyKey, tagsJSON, sig.Context.TTL, sig.Context.Priority,
		sig.DeliveryStatus, sig.ExpiresAt, sig.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert signal: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrDuplicateSignal
	}
	return nil
}

// GetSignal retrieves a signal by ID. Returns nil if not found.
func (s *Store) GetSignal(ctx context.Context, id string) (*models.Signal, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+signalColumns+` FROM signals WHERE id = $1`, id)
	sig, err := scanSignal(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get signal: %w", err)
	}
	return sig, nil
}

// GetInbox retrieves signals for an entity's inbox with cursor pagination.
// Returns signals, hasMore flag, and error.
func (s *Store) GetInbox(ctx context.Context, entityID, after string, limit int) ([]*models.Signal, bool, error) {
	var rows pgx.Rows
	var err error

	if after != "" {
		rows, err = s.pool.Query(ctx, `
			SELECT `+signalColumns+` FROM signals
			WHERE target_entity_id = $1
				AND (expires_at IS NULL OR expires_at > NOW())
				AND id > $2
			ORDER BY id ASC
			LIMIT $3`,
			entityID, after, limit+1)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT `+signalColumns+` FROM signals
			WHERE target_entity_id = $1
				AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY id ASC
			LIMIT $2`,
			entityID, limit+1)
	}
	if err != nil {
		return nil, false, fmt.Errorf("get inbox: %w", err)
	}
	defer rows.Close()

	var signals []*models.Signal
	for rows.Next() {
		sig, err := scanSignal(rows)
		if err != nil {
			return nil, false, fmt.Errorf("scan inbox signal: %w", err)
		}
		signals = append(signals, sig)
	}
	if rows.Err() != nil {
		return nil, false, fmt.Errorf("iterate inbox: %w", rows.Err())
	}

	hasMore := len(signals) > limit
	if hasMore {
		signals = signals[:limit]
	}
	return signals, hasMore, nil
}

// GetSignalsAfter retrieves signals after a given ID for SSE replay.
// Capped at 1000 to prevent memory issues.
func (s *Store) GetSignalsAfter(ctx context.Context, entityID, afterID string) ([]*models.Signal, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+signalColumns+` FROM signals
		WHERE target_entity_id = $1
			AND (expires_at IS NULL OR expires_at > NOW())
			AND id > $2
		ORDER BY id ASC
		LIMIT 1000`,
		entityID, afterID)
	if err != nil {
		return nil, fmt.Errorf("get signals after: %w", err)
	}
	defer rows.Close()

	var signals []*models.Signal
	for rows.Next() {
		sig, err := scanSignal(rows)
		if err != nil {
			return nil, fmt.Errorf("scan signal after: %w", err)
		}
		signals = append(signals, sig)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate signals after: %w", rows.Err())
	}
	return signals, nil
}

// UpdateSignalDeliveryStatus updates the delivery status of a signal.
func (s *Store) UpdateSignalDeliveryStatus(ctx context.Context, signalID, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE signals SET delivery_status = $2 WHERE id = $1`, signalID, status)
	if err != nil {
		return fmt.Errorf("update signal delivery status: %w", err)
	}
	return nil
}

// ============================================================
// CHANNELS
// ============================================================

// CreateChannel inserts a new channel.
func (s *Store) CreateChannel(ctx context.Context, ch *models.Channel) error {
	tagsJSON, err := json.Marshal(ch.Tags)
	if err != nil {
		return fmt.Errorf("marshal channel tags: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO channels (id, entity_id, label, tags, type, trustee_id, basic_auth_hash,
			active, signal_count, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		ch.ID, ch.EntityID, ch.Label, tagsJSON, ch.Type, ch.TrusteeID, ch.BasicAuthHash,
		ch.Active, ch.SignalCount, ch.CreatedAt, ch.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}
	return nil
}

// scanChannel scans a row into a Channel struct.
func scanChannel(row pgx.Row) (*models.Channel, error) {
	ch := &models.Channel{}
	var tagsJSON []byte
	err := row.Scan(
		&ch.ID, &ch.EntityID, &ch.Label, &tagsJSON, &ch.Type, &ch.TrusteeID,
		&ch.BasicAuthHash, &ch.Active, &ch.SignalCount, &ch.CreatedAt, &ch.ExpiresAt, &ch.RevokedAt,
	)
	if err != nil {
		return nil, err
	}
	if tagsJSON != nil {
		_ = json.Unmarshal(tagsJSON, &ch.Tags)
	}
	return ch, nil
}

const channelColumns = `id, entity_id, label, tags, type, trustee_id, basic_auth_hash,
	active, signal_count, created_at, expires_at, revoked_at`

// GetChannel retrieves a channel by ID. Returns nil if not found.
func (s *Store) GetChannel(ctx context.Context, id string) (*models.Channel, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+channelColumns+` FROM channels WHERE id = $1`, id)
	ch, err := scanChannel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return ch, nil
}

// GetChannelByID retrieves an active channel by ID. Returns nil if not found or not active.
func (s *Store) GetChannelByID(ctx context.Context, id string) (*models.Channel, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+channelColumns+` FROM channels WHERE id = $1 AND active = true`, id)
	ch, err := scanChannel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel by id: %w", err)
	}
	return ch, nil
}

// ListChannels retrieves all active channels for an entity.
func (s *Store) ListChannels(ctx context.Context, entityID string) ([]*models.Channel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+channelColumns+` FROM channels
		WHERE entity_id = $1 AND active = true
		ORDER BY created_at DESC`, entityID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []*models.Channel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, ch)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate channels: %w", rows.Err())
	}
	return channels, nil
}

// RevokeChannel marks a channel as inactive.
func (s *Store) RevokeChannel(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE channels SET active = false, revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoke channel: %w", err)
	}
	return nil
}

// IncrementChannelSignalCount increments the signal counter for a channel.
func (s *Store) IncrementChannelSignalCount(ctx context.Context, channelID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE channels SET signal_count = signal_count + 1 WHERE id = $1`, channelID)
	if err != nil {
		return fmt.Errorf("increment channel signal count: %w", err)
	}
	return nil
}

// ============================================================
// WEBHOOK CONFIGS
// ============================================================

// GetWebhookConfig retrieves the webhook config for an entity. Returns nil if not found.
func (s *Store) GetWebhookConfig(ctx context.Context, entityID string) (*models.WebhookConfig, error) {
	w := &models.WebhookConfig{}
	err := s.pool.QueryRow(ctx, `
		SELECT entity_id, url, created_at, updated_at
		FROM webhook_configs WHERE entity_id = $1`, entityID).Scan(
		&w.EntityID, &w.URL, &w.CreatedAt, &w.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook config: %w", err)
	}
	return w, nil
}

// SetWebhookConfig creates or updates the webhook config for an entity.
func (s *Store) SetWebhookConfig(ctx context.Context, entityID, url string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_configs (entity_id, url, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (entity_id) DO UPDATE SET url = $2, updated_at = NOW()`, entityID, url)
	if err != nil {
		return fmt.Errorf("set webhook config: %w", err)
	}
	return nil
}

// DeleteWebhookConfig removes the webhook config for an entity.
func (s *Store) DeleteWebhookConfig(ctx context.Context, entityID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM webhook_configs WHERE entity_id = $1`, entityID)
	if err != nil {
		return fmt.Errorf("delete webhook config: %w", err)
	}
	return nil
}

// ============================================================
// DELIVERY ATTEMPTS
// ============================================================

// SaveDeliveryAttempt inserts a delivery attempt record.
func (s *Store) SaveDeliveryAttempt(ctx context.Context, a *models.DeliveryAttempt) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO delivery_attempts (id, signal_id, webhook_url, attempt, status_code, error, next_retry_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.SignalID, a.WebhookURL, a.Attempt, a.StatusCode, a.Error, a.NextRetryAt, a.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert delivery attempt: %w", err)
	}
	return nil
}

// GetPendingRetries retrieves delivery attempts due for retry.
func (s *Store) GetPendingRetries(ctx context.Context, now time.Time) ([]*models.DeliveryAttempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, signal_id, webhook_url, attempt, status_code, error, next_retry_at, created_at
		FROM delivery_attempts
		WHERE next_retry_at IS NOT NULL AND next_retry_at <= $1
		LIMIT 100`, now)
	if err != nil {
		return nil, fmt.Errorf("get pending retries: %w", err)
	}
	defer rows.Close()

	var attempts []*models.DeliveryAttempt
	for rows.Next() {
		a := &models.DeliveryAttempt{}
		err := rows.Scan(&a.ID, &a.SignalID, &a.WebhookURL, &a.Attempt, &a.StatusCode, &a.Error, &a.NextRetryAt, &a.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, a)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate delivery attempts: %w", rows.Err())
	}
	return attempts, nil
}

// CleanupDeliveryAttempts removes delivery attempts older than the given time.
// Returns the number of rows deleted.
func (s *Store) CleanupDeliveryAttempts(ctx context.Context, olderThan time.Time) (int64, error) {
	ct, err := s.pool.Exec(ctx, `DELETE FROM delivery_attempts WHERE created_at < $1`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("cleanup delivery attempts: %w", err)
	}
	return ct.RowsAffected(), nil
}
