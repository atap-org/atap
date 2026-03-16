package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/atap-dev/atap/platform/internal/models"
)

// ============================================================
// ENCRYPTION KEYS
// ============================================================

// CreateEncKey stores a 32-byte AES-256-GCM key for the given entity.
// The key is used to encrypt and decrypt all credential ciphertext for the entity.
func (s *Store) CreateEncKey(ctx context.Context, entityID string, key []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO entity_enc_keys (entity_id, key_bytes)
		VALUES ($1, $2)
		ON CONFLICT (entity_id) DO UPDATE SET key_bytes = EXCLUDED.key_bytes`,
		entityID, key)
	if err != nil {
		return fmt.Errorf("insert enc key: %w", err)
	}
	return nil
}

// GetEncKey retrieves the AES-256-GCM key for the given entity.
// Returns nil, nil if no key exists (entity has no credentials or was crypto-shredded).
func (s *Store) GetEncKey(ctx context.Context, entityID string) ([]byte, error) {
	var key []byte
	err := s.pool.QueryRow(ctx, `
		SELECT key_bytes FROM entity_enc_keys WHERE entity_id = $1`, entityID).Scan(&key)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get enc key: %w", err)
	}
	return key, nil
}

// DeleteEncKey removes the encryption key for the given entity.
// This crypto-shreds all credentials — ciphertext rows remain but are unreadable.
func (s *Store) DeleteEncKey(ctx context.Context, entityID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM entity_enc_keys WHERE entity_id = $1`, entityID)
	if err != nil {
		return fmt.Errorf("delete enc key: %w", err)
	}
	return nil
}

// ============================================================
// CREDENTIALS
// ============================================================

// CreateCredential inserts a new credential row with encrypted VC JWT ciphertext.
func (s *Store) CreateCredential(ctx context.Context, cred *models.Credential) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO credentials (id, entity_id, type, status_index, status_list_id, credential_ct, issued_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cred.ID, cred.EntityID, cred.Type, cred.StatusIndex, cred.StatusListID, cred.CredentialCT, cred.IssuedAt)
	if err != nil {
		return fmt.Errorf("insert credential: %w", err)
	}
	return nil
}

// GetCredentials returns all credentials for the given entity (including revoked).
func (s *Store) GetCredentials(ctx context.Context, entityID string) ([]models.Credential, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entity_id, type, status_index, status_list_id, credential_ct, issued_at, revoked_at
		FROM credentials
		WHERE entity_id = $1
		ORDER BY issued_at ASC`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query credentials: %w", err)
	}
	defer rows.Close()

	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := rows.Scan(&c.ID, &c.EntityID, &c.Type, &c.StatusIndex, &c.StatusListID,
			&c.CredentialCT, &c.IssuedAt, &c.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		creds = append(creds, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("credentials rows: %w", err)
	}
	return creds, nil
}

// RevokeCredential sets revoked_at to NOW() for the given credential ID.
func (s *Store) RevokeCredential(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE credentials SET revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoke credential: %w", err)
	}
	return nil
}

// ============================================================
// CREDENTIAL STATUS LISTS
// ============================================================

// GetStatusList retrieves the Bitstring Status List by ID.
// Returns nil, nil if not found.
func (s *Store) GetStatusList(ctx context.Context, listID string) (*models.CredentialStatusList, error) {
	var sl models.CredentialStatusList
	err := s.pool.QueryRow(ctx, `
		SELECT id, bits, next_index, created_at, updated_at
		FROM credential_status_lists WHERE id = $1`, listID).Scan(
		&sl.ID, &sl.Bits, &sl.NextIndex, &sl.CreatedAt, &sl.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get status list: %w", err)
	}
	return &sl, nil
}

// UpdateStatusListBit reads the current bits, sets the bit at the given index,
// and updates the row atomically.
func (s *Store) UpdateStatusListBit(ctx context.Context, listID string, index int) error {
	// Read current bits, set the bit, write back.
	var bits []byte
	err := s.pool.QueryRow(ctx, `
		SELECT bits FROM credential_status_lists WHERE id = $1`, listID).Scan(&bits)
	if err != nil {
		return fmt.Errorf("read status list bits: %w", err)
	}

	byteIdx := index / 8
	if byteIdx >= len(bits) {
		return fmt.Errorf("bit index %d out of range (bits len %d)", index, len(bits))
	}
	bits[byteIdx] |= (1 << (7 - uint(index%8)))

	_, err = s.pool.Exec(ctx, `
		UPDATE credential_status_lists
		SET bits = $1, updated_at = NOW()
		WHERE id = $2`, bits, listID)
	if err != nil {
		return fmt.Errorf("update status list bits: %w", err)
	}
	return nil
}

// GetNextStatusIndex atomically increments the next_index counter and returns
// the value before the increment (i.e., the index to assign to the new credential).
func (s *Store) GetNextStatusIndex(ctx context.Context, listID string) (int, error) {
	var idx int
	err := s.pool.QueryRow(ctx, `
		UPDATE credential_status_lists
		SET next_index = next_index + 1
		WHERE id = $1
		RETURNING next_index - 1`, listID).Scan(&idx)
	if err != nil {
		return 0, fmt.Errorf("get next status index: %w", err)
	}
	return idx, nil
}
