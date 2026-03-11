# ATAP Platform — Claude Code Instructions

## What This Is
ATAP (Agent Trust and Authority Protocol) is an open protocol for verifiable
delegation of trust between AI agents, machines, humans, and organizations.

- atap.dev = protocol spec, SDKs, docs (open source)
- atap.app = hosted platform, mobile app, API (open source, asset is trust data)

## Monorepo Structure
```
platform/    — Go backend (Fiber framework)
mobile/      — Flutter app (iOS + Android)
sdks/        — Python, JS, Go client SDKs
spec/        — Protocol specification + JSON schemas
web/         — atap.dev static site
docs/        — Documentation
```

## Key Architecture Decisions
- Backend: Go 1.22+ with Fiber v2
- Database: PostgreSQL 16 with JSONB for attestations and flexible payloads
- Real-time: Redis pub/sub → SSE delivery to agents
- Mobile: Flutter 3.x (iOS + Android single codebase)
- Crypto: Ed25519 signing, X25519 encryption (NaCl/libsodium)
- License: Apache 2.0 for everything

## Build Order
Follow phase order strictly. Phase 1 must work end-to-end before Phase 2.
1. Phase 1 (Weeks 1-4): Agent registration, inbox, SSE delivery, webhooks, SDKs, mobile app foundation
2. Phase 2 (Weeks 5-8): Human entities, attestations, claim flow, delegations, World ID, SIMRelay
3. Phase 3 (Weeks 9-12): Branded templates, orgs, encryption, monetization
4. Phase 4 (Months 4-6): Federation, spec publication, ecosystem integrations

## Entity Model
- agent:// — AI agents (ID assigned by registry, ULID-based)
- machine:// — Persistent services (ID assigned by registry, can be descriptive)
- human:// — Trust anchors (ID derived from Ed25519 public key)
- org:// — Organizational umbrellas

## Human Identity
Human IDs are derived from their public key, NOT from email or phone:
```
human_id = lowercase(base32(sha256(ed25519_public_key))[:16])
```
Email, phone, World ID are ATTESTATIONS — verified properties stored in JSONB.
Attestations raise trust level but are NOT the identity.
Removing attestations (GDPR) MUST NOT break delegation chains.

## Code Style
- Go: standard library conventions, gofmt, no frameworks beyond Fiber
- Error handling: always wrap with context, use RFC 7807 for API errors
- Logging: zerolog, structured JSON
- Tests: table-driven tests, test files next to source
- SQL: use migrations, never raw DDL in application code

## ID Patterns
- Signal IDs: "sig_" + ULID
- Entity IDs (agent/machine): lowercase alphanumeric + hyphens, 4-64 chars
- Human IDs: lowercase(base32(sha256(ed25519_pubkey))[:16])
- Channel IDs: "chn_" + random hex (16 chars)
- Delegation IDs: "del_" + random hex (12 chars)
- Claim IDs: "clm_" + random hex (12 chars)
- Claim codes: "ATAP-" + 4 uppercase alphanumeric
- Tokens: "atap_" + 32 bytes base64url, stored as SHA-256 hash

## Canonical JSON (for signatures)
Sorted keys, no whitespace, no trailing newline.
Sign over: canonical(route) + "." + canonical(signal)

## SSE Delivery
- Primary delivery method for agents
- Last-Event-ID header for reconnection (replay missed signals from PostgreSQL)
- 30-second heartbeat comments to keep connection alive
- Redis pub/sub channel per entity inbox: "inbox:{entity-id}"

## Security Rules
- Private keys NEVER leave the device that generated them
- Human keys: device secure enclave only (mobile)
- Agent/machine keys: generated server-side, returned once at registration
- Platform CANNOT read encrypted signal bodies (zero-knowledge relay)
- All delegation documents are self-verifying (no phone-home needed)
- Tokens stored hashed (SHA-256), never plaintext
- Rate limit all endpoints

## Database
- PostgreSQL 16 with pgcrypto extension
- JSONB for: attestations, delegation constraints, signal data, template schemas
- Migrations in platform/migrations/ (numbered, SQL)
- Use golang-migrate

## Testing
- Unit tests next to source files (*_test.go)
- Integration tests in platform/test/
- Run: `go test ./...` from platform/
