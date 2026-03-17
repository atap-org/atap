---
phase: 04-credentials-and-mobile
plan: 02
subsystem: api
tags: [go, redis, didcomm, org-delegation, rate-limiting, fan-out]

requires:
  - phase: 03-approval-engine
    provides: approval models, DIDComm dispatch infrastructure, dispatchDIDCommMessage helper

provides:
  - store/org_delegates.go: GetOrgDelegates query — returns up to 50 members for an org DID
  - api/approvals.go: POST /v1/approvals handler with org fan-out + per-source rate limiting
  - OrgDelegateStore interface in api.go

affects:
  - any phase adding org management or delegation features
  - any handler that dispatches approvals or DIDComm to org entities

tech-stack:
  added: []
  patterns:
    - "Org fan-out: concurrent goroutine dispatch to all delegates via dispatchDIDCommMessageTo"
    - "Fan-out rate limiting: Redis INCR + conditional EXPIRE(NX) at fanout:rate:{src}:{org}"
    - "OrgDelegateStore interface follows same store abstraction pattern as EntityStore, MessageStore"
    - "TDD: RED (build-failing tests) → GREEN (implementation) → clean test run"

key-files:
  created:
    - platform/internal/store/org_delegates.go
    - platform/internal/store/org_delegates_test.go
    - platform/internal/api/approvals.go
    - platform/internal/api/approvals_test.go
  modified:
    - platform/internal/api/api.go
    - platform/cmd/server/main.go

key-decisions:
  - "Fan-out dispatched in a goroutine: CreateApproval returns 202 immediately; delegates receive messages asynchronously"
  - "Rate limit threshold 10/hr per (source_did, org_did) pair: Redis INCR returns new value; if > 10 → 429"
  - "Conditional EXPIRE: set TTL only when count == 1 (new key) to avoid resetting window mid-hour"
  - "dispatchDIDCommMessageTo added alongside existing dispatchDIDCommMessage for single-recipient dispatch"
  - "OrgDelegateStore nil-guarded in handler: nil store skips fan-out (backwards-compatible)"
  - "syncMockMessageStore (thread-safe) added in approvals_test.go for goroutine fan-out tests"
  - "Rate limit tests skip if Redis unavailable (best-effort Redis guard pattern)"

patterns-established:
  - "Fan-out pattern: look up org, check rate limit, dispatch goroutine per delegate, return 202"
  - "Rate limit pattern: INCR + expire-on-first used in checkFanOutRateLimit (reusable for other limits)"

requirements-completed:
  - MSG-06

duration: 10min
completed: 2026-03-16
---

# Phase 4 Plan 02: Org Delegate Fan-Out and Rate Limiting Summary

**Org approval fan-out: DIDComm dispatch to all org delegates (cap 50) with Redis INCR per-source rate limiting (10/hr/org)**

## Performance

- **Duration:** 10 min
- **Started:** 2026-03-16T13:48:24Z
- **Completed:** 2026-03-16T13:58:34Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- GetOrgDelegates store method returns up to 50 human/agent/machine entities with matching principal_did; server-side cap of 50 enforced regardless of caller input
- CreateApproval handler with org-type detection: fan-out dispatches DIDComm messages concurrently to all delegates via goroutine; non-org targets receive direct dispatch
- Per-source rate limiting using Redis INCR + conditional EXPIRE: key `fanout:rate:{sourceDID}:{orgDID}`, 1-hour TTL, 10 requests per hour limit; returns RFC 7807 429 when exceeded

## Task Commits

1. **Task 1: Org delegate query and fan-out dispatch** - `53ae12c` (feat)
2. **Task 2: Fan-out in CreateApproval + first-response-wins guard + per-source rate limiting** - `8667acb` (feat)

## Files Created/Modified

- `platform/internal/store/org_delegates.go` - GetOrgDelegates: SQL query with type filter + 50-cap
- `platform/internal/store/org_delegates_test.go` - Integration tests: empty, normal, excludes org type, 50-cap (skip without DATABASE_URL)
- `platform/internal/api/approvals.go` - CreateApproval handler with org fan-out, rate limit, DIDComm dispatch
- `platform/internal/api/approvals_test.go` - Unit tests: fan-out count, non-org no-fan-out, rate limit exceeded (429), below threshold (202)
- `platform/internal/api/api.go` - Added OrgDelegateStore interface; added orgDelegateStore field to Handler; updated NewHandler signature; registered POST /v1/approvals route
- `platform/cmd/server/main.go` - Updated NewHandler call to pass `db` for OrgDelegateStore

## Decisions Made

- Fan-out uses a goroutine (fire-and-forget): CreateApproval returns 202 immediately without waiting for all dispatches to complete. This matches the async DIDComm delivery model.
- Rate limit threshold of 10/hr uses INCR + EXPIRE(NX): only set TTL on first increment to preserve the sliding window. Subsequent increments don't reset the hour.
- OrgDelegateStore nil check in handler provides backwards compatibility for test handlers that don't set it.
- syncMockMessageStore (with mutex) created for goroutine-safe message counting in fan-out tests.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Pre-existing build failure in `platform/internal/credential/credential.go` (added in prior commit, missing trustbloc dependencies) causes `go build ./...` to fail. This is out of scope — internal packages build cleanly (`go build ./internal/api/... ./internal/store/...`).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Org delegate routing (MSG-06) is complete
- `store.Store` satisfies the new `OrgDelegateStore` interface via `GetOrgDelegates`
- Rate limiter pattern established in `checkFanOutRateLimit` can be reused for other per-source limits
- Pre-existing credential package build failure should be addressed in a subsequent plan

---
*Phase: 04-credentials-and-mobile*
*Completed: 2026-03-16*
