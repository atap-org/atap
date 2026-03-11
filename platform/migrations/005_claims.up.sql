CREATE TABLE claims (
    id          TEXT PRIMARY KEY,
    code        TEXT UNIQUE NOT NULL,
    creator_id  TEXT NOT NULL REFERENCES entities(id),
    redeemed_by TEXT REFERENCES entities(id),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'redeemed', 'expired')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ
);

CREATE INDEX idx_claims_code ON claims (code);
CREATE INDEX idx_claims_creator_id ON claims (creator_id);
