CREATE TABLE delegations (
    id            TEXT PRIMARY KEY,
    delegator_id  TEXT NOT NULL REFERENCES entities(id),
    delegate_id   TEXT NOT NULL REFERENCES entities(id),
    scope         JSONB DEFAULT '[]',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ,
    UNIQUE(delegator_id, delegate_id)
);

CREATE INDEX idx_delegations_delegator_id ON delegations (delegator_id);
CREATE INDEX idx_delegations_delegate_id ON delegations (delegate_id);
