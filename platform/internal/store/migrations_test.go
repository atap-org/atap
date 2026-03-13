package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrations verifies that migrations 008 and 009 produce the expected schema.
// Requires DATABASE_URL env var pointing to a test database with migrations applied.
// Skip if DATABASE_URL is not set.
func TestMigrations(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping migration tests")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	t.Run("migration_008_dropped_tables", func(t *testing.T) {
		droppedTables := []string{
			"signals",
			"channels",
			"webhook_delivery",
			"claims",
			"delegations",
			"push_tokens",
		}

		for _, table := range droppedTables {
			var exists bool
			err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM information_schema.tables
					WHERE table_schema = 'public' AND table_name = $1
				)`, table).Scan(&exists)
			if err != nil {
				t.Errorf("query information_schema for table %q: %v", table, err)
				continue
			}
			if exists {
				t.Errorf("table %q should have been dropped by migration 008, but it still exists", table)
			}
		}
	})

	t.Run("migration_009_entity_columns", func(t *testing.T) {
		expectedColumns := []struct {
			name     string
			dataType string
		}{
			{"did", "text"},
			{"principal_did", "text"},
			{"client_secret_hash", "text"},
		}

		for _, col := range expectedColumns {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'entities'
				  AND column_name = $1`, col.name).Scan(&dataType)
			if err != nil {
				t.Errorf("column entities.%s not found: %v", col.name, err)
				continue
			}
			if dataType != col.dataType {
				t.Errorf("column entities.%s: got type %q, want %q", col.name, dataType, col.dataType)
			}
		}
	})

	t.Run("migration_009_key_versions_table", func(t *testing.T) {
		expectedColumns := []struct {
			name     string
			dataType string
		}{
			{"id", "text"},
			{"entity_id", "text"},
			{"public_key", "bytea"},
			{"key_index", "integer"},
			{"valid_from", "timestamp with time zone"},
			{"valid_until", "timestamp with time zone"},
			{"created_at", "timestamp with time zone"},
		}

		for _, col := range expectedColumns {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'key_versions'
				  AND column_name = $1`, col.name).Scan(&dataType)
			if err != nil {
				t.Errorf("column key_versions.%s not found: %v", col.name, err)
				continue
			}
			if dataType != col.dataType {
				t.Errorf("column key_versions.%s: got type %q, want %q", col.name, dataType, col.dataType)
			}
		}
	})

	t.Run("migration_009_oauth_auth_codes_table", func(t *testing.T) {
		expectedColumns := []struct {
			name     string
			dataType string
		}{
			{"code", "text"},
			{"entity_id", "text"},
			{"redirect_uri", "text"},
			{"scope", "ARRAY"},
			{"code_challenge", "text"},
			{"dpop_jkt", "text"},
			{"expires_at", "timestamp with time zone"},
			{"used_at", "timestamp with time zone"},
			{"created_at", "timestamp with time zone"},
		}

		for _, col := range expectedColumns {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'oauth_auth_codes'
				  AND column_name = $1`, col.name).Scan(&dataType)
			if err != nil {
				t.Errorf("column oauth_auth_codes.%s not found: %v", col.name, err)
				continue
			}
			if dataType != col.dataType {
				t.Errorf("column oauth_auth_codes.%s: got type %q, want %q", col.name, dataType, col.dataType)
			}
		}
	})

	t.Run("migration_009_oauth_tokens_table", func(t *testing.T) {
		expectedColumns := []struct {
			name     string
			dataType string
		}{
			{"id", "text"},
			{"entity_id", "text"},
			{"token_type", "text"},
			{"scope", "ARRAY"},
			{"dpop_jkt", "text"},
			{"expires_at", "timestamp with time zone"},
			{"revoked_at", "timestamp with time zone"},
			{"created_at", "timestamp with time zone"},
		}

		for _, col := range expectedColumns {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'oauth_tokens'
				  AND column_name = $1`, col.name).Scan(&dataType)
			if err != nil {
				t.Errorf("column oauth_tokens.%s not found: %v", col.name, err)
				continue
			}
			if dataType != col.dataType {
				t.Errorf("column oauth_tokens.%s: got type %q, want %q", col.name, dataType, col.dataType)
			}
		}
	})
}
