---
phase: 01-foundation
plan: 01
subsystem: infra
tags: [go, ed25519, jcs, docker, postgres, migrations, crypto]

# Dependency graph
requires: []
provides:
  - Go 1.24 module with all Phase 1 dependencies
  - Ed25519 crypto primitives with JCS canonical JSON
  - Entity, RegisterResponse, EntityLookupResponse domain models
  - Entities-only PostgreSQL migration (001)
  - Multi-stage Docker build with correct binary name
  - Phase 1 config (no SMSVerify/WorldID/FCM)
affects: [01-02, 01-03]

# Tech tracking
tech-stack:
  added: [gowebpki/jcs, golang-migrate/migrate/v4]
  patterns: [JCS canonical JSON for signing, TDD for crypto module]

key-files:
  created:
    - platform/migrations/001_entities.up.sql
    - platform/migrations/001_entities.down.sql
    - platform/internal/crypto/crypto_test.go
  modified:
    - platform/go.mod
    - platform/go.sum
    - platform/Dockerfile
    - docker-compose.yml
    - platform/internal/config/config.go
    - platform/internal/models/models.go
    - platform/internal/crypto/crypto.go
    - platform/internal/store/store.go
    - platform/internal/api/api.go
    - platform/cmd/server/main.go

key-decisions:
  - "Trimmed store.go and api.go to Phase 1 scope (entities only) to ensure go build succeeds"
  - "RegisterResponse includes PrivateKey field per CONTEXT.md locked decision"
  - "GetEntity returns EntityLookupResponse (public view) instead of full Entity"

patterns-established:
  - "JCS via gowebpki/jcs.Transform for all canonical JSON (not json.Marshal)"
  - "128-bit entropy for channel IDs (chn_ + 32 hex chars)"
  - "TDD for crypto: write failing tests first, then implement"

requirements-completed: [CRY-01, CRY-02, CRY-03, CRY-04, INF-01, INF-02, INF-03, INF-06, TST-03, TST-04, REG-02, REG-03, REG-05]

# Metrics
duration: 5min
completed: 2026-03-11
---

# Phase 1 Plan 01: Infrastructure & Crypto Summary

**Go 1.24 module with JCS canonical JSON, 128-bit channel IDs, entities-only migration, and 14 passing crypto tests**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-11T15:36:25Z
- **Completed:** 2026-03-11T15:41:16Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- Go module updated to 1.24 with gowebpki/jcs and golang-migrate dependencies
- Crypto module: RFC 8785 JCS canonical JSON, 128-bit channel IDs, EncodePrivateKey, 14 unit tests
- Models/config/store/api trimmed to Phase 1 scope (entities only)
- Entities-only migration with type and token_hash indexes
- Dockerfile fixed (binary name atap-platform, includes migrations)
- docker-compose.yml cleaned up (no version key, no dev volumes, no initdb mount)

## Task Commits

Each task was committed atomically:

1. **Task 1: Update Go module, dependencies, Dockerfile, Docker Compose, migration, config, and models** - `b7bf9c3` (feat)
2. **Task 2 RED: Failing crypto tests** - `0554117` (test)
3. **Task 2 GREEN: JCS canonical JSON, channel entropy, EncodePrivateKey** - `dc39da6` (feat)

## Files Created/Modified
- `platform/go.mod` - Go 1.24 with gowebpki/jcs, golang-migrate deps
- `platform/go.sum` - Dependency checksums
- `platform/Dockerfile` - Multi-stage Alpine build, correct binary name, includes migrations
- `docker-compose.yml` - Cleaned up: no version key, no dev volume, no initdb mount
- `platform/internal/config/config.go` - Phase 1 fields only + MigrationsPath
- `platform/internal/models/models.go` - Entity, RegisterResponse (with PrivateKey), EntityLookupResponse, ProblemDetail
- `platform/internal/crypto/crypto.go` - JCS canonical JSON, 128-bit channel IDs, EncodePrivateKey
- `platform/internal/crypto/crypto_test.go` - 14 unit tests covering all crypto functions
- `platform/internal/store/store.go` - Entities-only CRUD
- `platform/internal/api/api.go` - Health, register, get-entity endpoints only
- `platform/cmd/server/main.go` - Fixed go-redis v9.18 API change
- `platform/migrations/001_entities.up.sql` - Entities table with indexes
- `platform/migrations/001_entities.down.sql` - Drop entities table

## Decisions Made
- Trimmed store.go and api.go to Phase 1 scope to ensure `go build ./...` succeeds (plan expected only warnings but types were fully removed from models)
- RegisterResponse includes PrivateKey field per CONTEXT.md locked decision
- GetEntity now returns EntityLookupResponse (public view) instead of raw Entity struct

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Trimmed store.go and api.go to Phase 1**
- **Found during:** Task 1 (models trimming)
- **Issue:** Removing Signal, Channel, Delegation types from models.go caused compile errors in store.go and api.go (plan expected these to be "warnings" but they were actual undefined type errors)
- **Fix:** Trimmed store.go to entities-only CRUD and api.go to health/register/get-entity. Signal/channel/delegation store methods and API handlers will be re-added in Plan 02.
- **Files modified:** platform/internal/store/store.go, platform/internal/api/api.go
- **Verification:** `go build ./...` succeeds
- **Committed in:** b7bf9c3 (Task 1 commit)

**2. [Rule 3 - Blocking] Fixed go-redis v9.18 API change**
- **Found during:** Task 2 (full build verification)
- **Issue:** `rdb.Options().Context` removed in go-redis v9.18.0, breaking Ping call in main.go
- **Fix:** Changed to `context.Background()` and added "context" import
- **Files modified:** platform/cmd/server/main.go
- **Verification:** `go build ./...` succeeds
- **Committed in:** dc39da6 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both blocking)
**Impact on plan:** Both fixes necessary for compilation. Store/API trimming is consistent with Phase 1 scope. go-redis fix is a standard dependency upgrade issue.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All crypto primitives ready for store and API layers (Plan 02)
- Entities migration ready for database setup
- Models define the exact shapes for registration and lookup
- Dockerfile and docker-compose ready for containerized testing

## Self-Check: PASSED

All 10 artifact files verified present. All 3 task commits verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-03-11*
