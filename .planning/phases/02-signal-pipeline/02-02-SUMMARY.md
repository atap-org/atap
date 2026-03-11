---
phase: 02-signal-pipeline
plan: 02
subsystem: api
tags: [fiber, redis, sse, ed25519, signals, inbox, pagination, pub-sub, streaming]

# Dependency graph
requires:
  - phase: 02-signal-pipeline
    plan: 01
    provides: Signal/Channel/Webhook models, store methods (SaveSignal, GetInbox, GetSignalsAfter), ErrDuplicateSignal, crypto.NewSignalID
provides:
  - SendSignal handler (POST /v1/inbox/:entityId) with Ed25519 signature verification and write-then-notify
  - GetInbox handler (GET /v1/inbox/:entityId) with cursor-based pagination
  - InboxStream handler (GET /v1/inbox/:entityId/stream) with Redis pub/sub SSE, replay, and heartbeat
  - SignalStore, ChannelStore, WebhookStore interfaces on Handler
  - Platform signing key generation in main.go
  - Unit tests for signal sending, inbox polling, pagination, auth, and idempotency
affects: [02-03-channels-webhooks, 02-04-integration-tests]

# Tech tracking
tech-stack:
  added: []
  patterns: [write-then-notify (PG save then Redis publish), subscribe-first replay for SSE gap avoidance, fasthttp StreamWriter for SSE, signedRequest path-only signing in tests]

key-files:
  created: []
  modified:
    - platform/internal/api/api.go
    - platform/internal/api/api_test.go
    - platform/internal/models/models.go
    - platform/cmd/server/main.go

key-decisions:
  - "SignalStore interface decouples API layer from PostgreSQL store implementation"
  - "SSE subscribes to Redis before PostgreSQL replay to eliminate replay gap"
  - "Nil Redis client handled gracefully (skip Publish) for unit tests without Redis"
  - "signedRequest test helper strips query params to match Fiber c.Path() behavior"

patterns-established:
  - "Write-then-notify: SaveSignal to PG, then Publish to Redis -- guarantees persistence before delivery"
  - "Subscribe-first SSE: Redis subscribe before DB replay avoids the gap where signals are missed"
  - "Signal signature verification: client signs canonical(route).canonical(signal), server verifies against entity public key"
  - "Own-inbox enforcement: authenticated entity ID must match URL entityId param"

requirements-completed: [SIG-01, SIG-04, SSE-01, SSE-02, SSE-03, SSE-04]

# Metrics
duration: 6min
completed: 2026-03-11
---

# Phase 2 Plan 02: Signal API and SSE Streaming Summary

**SendSignal with Ed25519 verification and write-then-notify, GetInbox with cursor pagination, InboxStream with Redis pub/sub SSE and subscribe-first replay**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-11T20:17:17Z
- **Completed:** 2026-03-11T20:23:23Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- POST /v1/inbox/:entityId sends signals with Ed25519 signature verification, write-then-notify pattern (PG then Redis), returns 202
- GET /v1/inbox/:entityId returns paginated inbox with cursor/hasMore, enforces own-inbox access
- GET /v1/inbox/:entityId/stream delivers real-time SSE via Redis pub/sub with subscribe-first replay and 30s heartbeat
- 10 new unit tests covering signal sending, inbox polling, pagination, auth, idempotency, and access control

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend Handler with Redis, implement SendSignal/GetInbox/InboxStream** - `6ad4a48` (feat)
2. **Task 2: Unit tests for signal sending and inbox polling** - `f6022e9` (test)

## Files Created/Modified
- `platform/internal/api/api.go` - Added SignalStore/ChannelStore/WebhookStore interfaces, updated Handler struct, implemented SendSignal, GetInbox, InboxStream handlers
- `platform/internal/api/api_test.go` - Extended fakeStore for all store interfaces, added 10 signal/inbox tests
- `platform/internal/models/models.go` - Added SendSignalRequest type
- `platform/cmd/server/main.go` - Platform key generation, BodyLimit config, pass Redis/key/stores to Handler

## Decisions Made
- SSE subscribes to Redis channel before replaying from PostgreSQL to prevent the replay gap (signals arriving between DB read and subscribe start)
- Nil Redis client is handled gracefully in SendSignal (skips Publish) to allow unit testing without Redis
- Test helper `signedRequest` strips query parameters from the signing path to match Fiber's `c.Path()` behavior
- Default priority set to "normal" if client doesn't specify

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added WebhookStore interface for webhook.go compilation**
- **Found during:** Task 1 (build verification)
- **Issue:** Existing webhook.go referenced undefined `WebhookStore` type causing build failure
- **Fix:** Added WebhookStore interface definition to api.go
- **Files modified:** platform/internal/api/api.go
- **Verification:** `go build ./...` passes
- **Committed in:** 6ad4a48 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed signedRequest test helper for query string URLs**
- **Found during:** Task 2 (TestGetInbox_Pagination failing)
- **Issue:** signedRequest was signing the full URL including query params, but Fiber's c.Path() returns path-only, causing signature mismatch on paginated requests
- **Fix:** Strip query string before computing signature in test helper
- **Files modified:** platform/internal/api/api_test.go
- **Verification:** TestGetInbox_Pagination passes
- **Committed in:** f6022e9 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes necessary for compilation and test correctness. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Signal API fully operational for Plans 03 (channels/webhooks) and 04 (integration tests)
- SSE streaming ready for end-to-end testing with live Redis
- Write-then-notify pattern established for all future signal delivery paths

## Self-Check: PASSED

All 4 files verified present. All 2 task commits verified in git log.

---
*Phase: 02-signal-pipeline*
*Completed: 2026-03-11*
