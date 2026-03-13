-- Add X25519 key agreement columns to entities
ALTER TABLE entities ADD COLUMN x25519_public_key BYTEA;
ALTER TABLE entities ADD COLUMN x25519_private_key BYTEA;
-- NOTE: x25519_private_key stored plaintext in Phase 2; will be encrypted per-entity in Phase 4 (PRV-01)

-- DIDComm message queue for offline delivery (MSG-02)
CREATE TABLE didcomm_messages (
    id             TEXT PRIMARY KEY,           -- "msg_" + ULID
    recipient_did  TEXT NOT NULL,
    sender_did     TEXT,                        -- NULL for anoncrypt
    message_type   TEXT,                        -- type URI (only populated if server is recipient)
    payload        BYTEA NOT NULL,              -- raw JWE bytes (encrypted, opaque to server)
    state          TEXT NOT NULL DEFAULT 'pending',  -- pending | delivered | expired
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ,
    delivered_at   TIMESTAMPTZ
);

CREATE INDEX idx_didcomm_messages_recipient ON didcomm_messages(recipient_did, state);
CREATE INDEX idx_didcomm_messages_expires ON didcomm_messages(expires_at) WHERE state = 'pending';
