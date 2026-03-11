-- Migration 001: Core tables
-- Run with: psql $DATABASE_URL -f 001_init.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- ENTITIES
-- ============================================================

CREATE TABLE entities (
    id                  TEXT PRIMARY KEY,
    type                TEXT NOT NULL CHECK (type IN ('agent', 'machine', 'human', 'org')),
    uri                 TEXT UNIQUE NOT NULL,

    -- Cryptographic identity
    public_key_ed25519  BYTEA NOT NULL,
    public_key_x25519   BYTEA,
    key_id              TEXT NOT NULL,

    -- Metadata
    name                TEXT,
    description         TEXT,
    trust_level         INTEGER NOT NULL DEFAULT 0 CHECK (trust_level BETWEEN 0 AND 3),

    -- Delivery
    delivery_pref       TEXT NOT NULL DEFAULT 'sse' CHECK (delivery_pref IN ('sse', 'webhook', 'poll')),
    webhook_url         TEXT,
    push_token          TEXT,
    push_platform       TEXT CHECK (push_platform IN ('fcm', 'apns', NULL)),

    -- Ownership
    owner_id            TEXT REFERENCES entities(id),
    org_id              TEXT REFERENCES entities(id),

    -- Attestations (for humans — JSONB, NOT the identity)
    attestations        JSONB NOT NULL DEFAULT '{}',

    -- Key recovery (for humans)
    recovery_backup     JSONB,

    -- Auth
    token_hash          BYTEA NOT NULL,

    -- Registry
    registry            TEXT NOT NULL DEFAULT 'atap.app',
    revocation_url      TEXT,

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ
);

CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_owner ON entities(owner_id);
CREATE INDEX idx_entities_org ON entities(org_id);
CREATE INDEX idx_entities_token ON entities(token_hash);

-- ============================================================
-- SIGNALS
-- ============================================================

CREATE TABLE signals (
    id                  TEXT PRIMARY KEY,
    version             TEXT NOT NULL DEFAULT '1',

    -- Route
    origin              TEXT NOT NULL,
    target              TEXT NOT NULL,
    reply_to            TEXT,
    channel_id          TEXT,
    thread_id           TEXT,
    ref_id              TEXT,

    -- Trust
    signature           BYTEA,
    signer_key_id       TEXT,
    delegation_id       TEXT,
    encrypted           BOOLEAN NOT NULL DEFAULT FALSE,

    -- Signal body
    content_type        TEXT NOT NULL DEFAULT 'application/json',
    data                JSONB,
    data_encrypted      BYTEA,

    -- Context
    source_type         TEXT,
    idempotency_key     TEXT,
    tags                TEXT[],
    ttl                 INTEGER,
    priority            INTEGER NOT NULL DEFAULT 1,

    -- Delivery tracking
    delivered           BOOLEAN NOT NULL DEFAULT FALSE,
    delivered_at        TIMESTAMPTZ,
    delivery_method     TEXT,

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ
);

CREATE INDEX idx_signals_target ON signals(target);
CREATE INDEX idx_signals_target_created ON signals(target, created_at DESC);
CREATE INDEX idx_signals_thread ON signals(thread_id) WHERE thread_id IS NOT NULL;
CREATE INDEX idx_signals_expires ON signals(expires_at) WHERE expires_at IS NOT NULL;
CREATE UNIQUE INDEX idx_signals_idempotency ON signals(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- ============================================================
-- CHANNELS
-- ============================================================

CREATE TABLE channels (
    id                  TEXT PRIMARY KEY,
    entity_id           TEXT NOT NULL REFERENCES entities(id),
    webhook_url         TEXT UNIQUE NOT NULL,

    label               TEXT,
    tags                TEXT[],

    expires_at          TIMESTAMPTZ,
    rate_limit          INTEGER,

    active              BOOLEAN NOT NULL DEFAULT TRUE,
    revoked_at          TIMESTAMPTZ,

    signal_count        BIGINT NOT NULL DEFAULT 0,
    last_signal_at      TIMESTAMPTZ,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_entity ON channels(entity_id);

-- ============================================================
-- DELEGATIONS
-- ============================================================

CREATE TABLE delegations (
    id                  TEXT PRIMARY KEY,
    version             TEXT NOT NULL DEFAULT '1',

    principal_id        TEXT NOT NULL,
    delegate_id         TEXT NOT NULL,
    via_ids             TEXT[],

    actions             TEXT[] NOT NULL,
    spend_limit         JSONB,
    data_classes        TEXT[],
    expires_at          TIMESTAMPTZ NOT NULL,

    constraints         JSONB,
    human_verification  JSONB,
    signatures          JSONB NOT NULL,

    status              TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired')),
    revoked_at          TIMESTAMPTZ,
    revoked_by          TEXT,
    revoke_reason       TEXT,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delegations_principal ON delegations(principal_id);
CREATE INDEX idx_delegations_delegate ON delegations(delegate_id);
CREATE INDEX idx_delegations_status ON delegations(status);

-- ============================================================
-- CLAIMS
-- ============================================================

CREATE TABLE claims (
    id                  TEXT PRIMARY KEY,
    agent_id            TEXT NOT NULL REFERENCES entities(id),
    claim_code          TEXT UNIQUE NOT NULL,

    requested_scopes    TEXT[],
    machine_id          TEXT,
    description         TEXT,
    context             JSONB,

    status              TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'declined', 'expired')),
    approved_by         TEXT,
    delegation_id       TEXT,
    approved_scopes     TEXT[],

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    resolved_at         TIMESTAMPTZ
);

CREATE INDEX idx_claims_agent ON claims(agent_id);
CREATE INDEX idx_claims_status ON claims(status);
CREATE INDEX idx_claims_code ON claims(claim_code);

-- ============================================================
-- APPROVAL TEMPLATES
-- ============================================================

CREATE TABLE approval_templates (
    id                  TEXT PRIMARY KEY,
    machine_id          TEXT NOT NULL REFERENCES entities(id),
    version             INTEGER NOT NULL DEFAULT 1,

    brand_name          TEXT NOT NULL,
    brand_logo_url      TEXT,
    brand_colors        JSONB,
    verified_domain     TEXT,
    domain_verified     BOOLEAN NOT NULL DEFAULT FALSE,

    approval_type       TEXT NOT NULL,
    title               TEXT NOT NULL,
    description         TEXT,
    display_fields      JSONB NOT NULL,
    required_trust_level INTEGER NOT NULL DEFAULT 1,
    scopes_requested    TEXT[],

    active              BOOLEAN NOT NULL DEFAULT TRUE,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_machine ON approval_templates(machine_id);
CREATE UNIQUE INDEX idx_templates_machine_type ON approval_templates(machine_id, approval_type);

-- ============================================================
-- PENDING ATTESTATIONS (temporary, cleaned up after verification)
-- ============================================================

CREATE TABLE pending_attestations (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    entity_id           TEXT NOT NULL REFERENCES entities(id),
    type                TEXT NOT NULL,  -- 'email', 'phone'
    target              TEXT NOT NULL,  -- email address or phone number
    code                TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 hour'
);

CREATE INDEX idx_pending_code ON pending_attestations(code);
