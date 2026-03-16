---
phase: 04-fix-signal-pipeline-bugs
plan: 01
subsystem: api, mobile
tags: [webhook, retry, claim, flutter, pagination, signal, json]

# Dependency graph
requires:
  - phase: 02-signal-pipeline
    provides: WebhookWorker, SignalStore, signal delivery pipeline
  - phase: 03-mobile-app
    provides: Flutter Signal model, InboxNotifier pagination
provides:
  - WebhookWorker with SignalStore field for payload-fetching retries
  - Claim redemption 409 sentinel error handling
  - Flutter Signal.fromJson/toJson using ts field matching Go API
  - Flutter pagination using after= query param matching Go backend
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [sentinel-error-409, signal-store-injection-for-retry]

key-files:
  created:
    - mobile/test/signal_model_test.dart
    - mobile/test/inbox_provider_test.dart
  modified:
    - platform/internal/api/webhook.go
    - platform/internal/api/human.go
    - platform/internal/api/api_test.go
    - platform/cmd/server/main.go
    - platform/test/integration_test.go
    - mobile/lib/core/models/signal.dart
    - mobile/lib/providers/inbox_provider.dart

key-decisions:
  - "WebhookWorker accepts SignalStore as second parameter; db satisfies both interfaces at call sites"
  - "pollRetries skips silently (with warning log) when signal is missing rather than failing the entire retry batch"

patterns-established:
  - "Sentinel error check pattern: errors.Is(err, store.ErrXxx) -> 409 before generic 500"

requirements-completed: [SIG-04, SSE-01, WHK-03]

# Metrics
duration: 7min
completed: 2026-03-12
---

# Phase 04 Plan 01: Fix Signal Pipeline Bugs Summary

**Four cross-language integration bug fixes: webhook retry payload fetch, claim 409 race condition, Flutter timestamp field mismatch, and pagination query parameter mismatch**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-12T07:32:06Z
- **Completed:** 2026-03-12T07:39:10Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- WebhookWorker now fetches signal payload from SignalStore before re-enqueuing retries, fixing empty payload delivery
- Claim redemption returns 409 Conflict with sentinel error check instead of 500 Internal Server Error on race conditions
- Flutter Signal.fromJson reads `ts` field matching Go API JSON output instead of `created_at`
- Flutter loadMore sends `?after=` parameter matching Go backend `c.Query("after")` instead of `?cursor=`

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix Go backend bugs -- webhook retry payload and claim 409** - `1202fdf` (fix)
2. **Task 2: Fix Flutter bugs -- Signal.fromJson timestamp and pagination param** - `0dded9e` (fix)

## Files Created/Modified
- `platform/internal/api/webhook.go` - Added signalStore field, updated NewWebhookWorker signature, pollRetries fetches payload
- `platform/internal/api/human.go` - Added errors.Is sentinel check for ErrClaimNotAvailable returning 409
- `platform/internal/api/api_test.go` - Added TestPollRetriesFetchesPayload, TestPollRetriesSkipsMissingSignal, TestClaimRedemption409
- `platform/cmd/server/main.go` - Updated NewWebhookWorker call to pass db as SignalStore
- `platform/test/integration_test.go` - Updated NewWebhookWorker call to pass db as SignalStore
- `mobile/lib/core/models/signal.dart` - Changed fromJson/toJson to use 'ts' field
- `mobile/lib/providers/inbox_provider.dart` - Changed loadMore to use after= instead of cursor=
- `mobile/test/signal_model_test.dart` - Tests for Signal.fromJson ts parsing and toJson ts emission
- `mobile/test/inbox_provider_test.dart` - Tests for after= query parameter contract

## Decisions Made
- WebhookWorker accepts SignalStore as second parameter; db satisfies both interfaces at call sites
- pollRetries skips silently (with warning log) when signal is missing rather than failing the entire retry batch

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All four signal pipeline bugs are fixed with targeted tests
- Go platform builds cleanly and all API tests pass (28 tests)
- All Flutter tests pass (15 tests including 5 new ones)

## Self-Check: PASSED

All 9 files verified present. Both task commits (1202fdf, 0dded9e) verified in git log.

---
*Phase: 04-fix-signal-pipeline-bugs*
*Completed: 2026-03-12*
