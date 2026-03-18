CREATE TABLE rate_limit_config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO rate_limit_config (key, value) VALUES
    ('public_rpm',   '30'),
    ('auth_rpm',     '120'),
    ('ip_allowlist', '[]');
