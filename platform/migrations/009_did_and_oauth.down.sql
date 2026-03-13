-- Migration 009 down: undo DID and OAuth 2.1 schema

DROP INDEX IF EXISTS idx_oauth_tokens_expires;
DROP INDEX IF EXISTS idx_oauth_tokens_entity;
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS oauth_auth_codes;
DROP TABLE IF EXISTS key_versions;

ALTER TABLE entities DROP COLUMN IF EXISTS client_secret_hash;
ALTER TABLE entities DROP COLUMN IF EXISTS principal_did;
ALTER TABLE entities DROP COLUMN IF EXISTS did;
