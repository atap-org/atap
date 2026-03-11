CREATE TABLE push_tokens (
    entity_id   TEXT PRIMARY KEY REFERENCES entities(id) ON DELETE CASCADE,
    token       TEXT NOT NULL,
    platform    TEXT NOT NULL CHECK (platform IN ('android', 'ios')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_push_tokens_token ON push_tokens (token);
