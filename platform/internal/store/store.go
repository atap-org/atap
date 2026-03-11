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
			name, trust_level, token_hash, registry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
		e.ID, e.Type, e.URI, e.PublicKeyEd25519, e.KeyID,
		e.Name, e.TrustLevel, e.TokenHash, e.Registry, e.CreatedAt)
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
			trust_level, token_hash, registry, created_at, updated_at
		FROM entities WHERE id = $1`, id).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.TokenHash, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity: %w", err)
	}
	return e, nil
}

// GetEntityByTokenHash retrieves an entity by its bearer token hash.
func (s *Store) GetEntityByTokenHash(ctx context.Context, hash []byte) (*models.Entity, error) {
	e := &models.Entity{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, type, uri, public_key_ed25519, key_id, name,
			trust_level, token_hash, registry, created_at, updated_at
		FROM entities WHERE token_hash = $1`, hash).Scan(
		&e.ID, &e.Type, &e.URI, &e.PublicKeyEd25519, &e.KeyID, &e.Name,
		&e.TrustLevel, &e.TokenHash, &e.Registry, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get entity by token: %w", err)
	}
	return e, nil
}
