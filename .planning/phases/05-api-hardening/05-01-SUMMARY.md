---
phase: 05-api-hardening
plan: 01
subsystem: api
tags: [rate-limiting, redis, middleware, fiber, postgresql, go]

# Dependency graph
requires: []
provides:
  - IP-based rate limiting middleware with Redis INCR fixed-window counters
  - rate_limit_config PostgreSQL table with DB-backed config cache
  - RateLimitConfigStore interface and GetRateLimitConfig store method
  - Tiered rate limits: public 30 rpm, authenticated 120 rpm
  - Background config refresh goroutine (60s polling)
affects: [06-observability]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Redis INCR fixed-window rate limiting with minute-granularity window keys
    - DB-backed config cache with sync.RWMutex and background refresh goroutine
    - Fail-closed middleware (503 on Redis unavailability)

key-files:
  created:
    - platform/migrations/015_rate_limit_config.up.sql
    - platform/migrations/015_rate_limit_config.down.sql
    - platform/internal/api/ratelimit.go
  modified:
    - platform/internal/api/api.go
    - platform/internal/store/store.go
    - platform/cmd/server/main.go

key-decisions:
  - "Fail closed on Redis unavailability (503) — protects backend from abuse even when Redis is down"
  - "Fixed-window counters with minute granularity (rl:ip:{group}:{ip}:{window}) — simple and predictable"
  - "Auth detection via Authorization or DPoP header presence — no token validation needed at rate limit layer"
  - "DB-backed config with 60s background refresh — allows live config changes without restart"

patterns-established:
  - "RateLimitMiddleware pattern: method on Handler accepting *rateLimitConfig, registered via app.Use in SetupRoutes"
  - "Background refresh goroutine pattern: context.WithCancel + defer cancel + ticker.Stop, matching existing revocation cleanup"

requirements-completed: [API-07]

# Metrics
duration: 3min
completed: 2026-03-18
---

# Phase 5 Plan 01: Rate Limiting Middleware Summary

**IP-based rate limiting via Redis INCR fixed-window counters with DB-backed config (public 30 rpm, auth 120 rpm), fail-closed on Redis failure**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-18T19:33:03Z
- **Completed:** 2026-03-18T19:36:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- PostgreSQL `rate_limit_config` table with seed data (migration 015) for live config changes
- Redis INCR fixed-window rate limiter with tiered limits (public 30 rpm / authenticated 120 rpm)
- Fail-closed 503 behavior when Redis unavailable, RFC 7807 429 on rate limit exceeded
- X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After headers on all rate-limited responses
- CIDR allowlist bypass and exempt paths (/v1/health, /.well-known/atap.json)
- Background config refresh goroutine (60s) wired into main.go with context cancellation

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration, store method, interface** - `fcf3d4e` (feat)
2. **Task 2: Rate limit middleware and wiring** - `fc47553` (feat)

## Files Created/Modified
- `platform/migrations/015_rate_limit_config.up.sql` - Rate limit config table with seed data
- `platform/migrations/015_rate_limit_config.down.sql` - Teardown migration
- `platform/internal/api/ratelimit.go` - rateLimitConfig struct, RateLimitMiddleware, StartRateLimitConfigRefresh, ipInAllowlist
- `platform/internal/api/api.go` - RateLimitConfigStore interface, rateLimitStore/rateLimitCfg fields on Handler, SetRateLimitConfig, middleware wired into SetupRoutes
- `platform/internal/store/store.go` - GetRateLimitConfig method
- `platform/cmd/server/main.go` - Rate limit config refresh goroutine and SetRateLimitConfig wiring

## Decisions Made
- Fail closed (503) on Redis unavailability — safer to deny requests than allow unbounded traffic when rate limiting is broken
- Fixed-window (not sliding) counters — simpler implementation, predictable behavior, TTL of 2x window avoids stale key accumulation
- Auth detection via header presence only — no token validation needed at the rate limit layer, keeps middleware fast
- DB-backed config cache with 60s refresh — supports live config changes without server restart

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created ratelimit.go as part of Task 1 to enable compilation**
- **Found during:** Task 1 (migration and interface)
- **Issue:** api.go now references `rateLimitConfig` type (from the `rateLimitCfg *rateLimitConfig` field and `SetRateLimitConfig` method added for Task 2), but the type is defined in ratelimit.go which is Task 2's artifact. The project would not compile after Task 1 alone.
- **Fix:** Created the full ratelimit.go as part of Task 1's execution so the type is available. Both tasks effectively committed together (but to separate commits), satisfying the plan's per-task commit requirement.
- **Files modified:** platform/internal/api/ratelimit.go
- **Verification:** `go build ./...` exits 0 after both commits
- **Committed in:** fc47553 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary — Go requires all referenced types to exist at compile time. The tasks had a forward dependency that required both to be implemented together.

## Issues Encountered
None beyond the compilation forward-reference handled above.

## User Setup Required
None - no external service configuration required beyond already-running Redis and PostgreSQL.

## Next Phase Readiness
- Rate limiting is active on all routes except /v1/health and /.well-known/atap.json
- Config is live-reloadable via the rate_limit_config table (UPDATE key SET value = '60' WHERE key = 'public_rpm')
- Ready for Phase 5 Plan 02 (next hardening task)

---
*Phase: 05-api-hardening*
*Completed: 2026-03-18*
