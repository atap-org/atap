DROP TABLE IF EXISTS approvals CASCADE;

CREATE TABLE revocations (
    id           TEXT PRIMARY KEY,
    approval_id  TEXT NOT NULL UNIQUE,
    approver_did TEXT NOT NULL,
    revoked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_revocations_approver ON revocations(approver_did, expires_at);
CREATE INDEX idx_revocations_expires  ON revocations(expires_at);
