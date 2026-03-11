# Requirements: ATAP Phase 1 — "The Doorbell"

**Defined:** 2026-03-11
**Core Value:** Any AI agent can register, get an inbox, and receive signals — even while offline.

## v1 Requirements

Requirements for Phase 1. Each maps to roadmap phases.

### Registration & Identity

- [ ] **REG-01**: Agent can self-register via `POST /v1/register` and receive entity URI, bearer token, public key, and inbox URLs in <1s
- [ ] **REG-02**: Registration generates Ed25519 keypair and ULID-based entity ID
- [ ] **REG-03**: Bearer token uses `atap_` prefix + 32 bytes base64url, stored as SHA-256 hash (never plaintext)
- [ ] **REG-04**: Entity can be looked up via `GET /v1/entities/{id}` (public endpoint, returns public key + metadata, no secrets)
- [ ] **REG-05**: Entity URI scheme enforced: `agent://{ulid}` format

### Signal Delivery

- [ ] **SIG-01**: Authenticated entity can send a signal to any entity's inbox via `POST /v1/inbox/{target-id}`
- [ ] **SIG-02**: Signals follow the ATAP format: route (origin/target/reply_to/channel/thread/ref), signal (type/encrypted/data), context (source/idempotency/tags/ttl/priority)
- [ ] **SIG-03**: Signals persist in PostgreSQL and survive platform restarts
- [ ] **SIG-04**: Inbox supports cursor-based pagination via `GET /v1/inbox/{entity-id}?after={cursor}&limit=50`
- [ ] **SIG-05**: Expired signals (past TTL) are excluded from inbox queries
- [ ] **SIG-06**: Idempotency key deduplication within 24-hour window via unique index

### SSE Streaming

- [ ] **SSE-01**: Entity can open SSE stream via `GET /v1/inbox/{entity-id}/stream` and receive signals in real-time
- [ ] **SSE-02**: SSE reconnection replays missed signals using `Last-Event-ID` header from PostgreSQL
- [ ] **SSE-03**: 30-second heartbeat comments keep connections alive through proxies
- [ ] **SSE-04**: PostgreSQL write completes before Redis publish (write-then-notify pattern to prevent signal loss)

### Webhook Delivery

- [ ] **WHK-01**: Platform pushes signals to entity's registered webhook URL via HTTP POST
- [ ] **WHK-02**: Webhook payload is signed with Ed25519, signature in `X-ATAP-Signature` header
- [ ] **WHK-03**: Failed webhooks retry with exponential backoff (1s, 5s, 30s, 5m, 30m), max 5 attempts
- [ ] **WHK-04**: Undeliverable signals marked after max retries

### Channels

- [ ] **CHN-01**: Entity can create inbound channels via `POST /v1/entities/{id}/channels` with label, tags, and optional expiration
- [ ] **CHN-02**: Each channel has a unique webhook URL that external services POST to
- [ ] **CHN-03**: Inbound webhook payloads are wrapped into ATAP signals and delivered to entity's inbox
- [ ] **CHN-04**: Entity can list own channels and revoke individual channels without affecting others
- [ ] **CHN-05**: Channel webhook URL uses 128-bit entropy (not 64-bit) for security

### Auth & Errors

- [ ] **AUTH-01**: All mutating/reading endpoints (except register, health, entity lookup, verify) require `Authorization: Bearer {token}` header
- [ ] **AUTH-02**: Auth middleware validates token by SHA-256 hash lookup, returns entity context
- [ ] **ERR-01**: All error responses follow RFC 7807 Problem Details format with type, title, status, detail, instance
- [ ] **ERR-02**: Health endpoint `GET /v1/health` returns protocol version, status, and timestamp

### Crypto

- [ ] **CRY-01**: Ed25519 keypair generation using Go stdlib `crypto/ed25519`
- [ ] **CRY-02**: Canonical JSON signing uses RFC 8785 (JCS) for cross-language compatibility
- [ ] **CRY-03**: Signable payload format: `JCS(route) + "." + JCS(signal)` signed with Ed25519
- [ ] **CRY-04**: Channel IDs use 128-bit random entropy (`chn_` + 32 hex chars)

### Infrastructure

- [ ] **INF-01**: `docker compose up` starts full stack (platform + PostgreSQL 16 + Redis 7) in under 60 seconds
- [ ] **INF-02**: Dockerfile produces cloud-deployable platform binary (multi-stage Alpine build)
- [ ] **INF-03**: Database migrations in numbered SQL files, run via golang-migrate
- [ ] **INF-04**: Structured JSON logging via zerolog
- [ ] **INF-05**: Graceful shutdown on SIGTERM/SIGINT
- [ ] **INF-06**: Go dependencies updated to current versions (pgx v5.7+, go-redis v9.7+, zerolog v1.34+)

### Mobile App Foundation

- [ ] **MOB-01**: Flutter app with entity registration screen (creates agent via platform API)
- [ ] **MOB-02**: Inbox view displaying received signals with pull-to-refresh
- [ ] **MOB-03**: Push notification setup (FCM for Android, APNs for iOS) — token registered with platform
- [ ] **MOB-04**: Platform stores push token per entity and sends push notification on new signal

### Testing

- [ ] **TST-01**: Integration tests covering full agent lifecycle: register → send signal → receive via SSE
- [ ] **TST-02**: Integration tests use testcontainers-go for real PostgreSQL and Redis (no mocks)
- [ ] **TST-03**: Unit tests for crypto functions (keypair generation, signing, verification, canonical JSON)
- [ ] **TST-04**: Unit tests for token generation and hash verification

## v2 Requirements

Deferred to Phase 2+. Tracked but not in current roadmap.

### Trust Chain (Phase 2)

- **DEL-01**: Human entity registration with key derived from Ed25519 public key
- **DEL-02**: Attestation storage and verification (email, phone)
- **DEL-03**: Claim flow (agent-initiated trust elevation)
- **DEL-04**: Delegation document creation and chain verification
- **DEL-05**: Trust level inheritance and enforcement
- **DEL-06**: World ID integration (Trust Level 2)
- **DEL-07**: SIMRelay reverse SMS verification (Trust Level 1)

### Marketplace (Phase 3)

- **MKT-01**: Branded approval templates
- **MKT-02**: Organization entities
- **MKT-03**: End-to-end encryption (X25519)
- **MKT-04**: Rate limiting per tier / monetization

### Ecosystem (Phase 4)

- **ECO-01**: Federation and key discovery (DNS, well-known endpoints)
- **ECO-02**: Client SDKs (Python, JS, Go)
- **ECO-03**: Spec publication

## Out of Scope

| Feature | Reason |
|---------|--------|
| Client SDKs | API not stable yet; ship after Phase 1 freezes. Provide curl examples + OpenAPI spec. |
| Landing page (atap.dev) | README with quickstart is sufficient until platform is deployed |
| Signal signature verification middleware | Phase 1 entities are Trust Level 0; trust block accepted but not verified |
| Key recovery | Only needed for human entities (Phase 2) |
| Token rotation endpoint | Desirable but not blocking for Phase 1; agents can re-register |
| Redis Streams | Pub/sub with write-then-notify pattern is sufficient; Streams add complexity |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REG-01 | TBD | Pending |
| REG-02 | TBD | Pending |
| REG-03 | TBD | Pending |
| REG-04 | TBD | Pending |
| REG-05 | TBD | Pending |
| SIG-01 | TBD | Pending |
| SIG-02 | TBD | Pending |
| SIG-03 | TBD | Pending |
| SIG-04 | TBD | Pending |
| SIG-05 | TBD | Pending |
| SIG-06 | TBD | Pending |
| SSE-01 | TBD | Pending |
| SSE-02 | TBD | Pending |
| SSE-03 | TBD | Pending |
| SSE-04 | TBD | Pending |
| WHK-01 | TBD | Pending |
| WHK-02 | TBD | Pending |
| WHK-03 | TBD | Pending |
| WHK-04 | TBD | Pending |
| CHN-01 | TBD | Pending |
| CHN-02 | TBD | Pending |
| CHN-03 | TBD | Pending |
| CHN-04 | TBD | Pending |
| CHN-05 | TBD | Pending |
| AUTH-01 | TBD | Pending |
| AUTH-02 | TBD | Pending |
| ERR-01 | TBD | Pending |
| ERR-02 | TBD | Pending |
| CRY-01 | TBD | Pending |
| CRY-02 | TBD | Pending |
| CRY-03 | TBD | Pending |
| CRY-04 | TBD | Pending |
| INF-01 | TBD | Pending |
| INF-02 | TBD | Pending |
| INF-03 | TBD | Pending |
| INF-04 | TBD | Pending |
| INF-05 | TBD | Pending |
| INF-06 | TBD | Pending |
| MOB-01 | TBD | Pending |
| MOB-02 | TBD | Pending |
| MOB-03 | TBD | Pending |
| MOB-04 | TBD | Pending |
| TST-01 | TBD | Pending |
| TST-02 | TBD | Pending |
| TST-03 | TBD | Pending |
| TST-04 | TBD | Pending |

**Coverage:**
- v1 requirements: 42 total
- Mapped to phases: 0
- Unmapped: 42

---
*Requirements defined: 2026-03-11*
*Last updated: 2026-03-11 after initial definition*
