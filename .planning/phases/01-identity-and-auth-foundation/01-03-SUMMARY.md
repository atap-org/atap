---
phase: 01-identity-and-auth-foundation
plan: "03"
subsystem: api
tags: [discovery, rfc7807, problem-details, fiber, golang]

requires:
  - phase: 01-identity-and-auth-foundation
    plan: "01"
    provides: api.Handler struct, problem() helper, SetupRoutes, config.Config

provides:
  - GET /.well-known/atap.json — ATAP server discovery document
  - RFC 7807 application/problem+json Content-Type on all error responses
  - GlobalErrorHandler exported for main.go and test use

affects: [02-oauth-and-tokens, 03-claims, all API consumers]

tech-stack:
  added: []
  patterns:
    - "Server discovery at /.well-known/atap.json outside /v1/ group"
    - "RFC 7807: problem() uses Fiber's JSON ctype param to set application/problem+json"
    - "GlobalErrorHandler exported for main.go wire-up"

key-files:
  created:
    - platform/internal/api/discovery.go
    - platform/internal/api/discovery_test.go
    - platform/internal/api/errors_test.go
  modified:
    - platform/internal/api/api.go
    - platform/cmd/server/main.go

key-decisions:
  - "Use Fiber's c.JSON(data, ctype) overload for application/problem+json — c.Set() before JSON is overwritten"
  - "Export GlobalErrorHandler so main.go can wire it without coupling to unexported symbol"
  - "trust_level=1 and max_approval_ttl=86400 are informational in Phase 1 — enforcement deferred"

requirements-completed:
  - SRV-01
  - SRV-02
  - SRV-03
  - API-06

duration: 12min
completed: 2026-03-13
---

# Phase 1 Plan 03: Discovery Endpoint and RFC 7807 Error Standardization Summary

**ATAP server discovery via GET /.well-known/atap.json and RFC 7807 application/problem+json enforced on all error responses**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-13T17:30:01Z
- **Completed:** 2026-03-13T17:42:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Implemented `GET /.well-known/atap.json` returning domain, api_base, didcomm_endpoint, claim_types, max_approval_ttl, trust_level, and full oauth metadata
- Fixed `problem()` helper to set `Content-Type: application/problem+json` using Fiber's ctype parameter
- Exported `GlobalErrorHandler` and wired it into the Fiber app in main.go, replacing the inline handler that was missing the correct content type

## Task Commits

1. **Task 1: Server discovery endpoint** (TDD)
   - `2059e3a` test(01-03): add failing tests for discovery endpoint
   - `d939172` feat(01-03): implement server discovery endpoint /.well-known/atap.json

2. **Task 2: RFC 7807 error response standardization** (TDD)
   - `d2550a5` test(01-03): add failing tests for RFC 7807 error response standardization
   - `ff1b94d` feat(01-03): standardize RFC 7807 error responses with application/problem+json

## Files Created/Modified

- `platform/internal/api/discovery.go` - Discovery handler returning ATAP server capabilities
- `platform/internal/api/discovery_test.go` - Tests for discovery endpoint + shared `newTestHandlerWithStores` helper
- `platform/internal/api/errors_test.go` - Tests verifying RFC 7807 content type, URI format, and global error handler
- `platform/internal/api/api.go` - Updated `problem()` with ctype param, exported `GlobalErrorHandler`
- `platform/cmd/server/main.go` - Wired `api.GlobalErrorHandler` into Fiber config

## Decisions Made

- **Fiber ctype param:** `c.Set("Content-Type", ...)` before `c.JSON()` is silently overwritten by Fiber. The fix is `c.JSON(data, "application/problem+json")` — Fiber accepts an optional ctype parameter in its `JSON()` method.
- **Exported GlobalErrorHandler:** The error handler was unexported, requiring duplication in main.go. Exporting it as `GlobalErrorHandler` removes that duplication and ensures consistent RFC 7807 behavior across all unhandled errors.
- **trust_level and max_approval_ttl are informational in Phase 1:** Published as L1 (DV TLS) and 86400s respectively, enforcement deferred to Phase 3.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fiber's c.Set() does not override JSON content type**
- **Found during:** Task 2 (RFC 7807 error standardization)
- **Issue:** Plan specified `c.Set("Content-Type", "application/problem+json")` before `c.JSON()`, but Fiber always overwrites Content-Type when `c.JSON()` is called
- **Fix:** Used Fiber's built-in ctype parameter: `c.JSON(data, "application/problem+json")` which is set directly during body serialization
- **Files modified:** platform/internal/api/api.go
- **Verification:** TestProblemHelperContentType passes
- **Committed in:** ff1b94d

**2. [Rule 2 - Missing Critical] main.go had inline error handler missing application/problem+json**
- **Found during:** Task 2 (verifying GlobalErrorHandler)
- **Issue:** main.go used an inline Fiber ErrorHandler that returned `fiber.Map` with `application/json`, not `application/problem+json`
- **Fix:** Exported `GlobalErrorHandler` from api package, wired into main.go via `ErrorHandler: api.GlobalErrorHandler`
- **Files modified:** platform/internal/api/api.go, platform/cmd/server/main.go
- **Verification:** Build passes; TestGlobalErrorHandlerRFC7807 and TestUnknownRoute404IsRFC7807 pass
- **Committed in:** ff1b94d

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing critical)
**Impact on plan:** Both fixes necessary for correctness. No scope creep.

## Issues Encountered

- During Task 2, linter concurrently committed 01-02 plan work (entity handlers, DID resolution, key version store) which modified `api.go`, `discovery_test.go`, and created `entities.go`, `did.go`. This caused temporary build failures. The linter-committed code was correct and complementary; execution continued after confirming all tests passed.

## Next Phase Readiness

- Discovery endpoint complete, ready for OAuth/DPoP plan (01-04)
- All error responses use RFC 7807 with correct content type
- GlobalErrorHandler wired for consistent error behavior across all routes
- entity CRUD, DID resolution, key rotation all complete (from 01-02)

---
*Phase: 01-identity-and-auth-foundation*
*Completed: 2026-03-13*
