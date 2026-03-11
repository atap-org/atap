CREATE TABLE channels (
    id              TEXT PRIMARY KEY,           -- chn_ + 32 hex
    entity_id       TEXT NOT NULL REFERENCES entities(id),
    label           TEXT,
    tags            JSONB DEFAULT '[]',
    type            TEXT NOT NULL CHECK (type IN ('trusted', 'open')),
    trustee_id      TEXT,                      -- for trusted channels: entity ID of trustee
    basic_auth_hash BYTEA,                     -- for open channels: bcrypt hash of password
    active          BOOLEAN NOT NULL DEFAULT true,
    signal_count    BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_channels_entity ON channels(entity_id) WHERE active = true;
CREATE INDEX idx_channels_trustee ON channels(trustee_id) WHERE trustee_id IS NOT NULL AND active = true;

CREATE TABLE webhook_configs (
    entity_id   TEXT PRIMARY KEY REFERENCES entities(id),
    url         TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
