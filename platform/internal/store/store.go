package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atap-dev/atap/platform/internal/models"
)

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
			name, trust_level, registry, did, principal_did, client_secret_hash,
			x25519_public_key, x25519_private_key,
			created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)`,
		e.ID, e.Type, buildLegacyURI(e), e.PublicKeyEd25519, e.KeyID,
		e.Name, e.TrustLevel, e.Registry, e.DID, e.PrincipalDID, nullableString(e.ClientSecretHash),
		nullableBytes(e.X25519PublicKey), nullableBytes(e.X25519PrivateKey),
		e.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert entity: %w", err)
	}
	return nil
}

// GetEntity retrieves an entity by ID.
func (s *Store) GetEntity(ctx context.Context, id string) (*models.Entity, error) {
	e := &models.Entity{}
	var x25519Pub, x25519Priv []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, public_key_ed25519, key_id, name,
			trust_level, registry, COALESCE(did, ''), COALESCE(principal_did, ''),
			COALESCE(client_secret_hash, ''), x25519_public_key, x25519_private_key,
			created_at, updated_at
		FROM entities WHERE id = $1`, id).Scan(
		&e.ID, &e.Type, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.Registry, &e.DID, &e.PrincipalDID,
		&e.ClientSecretHash, &x25519Pub, &x25519Priv,
		&e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	e.X25519PublicKey = x25519Pub
	e.X25519PrivateKey = x25519Priv
	return e, nil
}

// GetEntityByKeyID retrieves an entity by its Ed25519 key ID.
func (s *Store) GetEntityByKeyID(ctx context.Context, keyID string) (*models.Entity, error) {
	e := &models.Entity{}
	var x25519Pub, x25519Priv []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, public_key_ed25519, key_id, name,
			trust_level, registry, COALESCE(did, ''), COALESCE(principal_did, ''),
			COALESCE(client_secret_hash, ''), x25519_public_key, x25519_private_key,
			created_at, updated_at
		FROM entities WHERE key_id = $1`, keyID).Scan(
		&e.ID, &e.Type, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.Registry, &e.DID, &e.PrincipalDID,
		&e.ClientSecretHash, &x25519Pub, &x25519Priv,
		&e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity by key_id: %w", err)
	}
	e.X25519PublicKey = x25519Pub
	e.X25519PrivateKey = x25519Priv
	return e, nil
}

// GetEntityByDID retrieves an entity by its DID string.
func (s *Store) GetEntityByDID(ctx context.Context, did string) (*models.Entity, error) {
	e := &models.Entity{}
	var x25519Pub, x25519Priv []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, public_key_ed25519, key_id, name,
			trust_level, registry, COALESCE(did, ''), COALESCE(principal_did, ''),
			COALESCE(client_secret_hash, ''), x25519_public_key, x25519_private_key,
			created_at, updated_at
		FROM entities WHERE did = $1`, did).Scan(
		&e.ID, &e.Type, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.Registry, &e.DID, &e.PrincipalDID,
		&e.ClientSecretHash, &x25519Pub, &x25519Priv,
		&e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity by did: %w", err)
	}
	e.X25519PublicKey = x25519Pub
	e.X25519PrivateKey = x25519Priv
	return e, nil
}

// DeleteEntity deletes an entity by ID (crypto-shred for GDPR).
func (s *Store) DeleteEntity(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM entities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete entity: %w", err)
	}
	return nil
}

// ============================================================
// RATE LIMIT CONFIG
// ============================================================

// GetRateLimitConfig reads all rate limit configuration key-value pairs.
func (s *Store) GetRateLimitConfig(ctx context.Context) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM rate_limit_config`)
	if err != nil {
		return nil, fmt.Errorf("get rate limit config: %w", err)
	}
	defer rows.Close()

	cfg := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan rate limit config row: %w", err)
		}
		cfg[k] = v
	}
	return cfg, rows.Err()
}

// ============================================================
// HELPERS
// ============================================================

// buildLegacyURI constructs the legacy URI field for the entities table.
// The URI column still exists in the DB schema so we populate it.
func buildLegacyURI(e *models.Entity) string {
	if e.DID != "" {
		return e.DID
	}
	return e.Type + "://" + e.ID
}

// nullableString converts an empty string to nil for SQL NULL.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullableBytes converts an empty/nil byte slice to nil for SQL NULL.
func nullableBytes(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
