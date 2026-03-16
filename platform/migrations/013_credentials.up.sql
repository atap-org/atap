-- 013_credentials.up.sql
-- Per-entity encryption keys for crypto-shredding (PRV-01, PRV-02)
CREATE TABLE entity_enc_keys (
    entity_id  TEXT PRIMARY KEY REFERENCES entities(id) ON DELETE CASCADE,
    key_bytes  BYTEA NOT NULL,  -- 32-byte AES-256 key
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Verifiable Credentials (CRD-01 through CRD-06)
CREATE TABLE credentials (
    id             TEXT PRIMARY KEY,            -- "crd_" + ULID
    entity_id      TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    type           TEXT NOT NULL,               -- ATAPEmailVerification etc.
    status_index   INT NOT NULL,               -- index in status list
    status_list_id TEXT NOT NULL,
    credential_ct  BYTEA NOT NULL,             -- AES-256-GCM encrypted VC JWT
    issued_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at     TIMESTAMPTZ
);

CREATE INDEX idx_credentials_entity ON credentials(entity_id);

-- Bitstring Status Lists (CRD-05)
CREATE TABLE credential_status_lists (
    id         TEXT PRIMARY KEY,              -- sequential string ID
    bits       BYTEA NOT NULL,               -- raw bitstring (16 KB = 131072 slots)
    next_index INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed one status list with 16384 zero bytes (131072 credential slots)
INSERT INTO credential_status_lists (id, bits) VALUES ('1', decode(repeat('00', 16384), 'hex'));
