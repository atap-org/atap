---
phase: 05-api-hardening
plan: 02
subsystem: testing
tags: [rate-limiting, redis, fiber, middleware, go-test]

# Dependency graph
requires:
  - phase: 05-api-hardening plan 01
    provides: RateLimitMiddleware, rateLimitConfig, ipInAllowlist, fixed-window Redis counters

provides:
  - Comprehensive unit tests for rate limit middleware (10 test functions)
  - Tests covering: public threshold 429, auth threshold 429, within-limit 200+headers, header values, CIDR allowlist bypass, /v1/health exempt, /.well-known/atap.json exempt, Redis-down 503, IPv4-mapped normalization, ipInAllowlist helper
affects: [future middleware changes, ci-pipeline]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Use testClientIP constant to document Fiber app.Test IP (0.0.0.0 — no real TCP)"
    - "Pre-seed Redis with h.redis.Set before test, DEL first, cleanup with t.Cleanup"
    - "currentRateKey helper mirrors production key computation for test isolation"

key-files:
  created:
    - platform/internal/api/ratelimit_test.go
  modified:
    - platform/internal/api/api.go

key-decisions:
  - "Fiber app.Test uses 0.0.0.0 as client IP (no real TCP connection), so test fixtures must seed keys for that IP not 127.0.0.1"
  - "Pre-existing approval test failures (scope atap:approve vs atap:send) documented as deferred — confirmed not caused by this plan"

patterns-established:
  - "Rate limit test pattern: newRateLimitTestHandler creates minimal Fiber app with only rate limit middleware + dummy routes"

requirements-completed: [API-07]

# Metrics
duration: 7min
completed: 2026-03-18
---

# Phase 5 Plan 2: Rate Limit Middleware Tests Summary

**10-function test suite covering all API-07 behaviors: threshold enforcement for public (30 rpm) and auth (120 rpm) tiers, header injection, exempt paths, CIDR allowlist bypass, Redis-down 503 fail-closed, and ipInAllowlist unit tests**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-03-18T19:38:00Z
- **Completed:** 2026-03-18T19:42:40Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created `ratelimit_test.go` with 10 test functions (428 lines) covering every API-07 behavior
- All 10 rate limit tests pass with Redis available, RedisDown test passes without Redis
- Removed resolved Phase 4 TODO comment from `api.go` (rate limiting now globally applied)

## Task Commits

Each task was committed atomically:

1. **Task 1: Write comprehensive rate limit middleware tests** - `77c137e` (test)
2. **Task 2: Run full test suite and verify no regressions** - `a316cff` (chore)

## Files Created/Modified
- `platform/internal/api/ratelimit_test.go` - 10 test functions for rate limit middleware (428 lines)
- `platform/internal/api/api.go` - Removed resolved Phase 4 TODO comment for IP rate limiting

## Decisions Made
- Fiber's `app.Test()` uses `0.0.0.0` as the client IP (no real TCP), so Redis keys and allowlist CIDRs in tests must use `0.0.0.0` not `127.0.0.1`. Documented via `testClientIP` constant.
- Pre-existing approval test failures (9 tests failing with `403 atap:send scope`) confirmed pre-existing before this plan — logged to `deferred-items.md`, not fixed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test IP address from 127.0.0.1 to 0.0.0.0**
- **Found during:** Task 1 (first test run)
- **Issue:** Plan specified pre-seeding Redis keys for `127.0.0.1`, but Fiber's `app.Test()` reports `0.0.0.0` as client IP (no real TCP connection). Tests were seeding the wrong key.
- **Fix:** Added `testClientIP = "0.0.0.0"` constant and updated all key computations and allowlist CIDR to use that IP.
- **Files modified:** platform/internal/api/ratelimit_test.go
- **Verification:** All 10 tests pass
- **Committed in:** 77c137e (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in plan's assumed test IP)
**Impact on plan:** Fix necessary for tests to actually exercise the middleware. No scope creep.

## Issues Encountered
- Pre-existing approval test failures (`TestCreateApproval_OrgFanOut` etc.) failing with 403 `atap:send` scope before this plan. Logged to `deferred-items.md`. Not caused by rate limiting changes.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Rate limit middleware is fully tested and passing
- Pre-existing approval test failures should be addressed before final CI check
- Ready for next phase plan

---
*Phase: 05-api-hardening*
*Completed: 2026-03-18*
