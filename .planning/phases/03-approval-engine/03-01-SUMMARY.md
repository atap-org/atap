---
phase: 03-approval-engine
plan: 01
subsystem: database
tags: [approvals, models, postgres, pgx, ulid, jsonb, recursive-cte]

requires:
  - phase: 02-signal-pipeline
    provides: DIDComm message types (TypeApprovalRequest etc.) already defined; pgx store pattern established

provides:
  - Approval, ApprovalSubject, ApprovalResponse types in models.go (spec §8.5-8.7, §8.11)
  - Template, TemplateBrand, TemplateColors, TemplateDisplay, TemplateField, TemplateProof types (spec §11.2)
  - Approval state constants (requested/approved/declined/expired/rejected/consumed/revoked)
  - NewApprovalID() in crypto.go generating apr_ + lowercase ULID
  - Migration 011: approvals table with 5 indexes
  - Store CRUD: CreateApproval, GetApproval, UpdateApprovalState, ConsumeApproval, ListApprovals
  - Store: GetChildApprovals (recursive CTE), RevokeWithChildren, CleanupExpiredApprovals

affects:
  - 03-02 (JWS signing engine — uses Approval types and store)
  - 03-03 (API handlers — uses all store methods)

tech-stack:
  added: []
  patterns:
    - "Approval document stored as JSONB; server-side fields (state, responded_at, updated_at) in dedicated indexed columns"
    - "json:\"-\" on server-side Approval fields excludes them from JCS/JWS signing scope"
    - "ConsumeApproval uses atomic WHERE state='approved' AND valid_until IS NULL UPDATE to prevent double-consume"
    - "RevokeWithChildren uses recursive CTE (WITH RECURSIVE) for full N-level descendant cascade"

key-files:
  created:
    - platform/migrations/011_approvals.up.sql
    - platform/migrations/011_approvals.down.sql
    - platform/internal/store/approvals.go
    - platform/internal/store/approvals_test.go
  modified:
    - platform/internal/models/models.go
    - platform/internal/crypto/crypto.go

key-decisions:
  - "Approval state kept in a dedicated column (not buried in JSONB) to enable index-backed queries for state+did combos"
  - "Server-side fields (State, RespondedAt, UpdatedAt) use json:\"-\" — excluded from signed document JSONB, overlaid from columns on read"
  - "ConsumeApproval WHERE valid_until IS NULL is the authoritative one-time check — no application-level locking needed"

patterns-established:
  - "Approval JSONB document pattern: marshal struct (json:\"-\" fields excluded) into document column; unmarshal + overlay server fields on read"
  - "Recursive CTE pattern for parent-child approval chains: WITH RECURSIVE descendants AS (...)"

requirements-completed: [APR-03, APR-04, APR-09, APR-10, APR-11]

duration: 9min
completed: 2026-03-13
---

# Phase 3 Plan 01: Approval Engine Data Foundation Summary

**Approval data model, PostgreSQL schema (migration 011), and full CRUD store for multi-signature approval documents — with atomic one-time consumption and recursive parent-chain revocation**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-13T20:44:08Z
- **Completed:** 2026-03-13T20:53:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Approval, ApprovalSubject, ApprovalResponse, and all Template types added to models.go with spec-compliant JSON tags; server-side fields excluded from signing via json:"-"
- Migration 011 creates the approvals table with 5 targeted indexes (from+state, to+state, via sparse, parent sparse, expires sparse)
- Store layer implements 8 methods including ConsumeApproval (atomic WHERE valid_until IS NULL single-row UPDATE) and RevokeWithChildren (recursive CTE cascade)

## Task Commits

1. **Task 1: Approval types in models.go + NewApprovalID in crypto.go** - `6685cae` (feat)
2. **Task 2: Migration 011 + store/approvals.go + tests** - `d4e1520` (feat)

## Files Created/Modified

- `platform/internal/models/models.go` - Added Approval, ApprovalSubject, ApprovalResponse, Template* types + state constants
- `platform/internal/crypto/crypto.go` - Added NewApprovalID() generating apr_ + lowercase ULID
- `platform/migrations/011_approvals.up.sql` - approvals table with 5 indexes
- `platform/migrations/011_approvals.down.sql` - DROP TABLE IF EXISTS approvals
- `platform/internal/store/approvals.go` - Full CRUD: CreateApproval, GetApproval, UpdateApprovalState, ConsumeApproval, ListApprovals, GetChildApprovals, RevokeWithChildren, CleanupExpiredApprovals
- `platform/internal/store/approvals_test.go` - 16 subtests using in-memory mock store pattern (consistent with messages_test.go)

## Decisions Made

- Approval state kept in dedicated indexed column (not in JSONB) to enable efficient state+DID compound queries. The JSONB document field stores the signed document for retrieval; state is a server concern.
- Server-side fields (State, RespondedAt, UpdatedAt) use `json:"-"` so `json.Marshal(approval)` naturally produces the correct signing payload without needing a separate struct copy.
- ConsumeApproval relies on atomic `WHERE state='approved' AND valid_until IS NULL` — if 0 rows affected, the approval was already consumed or is persistent. No application-level mutex needed.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02 (JWS signing engine) can now import `models.Approval` and `crypto.NewApprovalID` directly
- Plan 03 (API handlers) can use all 8 store methods
- Migration 011 must be applied before integration tests that hit the real DB
- `CleanupExpiredApprovals` should be wired to a background timer in `cmd/server/main.go` (same pattern as `CleanupExpiredMessages`)

## Self-Check: PASSED

All created files confirmed on disk. All task commits (6685cae, d4e1520) confirmed in git history.

---
*Phase: 03-approval-engine*
*Completed: 2026-03-13*
