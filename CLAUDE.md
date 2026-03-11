# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ATAP (Agent Trust and Authority Protocol) — open protocol for verifiable delegation of trust between AI agents, machines, humans, and organizations. Every entity gets a cryptographic identity and a verifiable delegation chain.

- **atap.dev** — protocol spec, SDKs, docs
- **atap.app** — hosted platform, mobile app

## Build & Run

```bash
# Full stack with Docker
docker compose up -d

# Local development (requires Go 1.22+, Docker for Postgres/Redis)
docker compose up -d postgres redis
cd platform && go build -o ../bin/atap-platform ./cmd/server && ../bin/atap-platform

# Or use the setup script
./scripts/setup.sh

# Run tests
cd platform && go test ./...

# Run a single test
cd platform && go test ./internal/store -run TestEntityCreate
```

**Environment variables** (all have defaults for local dev): `PORT`, `DATABASE_URL`, `REDIS_URL`, `PLATFORM_DOMAIN`. See `platform/internal/config/config.go` for the full list.

## Repository Structure

```
platform/           — Go backend (the main codebase right now)
  cmd/server/       — Entrypoint (main.go)
  internal/
    api/            — HTTP handlers + routes (Fiber v2), auth middleware
    config/         — Env-based configuration
    crypto/         — Ed25519 keys, token generation, ID generation, canonical JSON
    models/         — All domain types (Entity, Signal, Channel, Delegation, Claim, etc.)
    store/          — PostgreSQL data access layer (pgx/v5 pool)
  migrations/       — Numbered SQL migrations (also used as initdb scripts)
mobile/             — Flutter app (iOS + Android) — not yet built
spec/               — Protocol specification
docs/               — Build guide and documentation
```

## Architecture

**Request flow:** Fiber HTTP → `api.Handler` (auth middleware checks Bearer token via SHA-256 hash lookup) → `store.Store` (pgx connection pool) → PostgreSQL. Real-time delivery uses Redis pub/sub → SSE streams.

**Key patterns:**
- All API routes are under `/v1/`, defined in `api.SetupRoutes()`
- Auth: Bearer tokens prefixed `atap_`, stored as SHA-256 hashes. Token returned once at registration.
- Errors follow RFC 7807 (Problem Details) via the `problem()` helper
- SSE: Redis pub/sub channel per entity (`inbox:{entity-id}`), 30s heartbeat, Last-Event-ID replay from PostgreSQL
- Channels are inbound webhook URLs that wrap external payloads into ATAP signals

## Entity Model & Identity

Four entity types with URI schemes: `agent://`, `machine://`, `human://`, `org://`

**Human IDs are derived from their public key**, not email/phone:
```
human_id = lowercase(base32(sha256(ed25519_pubkey))[:16])
```
Email, phone, World ID are **attestations** (verified properties in JSONB) — they raise trust level but are NOT the identity. Removing attestations (GDPR) must NOT break delegation chains.

## ID Conventions

| Type | Format |
|------|--------|
| Entity | ULID (lowercase) |
| Signal | `sig_` + ULID |
| Channel | `chn_` + 16 hex chars |
| Delegation | `del_` + 12 hex chars |
| Claim | `clm_` + 12 hex chars |
| Claim code | `ATAP-` + 4 alphanumeric |
| Key | `key_{prefix}_{hex}` |
| Token | `atap_` + 32 bytes base64url |

## Code Style

- Go standard library conventions, `gofmt`
- Error handling: always wrap with context (`fmt.Errorf("action: %w", err)`)
- Logging: zerolog, structured JSON
- Tests: table-driven, test files next to source (`*_test.go`)
- SQL: use numbered migrations in `platform/migrations/`, never raw DDL in app code
- No frameworks beyond Fiber for HTTP

## Crypto Rules

- Ed25519 signing, X25519 encryption (NaCl/libsodium)
- Private keys NEVER leave the device that generated them
- Canonical JSON for signatures: sorted keys, no whitespace — sign over `canonical(route) + "." + canonical(signal)`
- Tokens stored as SHA-256 hash, never plaintext

## Build Phases

The project follows a strict phase order (each must work end-to-end before the next):
1. **Phase 1** — Agent registration, inbox, SSE delivery, webhooks, channels, SDKs (current phase)
2. **Phase 2** — Human entities, attestations, claims, delegations, World ID
3. **Phase 3** — Branded templates, orgs, encryption, monetization
4. **Phase 4** — Federation, spec publication, ecosystem integrations

Detailed specifications for all phases are in `ATAP-BUILD-GUIDE.md` and `docs/BUILD-GUIDE.md`.
