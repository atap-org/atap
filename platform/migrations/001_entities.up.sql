-- Migration 001: Entities table
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE entities (
    id                  TEXT PRIMARY KEY,
    type                TEXT NOT NULL CHECK (type IN ('agent', 'machine', 'human', 'org')),
    uri                 TEXT UNIQUE NOT NULL,

    -- Cryptographic identity
    public_key_ed25519  BYTEA NOT NULL,
    key_id              TEXT NOT NULL,

    -- Metadata
    name                TEXT,
    trust_level         INTEGER NOT NULL DEFAULT 0 CHECK (trust_level BETWEEN 0 AND 3),

    -- Auth
    token_hash          BYTEA NOT NULL,

    -- Registry
    registry            TEXT NOT NULL DEFAULT 'atap.app',

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_token ON entities(token_hash);
