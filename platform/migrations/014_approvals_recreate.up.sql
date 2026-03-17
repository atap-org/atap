-- Re-create approvals table for delegate routing persistence (MSG-06).
-- Migration 012 dropped this table when adding revocations.
-- Phase 4 approval persistence requires it for first-response-wins semantics.
CREATE TABLE IF NOT EXISTS approvals (
    id           TEXT PRIMARY KEY,
    state        TEXT NOT NULL DEFAULT 'requested',
    from_did     TEXT NOT NULL,
    to_did       TEXT NOT NULL,
    via_did      TEXT,
    parent_id    TEXT REFERENCES approvals(id),
    document     JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until  TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_approvals_from ON approvals(from_did, state);
CREATE INDEX IF NOT EXISTS idx_approvals_to ON approvals(to_did, state);
CREATE INDEX IF NOT EXISTS idx_approvals_via ON approvals(via_did) WHERE via_did IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_approvals_parent ON approvals(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_approvals_expires ON approvals(valid_until)
    WHERE state = 'approved' AND valid_until IS NOT NULL;
