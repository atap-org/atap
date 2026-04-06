-- Migration 016: Claims — human-to-agent claim flow
-- An agent creates a claim (short code + link). A human opens the link,
-- authenticates via email OTP, and approves. The platform creates the
-- human entity (server-side custody) and sets principal_did on the agent.

CREATE TABLE IF NOT EXISTS claims (
    id          TEXT PRIMARY KEY,                  -- "clm_" + 12 hex chars
    code        TEXT UNIQUE NOT NULL,              -- "ATAP-XXXX" short code
    agent_id    TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    agent_name  TEXT NOT NULL DEFAULT '',           -- snapshot for display
    description TEXT NOT NULL DEFAULT '',           -- what the agent does
    scopes      TEXT[] NOT NULL DEFAULT '{}',       -- requested scopes
    status      TEXT NOT NULL DEFAULT 'pending',    -- pending | redeemed | expired | declined
    redeemed_by TEXT REFERENCES entities(id),       -- human entity ID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_claims_code ON claims (code);
CREATE INDEX IF NOT EXISTS idx_claims_agent_id ON claims (agent_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON claims (status) WHERE status = 'pending';
