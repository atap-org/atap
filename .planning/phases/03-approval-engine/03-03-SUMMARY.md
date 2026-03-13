---
phase: 03-approval-engine
plan: 03
subsystem: api
tags: [approval, jws, ed25519, didcomm, fiber, go, two-party, three-party, appr-12, appr-09, appr-08]

requires:
  - phase: 03-01
    provides: Approval data model, PostgreSQL store, migrations
  - phase: 03-02
    provides: JWS signing/verification (SignApproval, VerifyApprovalSignature), lifecycle state machine (ValidateTransition, ClampValidUntil), template fetch

provides:
  - 6 HTTP endpoints under /v1/approvals (POST create, POST respond, GET by id, GET status, GET list, DELETE revoke)
  - Full two-party and three-party approval flows end-to-end
  - APR-08: via rejection with TypeApprovalRejected DIDComm dispatch and 422 response
  - APR-09: atomic one-time approval consumption on first status check
  - APR-12: kid validation in JWS header against entity DID URL
  - Background expiry cleanup goroutine in main.go
  - 8 integration tests covering all spec requirements

affects: [04-human-claims, 05-delegations, mobile-app]

tech-stack:
  added: []
  patterns:
    - "Client-generated approval IDs: client generates apr_ ULID before signing, includes in create request body"
    - "Time precision for JWS: use time.Truncate(time.Second) when valid_until must survive RFC3339 round-trip"
    - "Public route before auth group: register v1.Get public routes BEFORE auth group to prevent DPoP middleware interception"
    - "DIDComm dispatch via QueueMessage: plaintext DIDComm messages serialized to JSON and queued to messageStore"
    - "mockMessageStoreWithCapture: captures dispatched messages for DIDComm assertion in tests"

key-files:
  created:
    - platform/internal/api/approvals.go
    - platform/internal/api/approvals_test.go
  modified:
    - platform/internal/api/api.go
    - platform/cmd/server/main.go

key-decisions:
  - "Client-generated approval ID: client includes id and created_at in POST /v1/approvals so server can verify from_signature against a known document (avoids chicken-and-egg with server-assigned IDs)"
  - "Public status route registered before auth group: Fiber v2 group middleware can intercept all paths in namespace; status route must precede auth.Group() call"
  - "DIDComm dispatch via messageStore.QueueMessage: no Mediator struct exists; plaintext approval messages are JSON-serialized and queued directly"
  - "Via validation fires after from_signature verification: signature check must pass even for requests that will be rejected by via policy"
  - "time.Truncate(time.Second) for valid_until in tests: RFC3339 has second precision; nanosecond mismatch causes JWS canonical JSON difference"

patterns-established:
  - "approvalResponse helper: builds API response map including server-side state fields excluded from JWS signing scope"
  - "dispatchApprovalMessage: serializes PlaintextMessage to JSON, queues per-recipient via messageStore"

requirements-completed: [APR-01, APR-02, API-03, APR-08, APR-09, APR-10, APR-11, APR-12]

duration: 14min
completed: 2026-03-13
---

# Phase 03 Plan 03: Approval HTTP API Integration Summary

**6-endpoint approval REST API with two-party + three-party flows, APR-08 via rejection, APR-09 atomic one-time consumption, APR-12 kid validation, and 8 integration tests**

## Performance

- **Duration:** 14 min
- **Started:** 2026-03-13T21:33:30Z
- **Completed:** 2026-03-13T21:46:20Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Implemented all 6 approval endpoints per spec Section 13.3 with full DPoP authentication
- Three-party via rejection (APR-08) dispatches TypeApprovalRejected DIDComm message with reason codes and returns 422
- One-time approval atomic consumption (APR-09): first status check transitions to consumed, second returns valid=false
- Signature kid header validation (APR-12): kid extracted from JWS header validated against entity DID URL before cryptographic check
- Background expiry cleanup goroutine in main.go on 5-minute ticker
- 8 integration tests covering complete spec surface area with all approvals passing

## Task Commits

1. **Task 1: ApprovalStore interface + Handler wiring + all 6 API handlers** - `e0efcbf` (feat)
2. **Task 2: main.go wiring + expiry cleanup + integration tests** - `7dab961` (test)

## Files Created/Modified

- `platform/internal/api/api.go` - Added ApprovalStore interface, approvalStore field in Handler, NewHandler signature, approval routes in SetupRoutes
- `platform/internal/api/approvals.go` - All 6 approval handlers + DIDComm dispatch helpers + approvalResponse helper
- `platform/internal/api/approvals_test.go` - 8 integration tests with mockApprovalStore and mockMessageStoreWithCapture
- `platform/cmd/server/main.go` - Wire approvalStore into NewHandler, add expiry cleanup goroutine

## Decisions Made

- **Client-generated approval IDs:** Client generates `apr_` ULID and includes `id` + `created_at` in POST /v1/approvals body. This allows server to reconstruct the exact document the client signed for `from_signature` verification. Without this, server-assigned IDs would create a chicken-and-egg problem with pre-signing.
- **Public status route before auth group:** Fiber v2 group middleware can intercept all paths under the same namespace prefix. `v1.Get("/approvals/:approvalId/status")` must be registered BEFORE `auth := v1.Group("", h.DPoPAuthMiddleware())` to prevent the DPoP middleware from firing on the public status endpoint.
- **DIDComm dispatch via QueueMessage:** No Mediator struct with RouteMessage exists in this codebase. Plaintext DIDComm approval messages are JSON-serialized and queued directly via `messageStore.QueueMessage`.
- **Via validation after from_signature verification:** The from signature check runs even for requests that will be rejected by via policy. This ensures the from entity authenticated the request even when the via system rejects it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Client-generated approval IDs required for from_signature verification**
- **Found during:** Task 2 (Integration tests)
- **Issue:** Plan specified server-assigned IDs (`crypto.NewApprovalID()`) but also from_signature verification. The client signs a document including the ID, but if the server generates the ID, the signed document differs from the server-constructed document — cryptographic verification always fails.
- **Fix:** Updated CreateApproval to accept optional `id` and `created_at` fields in the request body. When provided, these are used to construct the approval document for signature verification. This aligns with standard DID-based approval protocols where the requester pre-generates the document they sign.
- **Files modified:** platform/internal/api/approvals.go, platform/internal/api/approvals_test.go
- **Verification:** All 8 integration tests pass including TestSignatureKIDValidation
- **Committed in:** 7dab961 (Task 2 commit)

**2. [Rule 1 - Bug] Public status route intercepted by DPoP middleware**
- **Found during:** Task 2 (Integration tests)
- **Issue:** `v1.Get("/approvals/:approvalId/status")` registered after the auth group caused Fiber v2 to apply DPoP middleware to the status route, returning 401 for public requests.
- **Fix:** Moved status route registration to BEFORE `auth := v1.Group("", h.DPoPAuthMiddleware())`.
- **Files modified:** platform/internal/api/api.go
- **Verification:** Status endpoint returns 200 without auth headers
- **Committed in:** 7dab961 (Task 2 commit)

**3. [Rule 1 - Bug] time.Time precision mismatch for valid_until JWS signing**
- **Found during:** Task 2 (TestRevokeWithChildren)
- **Issue:** Client signs `valid_until` with nanosecond precision; RFC3339 string in JSON request body has second precision. After server parses the body, the reconstructed time differs in nanoseconds, causing canonical JSON mismatch and signature verification failure.
- **Fix:** Tests use `time.Truncate(time.Second)` for times that appear in approval documents with valid_until. Production clients should also truncate to second precision before signing.
- **Files modified:** platform/internal/api/approvals_test.go
- **Verification:** TestRevokeWithChildren passes
- **Committed in:** 7dab961 (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (3 bugs)
**Impact on plan:** All fixes necessary for correctness. The client-generated ID approach is the correct design for pre-signing approval documents in DID protocols. No scope creep.

## Issues Encountered

None beyond the deviations documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Approval engine complete end-to-end: two-party, three-party, one-time, persistent, revocation, expiry
- Phase 03 complete — ready for Phase 04 human claims and delegations
- Note for production: clients must generate approval IDs before signing; `time.Truncate(time.Second)` recommended for valid_until fields

---
*Phase: 03-approval-engine*
*Completed: 2026-03-13*
