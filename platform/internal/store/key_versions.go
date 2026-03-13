package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/atap-dev/atap/platform/internal/models"
)

// CreateKeyVersion inserts a new key version for an entity.
func (s *Store) CreateKeyVersion(ctx context.Context, kv *models.KeyVersion) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO key_versions (id, entity_id, public_key, key_index, valid_from, valid_until, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		kv.ID, kv.EntityID, kv.PublicKey, kv.KeyIndex, kv.ValidFrom, kv.ValidUntil, kv.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert key version: %w", err)
	}
	return nil
}

// GetActiveKeyVersion returns the current active key version for an entity
// (the key where valid_until IS NULL).
func (s *Store) GetActiveKeyVersion(ctx context.Context, entityID string) (*models.KeyVersion, error) {
	kv := &models.KeyVersion{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, entity_id, public_key, key_index, valid_from, valid_until, created_at
		FROM key_versions
		WHERE entity_id = $1 AND valid_until IS NULL
		ORDER BY key_index DESC
		LIMIT 1`, entityID).Scan(
		&kv.ID, &kv.EntityID, &kv.PublicKey, &kv.KeyIndex,
		&kv.ValidFrom, &kv.ValidUntil, &kv.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active key version: %w", err)
	}
	return kv, nil
}

// GetKeyVersions returns all key versions for an entity ordered by key_index ascending.
func (s *Store) GetKeyVersions(ctx context.Context, entityID string) ([]models.KeyVersion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entity_id, public_key, key_index, valid_from, valid_until, created_at
		FROM key_versions
		WHERE entity_id = $1
		ORDER BY key_index ASC`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query key versions: %w", err)
	}
	defer rows.Close()

	var versions []models.KeyVersion
	for rows.Next() {
		var kv models.KeyVersion
		if err := rows.Scan(&kv.ID, &kv.EntityID, &kv.PublicKey, &kv.KeyIndex,
			&kv.ValidFrom, &kv.ValidUntil, &kv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan key version: %w", err)
		}
		versions = append(versions, kv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key versions: %w", err)
	}
	return versions, nil
}

// RotateKey atomically expires the current active key and inserts a new key version.
// It also updates entities.public_key_ed25519 to keep it in sync.
func (s *Store) RotateKey(ctx context.Context, entityID string, newPubKey []byte) (*models.KeyVersion, error) {
	var newKV models.KeyVersion

	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// 1. Set valid_until on current active key
		now := time.Now().UTC()
		_, err := tx.Exec(ctx, `
			UPDATE key_versions
			SET valid_until = $1
			WHERE entity_id = $2 AND valid_until IS NULL`, now, entityID)
		if err != nil {
			return fmt.Errorf("expire active key: %w", err)
		}

		// 2. Get the max key_index for this entity
		var maxIndex int
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(key_index), 0)
			FROM key_versions
			WHERE entity_id = $1`, entityID).Scan(&maxIndex)
		if err != nil {
			return fmt.Errorf("get max key_index: %w", err)
		}

		// 3. Insert new key version
		newKV = models.KeyVersion{
			ID:        newKeyVersionID(),
			EntityID:  entityID,
			PublicKey: newPubKey,
			KeyIndex:  maxIndex + 1,
			ValidFrom: now,
			CreatedAt: now,
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO key_versions (id, entity_id, public_key, key_index, valid_from, valid_until, created_at)
			VALUES ($1, $2, $3, $4, $5, NULL, $6)`,
			newKV.ID, newKV.EntityID, newKV.PublicKey, newKV.KeyIndex, newKV.ValidFrom, newKV.CreatedAt)
		if err != nil {
			return fmt.Errorf("insert new key version: %w", err)
		}

		// 4. Update entity's current public key
		_, err = tx.Exec(ctx, `
			UPDATE entities
			SET public_key_ed25519 = $1, updated_at = $2
			WHERE id = $3`, newPubKey, now, entityID)
		if err != nil {
			return fmt.Errorf("update entity public key: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rotate key: %w", err)
	}

	return &newKV, nil
}

// newKeyVersionID generates a new unique key version ID.
func newKeyVersionID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("kv_%x", b)
}
