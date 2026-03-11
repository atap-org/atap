CREATE TABLE signals (
    id                TEXT PRIMARY KEY,           -- sig_ + ULID
    version           TEXT NOT NULL DEFAULT '1',
    ts                TIMESTAMPTZ NOT NULL,

    -- Route
    origin            TEXT NOT NULL,              -- sender entity URI
    target            TEXT NOT NULL,              -- target entity URI
    target_entity_id  TEXT NOT NULL REFERENCES entities(id),
    reply_to          TEXT,
    channel_id        TEXT,                       -- channel that received it (if via channel)
    thread_id         TEXT,
    ref_id            TEXT,

    -- Trust
    trust_level       INTEGER NOT NULL DEFAULT 0,
    signer            TEXT NOT NULL,              -- signer entity URI
    signer_key_id     TEXT NOT NULL,
    signature         TEXT NOT NULL,              -- base64 Ed25519 signature

    -- Signal body
    signal_type       TEXT NOT NULL,
    encrypted         BOOLEAN NOT NULL DEFAULT false,
    data              JSONB,                      -- signal payload (max 64KB enforced in app)

    -- Context
    source_type       TEXT NOT NULL DEFAULT 'agent' CHECK (source_type IN ('agent', 'external', 'system')),
    idempotency_key   TEXT,
    tags              JSONB DEFAULT '[]',
    ttl               INTEGER,                    -- seconds, NULL = default 7 days
    priority          TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('normal', 'high', 'urgent')),

    -- Server-side
    delivery_status   TEXT NOT NULL DEFAULT 'pending' CHECK (delivery_status IN ('pending', 'delivered', 'failed')),
    expires_at        TIMESTAMPTZ,               -- computed: ts + ttl (or ts + 7 days)
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Inbox query: by target, ordered by ID (ULID = time-ordered), excluding expired
CREATE INDEX idx_signals_inbox ON signals(target_entity_id, id) WHERE delivery_status != 'failed';

-- Cursor pagination and SSE replay: signals after a given ID for a target
CREATE INDEX idx_signals_target_after ON signals(target_entity_id, id ASC);

-- Idempotency dedup: unique within 24h window (use app-level check since partial index on NOW() is not immutable)
CREATE UNIQUE INDEX idx_signals_idempotency ON signals(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- TTL cleanup: find expired signals efficiently
CREATE INDEX idx_signals_expires ON signals(expires_at) WHERE expires_at IS NOT NULL;

-- Thread lookup
CREATE INDEX idx_signals_thread ON signals(thread_id) WHERE thread_id IS NOT NULL;
