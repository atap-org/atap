---
phase: 01-foundation
plan: 02
subsystem: api
tags: [go, fiber, ed25519, signed-requests, postgresql, golang-migrate, http-tests]

# Dependency graph
requires:
  - phase: 01-foundation-01
    provides: Ed25519 crypto primitives, entity models, entities migration, Dockerfile
provides:
  - PostgreSQL store with CreateEntity, GetEntity, GetEntityByPublicKey
  - 4 HTTP endpoints (health, register, entity lookup, authenticated me)
  - Ed25519 signed request auth middleware (replaces bearer token auth)
  - SignRequest/VerifyRequest crypto functions
  - HTTP-level tests with fake store (no Docker required)
  - golang-migrate auto-migration on startup
  - Graceful shutdown on SIGTERM
affects: [02-01, 02-02]

# Tech tracking
tech-stack:
  added: [golang-migrate/migrate/v4]
  patterns: [Ed25519 signed request auth, fake store interface for testing, RFC 7807 error responses]

key-files:
  created:
    - platform/internal/api/api_test.go
  modified:
    - platform/internal/store/store.go
    - platform/internal/api/api.go
    - platform/cmd/server/main.go
    - platform/internal/crypto/crypto.go
    - platform/internal/crypto/crypto_test.go
    - platform/internal/models/models.go
    - platform/migrations/001_entities.up.sql

key-decisions:
  - "Replaced bearer token auth with Ed25519 signed request auth (user decision during checkpoint)"
  - "Auth middleware verifies Ed25519 signatures instead of token hash lookup"
  - "Removed token_hash from entities table and store — identity is the public key"
  - "GetEntityByPublicKey replaces GetEntityByTokenHash in the store interface"

patterns-established:
  - "Signed request auth: clients sign method+path+timestamp+body with Ed25519 private key"
  - "EntityStore interface enables testing without PostgreSQL (fake store with in-memory map)"
  - "RFC 7807 ProblemDetail for all error responses via problem() helper"

requirements-completed: [REG-01, REG-04, AUTH-01, AUTH-02, ERR-01, ERR-02, INF-04, INF-05, INF-01, INF-02, INF-03]

# Metrics
duration: multi-session
completed: 2026-03-11
---

# Phase 1 Plan 02: Store, API & HTTP Tests Summary

**4 HTTP endpoints with Ed25519 signed request auth, fake-store HTTP tests, and golang-migrate wiring -- verified end-to-end via Docker Compose**

## Performance

- **Duration:** Multi-session (included checkpoint for user verification)
- **Started:** 2026-03-11T16:42:00Z
- **Completed:** 2026-03-11T17:39:00Z
- **Tasks:** 3 (2 auto + 1 human-verify checkpoint)
- **Files modified:** 7

## Accomplishments
- Store trimmed to Phase 1: CreateEntity, GetEntity, GetEntityByPublicKey (no signal/channel methods)
- API layer with 4 endpoints: health, register, entity lookup, authenticated /v1/me
- Ed25519 signed request auth replaces bearer token auth (design change during execution)
- HTTP-level tests using fake store -- all pass without Docker/Postgres
- golang-migrate runs migrations on startup
- Full stack verified via Docker Compose (health, register, signed auth all working)

## Task Commits

Each task was committed atomically:

1. **Task 1: Trim store, rewrite API to 4 endpoints, wire main.go with golang-migrate** - `6f1febf` (feat)
2. **Task 2: HTTP-level tests for all 4 endpoints and auth middleware** - `177ac01` (test)
3. **Task 3: Docker Compose UAT** - user-verified (checkpoint)

## Files Created/Modified
- `platform/internal/store/store.go` - Entities-only CRUD (CreateEntity, GetEntity, GetEntityByPublicKey)
- `platform/internal/api/api.go` - 4 endpoints + Ed25519 signed request auth middleware + RFC 7807 errors
- `platform/internal/api/api_test.go` - HTTP tests with fake store for all endpoints and auth
- `platform/cmd/server/main.go` - golang-migrate wiring, graceful shutdown, no Redis passed to Handler
- `platform/internal/crypto/crypto.go` - Added SignRequest and VerifyRequest for signed auth
- `platform/internal/crypto/crypto_test.go` - Tests for SignRequest/VerifyRequest
- `platform/internal/models/models.go` - Removed TokenHash field from Entity
- `platform/migrations/001_entities.up.sql` - Removed token_hash column and index

## Decisions Made
- **Replaced bearer token auth with Ed25519 signed request auth:** During the checkpoint, the user decided to switch from bearer token authentication to Ed25519 signed request authentication. Clients now sign each request with their private key (method + path + timestamp + body), and the auth middleware verifies the signature using the entity's public key. This eliminates token storage and is more aligned with the protocol's cryptographic identity model.
- **Removed token_hash from database schema:** With signed auth, there is no need to store token hashes. The entity's public key (already stored) serves as the authentication credential. GetEntityByPublicKey replaces GetEntityByTokenHash.
- **EntityStore interface for testability:** Defined an interface in api.go so HTTP tests can use a fake in-memory store instead of requiring PostgreSQL.

## Deviations from Plan

### Design Change (User Decision)

**1. Bearer token auth replaced with Ed25519 signed request auth**
- **Found during:** Task 3 checkpoint (user decision)
- **Issue:** Bearer token auth, while functional, was inconsistent with ATAP's cryptographic identity model. Every entity already has an Ed25519 keypair -- using it for auth is more natural.
- **Change:** Added SignRequest/VerifyRequest to crypto.go, rewrote auth middleware to verify signatures instead of looking up token hashes, removed token_hash from Entity model and migration, replaced GetEntityByTokenHash with GetEntityByPublicKey in store.
- **Files modified:** crypto.go, crypto_test.go, api.go, api_test.go, store.go, models.go, 001_entities.up.sql
- **Impact:** Simplifies the auth model. No tokens to manage or store. Aligns with protocol philosophy.

---

**Total deviations:** 1 design change (user-approved)
**Impact on plan:** Positive -- simplifies auth, removes token storage, aligns with protocol identity model.

## Issues Encountered
None -- all tasks completed successfully.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 1 Foundation complete: agents can register, be looked up, and authenticate via signed requests
- Ready for Phase 2: Signal Pipeline (inbox, SSE streaming, webhooks, channels)
- Store interface pattern established for continued testability
- Ed25519 signed auth middleware ready to protect all future endpoints

## Self-Check: PASSED

All 8 artifact files verified present. Both task commits (6f1febf, 177ac01) verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-03-11*
