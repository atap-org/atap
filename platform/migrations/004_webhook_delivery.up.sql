CREATE TABLE delivery_attempts (
    id            TEXT PRIMARY KEY,
    signal_id     TEXT NOT NULL REFERENCES signals(id),
    webhook_url   TEXT NOT NULL,
    attempt       INTEGER NOT NULL,
    status_code   INTEGER,
    error         TEXT,
    next_retry_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_signal ON delivery_attempts(signal_id);
CREATE INDEX idx_delivery_retry ON delivery_attempts(next_retry_at) WHERE next_retry_at IS NOT NULL;

-- Cleanup: delivery attempts older than 24h (app-level job)
CREATE INDEX idx_delivery_cleanup ON delivery_attempts(created_at);
