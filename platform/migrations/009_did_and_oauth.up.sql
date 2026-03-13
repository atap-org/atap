-- Migration 009: DID and OAuth 2.1 schema
-- Extends entities with DID fields and creates tables for key rotation and OAuth 2.1 flows.

-- Extend entities with DID-based identity fields
ALTER TABLE entities ADD COLUMN did TEXT UNIQUE;
ALTER TABLE entities ADD COLUMN principal_did TEXT;
ALTER TABLE entities ADD COLUMN client_secret_hash TEXT;

-- Key versions for DID key rotation (DID-07)
CREATE TABLE key_versions (
    id          TEXT PRIMARY KEY,
    entity_id   TEXT NOT NULL REFERENCES entities(id),
    public_key  BYTEA NOT NULL,
    key_index   INTEGER NOT NULL DEFAULT 1,
    valid_from  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth 2.1 authorization codes (Authorization Code + PKCE flow)
CREATE TABLE oauth_auth_codes (
    code            TEXT PRIMARY KEY,
    entity_id       TEXT NOT NULL REFERENCES entities(id),
    redirect_uri    TEXT NOT NULL,
    scope           TEXT[] NOT NULL,
    code_challenge  TEXT NOT NULL,
    dpop_jkt        TEXT NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth 2.1 access and refresh tokens
CREATE TABLE oauth_tokens (
    id          TEXT PRIMARY KEY,
    entity_id   TEXT NOT NULL REFERENCES entities(id),
    token_type  TEXT NOT NULL DEFAULT 'access',
    scope       TEXT[] NOT NULL,
    dpop_jkt    TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_tokens_entity ON oauth_tokens(entity_id, token_type);
CREATE INDEX idx_oauth_tokens_expires ON oauth_tokens(expires_at);
