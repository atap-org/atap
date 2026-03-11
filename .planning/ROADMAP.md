# Roadmap: ATAP Phase 1 — "The Doorbell"

## Overview

ATAP Phase 1 delivers a working signal broker: any AI agent can register with a cryptographic identity, get a durable inbox, and receive signals via SSE streaming, webhook push, or polling — even while offline. The build follows the component dependency graph: foundation (infrastructure, crypto, auth, registration) first, then the full signal delivery pipeline (inbox, SSE, webhooks, channels), then the Flutter mobile app as the human-facing client. Three phases, strict sequential dependency.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Infrastructure, crypto primitives, auth middleware, and entity registration
- [ ] **Phase 2: Signal Pipeline** - Signal delivery, SSE streaming, webhook push, inbound channels, and integration tests
- [ ] **Phase 3: Mobile App** - Flutter app with registration, inbox view, and push notifications

## Phase Details

### Phase 1: Foundation
**Goal**: A running platform where agents can register with cryptographic identity and be looked up by other entities
**Depends on**: Nothing (first phase)
**Requirements**: REG-01, REG-02, REG-03, REG-04, REG-05, CRY-01, CRY-02, CRY-03, CRY-04, AUTH-01, AUTH-02, ERR-01, ERR-02, INF-01, INF-02, INF-03, INF-04, INF-05, INF-06, TST-03, TST-04
**Success Criteria** (what must be TRUE):
  1. `docker compose up` starts the full stack (platform + PostgreSQL + Redis) and the health endpoint responds with protocol version and status
  2. An agent can POST to `/v1/register` and receive an entity URI, bearer token, and public key within 1 second
  3. A registered entity can be looked up via GET `/v1/entities/{id}` by any caller (no auth required) and the response includes the public key and metadata but no secrets
  4. Authenticated requests with a valid bearer token succeed; requests without a token or with an invalid token receive RFC 7807 error responses
  5. Unit tests pass for Ed25519 keypair generation, canonical JSON signing (RFC 8785 JCS), signature verification, token generation, and token hash verification
**Plans**: 2 plans

Plans:
- [x] 01-01-PLAN.md — Infrastructure, crypto primitives (JCS, 128-bit channel IDs), models, migration, Dockerfile
- [x] 01-02-PLAN.md — Store, API handlers (4 endpoints), HTTP tests, main.go wiring with golang-migrate

### Phase 2: Signal Pipeline
**Goal**: Agents can send signals to any entity's inbox and receive them in real-time via SSE, webhook push, or polling — with durable persistence and no signal loss
**Depends on**: Phase 1
**Requirements**: SIG-01, SIG-02, SIG-03, SIG-04, SIG-05, SIG-06, SSE-01, SSE-02, SSE-03, SSE-04, WHK-01, WHK-02, WHK-03, WHK-04, CHN-01, CHN-02, CHN-03, CHN-04, CHN-05, TST-01, TST-02
**Success Criteria** (what must be TRUE):
  1. An authenticated entity can send a signal to another entity's inbox and the recipient can retrieve it via cursor-based pagination polling
  2. An entity connected to the SSE stream receives signals in real-time; after disconnecting and reconnecting with Last-Event-ID, all missed signals are replayed without loss
  3. An entity with a registered webhook URL receives signal payloads via HTTP POST with a valid Ed25519 signature in the X-ATAP-Signature header; failed deliveries retry with exponential backoff
  4. An entity can create an inbound channel with a unique webhook URL, external services can POST to that URL, and the payload arrives in the entity's inbox as an ATAP signal
  5. Integration tests using testcontainers-go pass the full agent lifecycle: register, send signal, receive via SSE, verify persistence across restart
**Plans**: 4 plans

Plans:
- [x] 02-01-PLAN.md — Models, migrations (signals/channels/webhooks), store methods, crypto helpers
- [ ] 02-02-PLAN.md — Signal sending, inbox polling, SSE streaming with Redis pub/sub, unit tests
- [ ] 02-03-PLAN.md — Webhook delivery worker with retry, inbound channels (trusted + open), unit tests
- [ ] 02-04-PLAN.md — Integration tests with testcontainers-go (full lifecycle, SSE, webhooks, channels)

### Phase 3: Mobile App
**Goal**: A Flutter mobile app where a user can register an agent, view their inbox, and receive push notifications for new signals
**Depends on**: Phase 2
**Requirements**: MOB-01, MOB-02, MOB-03, MOB-04
**Success Criteria** (what must be TRUE):
  1. A user can open the Flutter app, register a new agent entity via the platform API, and see confirmation of successful registration
  2. The inbox view displays received signals with pull-to-refresh, showing signal metadata and payload
  3. When a new signal arrives in the entity's inbox, the device receives a push notification (FCM on Android, APNs on iOS) even when the app is in the background
**Plans**: TBD

Plans:
- [ ] 03-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 2/2 | Complete | 2026-03-11 |
| 2. Signal Pipeline | 1/4 | In progress | - |
| 3. Mobile App | 0/1 | Not started | - |
