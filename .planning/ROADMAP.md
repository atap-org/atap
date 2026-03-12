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
- [ ] **Phase 4: Fix Signal Pipeline Bugs** - JSON field mismatch, pagination param, webhook retry payload, race condition
- [ ] **Phase 5: Mobile Push Notifications** - Flutter Signal.fromJson fix, firebase_messaging setup, FCM token acquisition

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
**Goal**: Flutter mobile app with human onboarding via claim links, card-based inbox with SSE streaming, and push notifications -- plus platform extensions for claims, delegations, human registration, and push delivery
**Depends on**: Phase 2
**Requirements**: MOB-01, MOB-02, MOB-03, MOB-04
**Success Criteria** (what must be TRUE):
  1. A user can open the Flutter app, register a new agent entity via the platform API, and see confirmation of successful registration
  2. The inbox view displays received signals with pull-to-refresh, showing signal metadata and payload
  3. When a new signal arrives in the entity's inbox, the device receives a push notification (FCM on Android, APNs on iOS) even when the app is in the background
**Plans**: 5 plans

Plans:
- [ ] 03-01-PLAN.md — Platform data layer: migrations (claims, delegations, push tokens), models, store methods
- [ ] 03-02-PLAN.md — Flutter project scaffold, Ed25519 cross-language validation, core services (crypto, API client, secure storage)
- [ ] 03-03-PLAN.md — Platform API endpoints: claims, human registration, delegations, push tokens, push notification service
- [ ] 03-04-PLAN.md — Flutter features: onboarding flow, inbox view with SSE, signal detail, push notification handling
- [ ] 03-05-PLAN.md — API tests for new endpoints and human verification checkpoint

### Phase 4: Fix Signal Pipeline Bugs
**Goal**: Fix all cross-language integration bugs that break signal display, inbox pagination, and webhook retry delivery
**Depends on**: Phase 2, Phase 3
**Requirements**: SIG-04, SSE-01, WHK-03
**Gap Closure:** Closes gaps from v1.0 audit
**Success Criteria** (what must be TRUE):
  1. Flutter `Signal.fromJson` parses the `ts` field from Go API response without errors (Go uses `json:"ts"`, not `created_at`)
  2. Flutter inbox `loadMore()` sends `?after=` parameter matching Go's expected query param, and pagination advances correctly
  3. Webhook retry re-enqueues signal with full payload (not empty body)
  4. Concurrent claim redemption returns 409 Conflict (not 500) when claim is already redeemed
**Plans**: 1 plan

Plans:
- [ ] 04-01-PLAN.md — Webhook retry payload fetch, claim 409 error handling, Flutter Signal.fromJson ts fix, pagination param fix

### Phase 5: Mobile Push Notifications
**Goal**: Complete mobile push notification pipeline — Flutter FCM integration, token acquisition, and Signal.fromJson crash fix
**Depends on**: Phase 4
**Requirements**: MOB-02, MOB-03
**Gap Closure:** Closes gaps from v1.0 audit
**Success Criteria** (what must be TRUE):
  1. Flutter `Signal.fromJson` correctly parses all fields from the Go API response without crashes
  2. `firebase_messaging` is in pubspec.yaml and the app acquires an FCM token on startup
  3. FCM token is registered with the platform via `POST /v1/entities/{id}/push-token`
  4. A new signal triggers a push notification on the device even when the app is in background
**Plans**: TBD

Plans:
- [ ] 05-01-PLAN.md — Flutter Signal.fromJson fix, firebase_messaging setup, FCM token acquisition and registration

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 2/2 | Complete | 2026-03-11 |
| 2. Signal Pipeline | 4/4 | Complete | 2026-03-11 |
| 3. Mobile App | 5/5 | Complete | 2026-03-12 |
| 4. Fix Signal Pipeline Bugs | 0/1 | Pending |  |
| 5. Mobile Push Notifications | 0/1 | Pending |  |
