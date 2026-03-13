-- Migration 008: Strip old signal pipeline
-- Drops all tables from the signal-based protocol (signals, channels, webhooks, claims, delegations, push_tokens).
-- The entities table survives; its schema is extended in migration 009.

DROP TABLE IF EXISTS push_tokens CASCADE;
DROP TABLE IF EXISTS delegations CASCADE;
DROP TABLE IF EXISTS claims CASCADE;
DROP TABLE IF EXISTS delivery_attempts CASCADE;
DROP TABLE IF EXISTS webhook_delivery CASCADE;
DROP TABLE IF EXISTS webhook_configs CASCADE;
DROP TABLE IF EXISTS channels CASCADE;
DROP TABLE IF EXISTS signals CASCADE;
