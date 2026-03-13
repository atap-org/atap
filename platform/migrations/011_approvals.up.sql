-- Approval documents for multi-signature consent (spec §8)
CREATE TABLE approvals (
    id           TEXT PRIMARY KEY,              -- "apr_" + ULID
    state        TEXT NOT NULL DEFAULT 'requested',
                                                -- requested|approved|declined|expired
                                                -- rejected|consumed|revoked
    from_did     TEXT NOT NULL,
    to_did       TEXT NOT NULL,
    via_did      TEXT,                          -- NULL for two-party
    parent_id    TEXT REFERENCES approvals(id),
    document     JSONB NOT NULL,               -- full approval JSON (for retrieval + signing)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until  TIMESTAMPTZ,                  -- NULL = one-time approval
    responded_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approvals_from ON approvals(from_did, state);
CREATE INDEX idx_approvals_to ON approvals(to_did, state);
CREATE INDEX idx_approvals_via ON approvals(via_did) WHERE via_did IS NOT NULL;
CREATE INDEX idx_approvals_parent ON approvals(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_approvals_expires ON approvals(valid_until)
    WHERE state = 'approved' AND valid_until IS NOT NULL;
