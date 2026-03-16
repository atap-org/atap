---
phase: 03-mobile-app
plan: 01
subsystem: database
tags: [postgres, migrations, claims, delegations, push-tokens]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: entities table and Store pattern
  - phase: 02-signal-pipeline
    provides: signals, channels, webhook tables
provides:
  - claims table with invite code flow (create, lookup by code, redeem)
  - delegations table with delegator/delegate trust chain
  - push_tokens table with upsert for notification delivery
  - Claim, Delegation, PushToken domain models
  - NewClaimID, NewDelegationID, GenerateClaimCode crypto functions
  - 9 new store methods (CreateClaim, GetClaimByCode, RedeemClaim, CreateDelegation, GetDelegationsByDelegator, GetDelegationsByDelegate, UpsertPushToken, GetPushToken, DeletePushToken)
affects: [03-mobile-app]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - scanClaim/scanDelegation private row-scanner helpers
    - ErrClaimNotAvailable sentinel error for redeem validation

key-files:
  created:
    - platform/migrations/005_claims.up.sql
    - platform/migrations/005_claims.down.sql
    - platform/migrations/006_delegations.up.sql
    - platform/migrations/006_delegations.down.sql
    - platform/migrations/007_push_tokens.up.sql
    - platform/migrations/007_push_tokens.down.sql
  modified:
    - platform/internal/models/models.go
    - platform/internal/crypto/crypto.go
    - platform/internal/store/store.go

key-decisions:
  - "ErrClaimNotAvailable sentinel error for RedeemClaim validation (not-found vs already-redeemed)"
  - "Push tokens use entity_id as primary key (one token per entity)"

patterns-established:
  - "scanClaim/scanDelegation private helpers following scanSignal/scanChannel pattern"
  - "Claim code format ATAP-XXXX with 36-char alphanumeric alphabet"

requirements-completed: [MOB-04]

# Metrics
duration: 2min
completed: 2026-03-11
---

# Phase 3 Plan 01: Mobile Data Layer Summary

**Claims, delegations, and push tokens data layer with 3 migrations, 3 model types, and 9 store methods**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-11T22:18:07Z
- **Completed:** 2026-03-11T22:19:41Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Three new PostgreSQL migrations (005-007) for claims, delegations, and push_tokens tables
- Claim, Delegation, PushToken domain models plus API request/response types
- Nine store methods covering full CRUD for claims, delegations, and push tokens
- NewClaimID, NewDelegationID, GenerateClaimCode crypto ID generators

## Task Commits

Each task was committed atomically:

1. **Task 1: Create migrations and extend models** - `0ed7dbc` (feat)
2. **Task 2: Implement store methods** - `ef944ac` (feat)

## Files Created/Modified
- `platform/migrations/005_claims.up.sql` - Claims table with code, creator, status, timestamps
- `platform/migrations/005_claims.down.sql` - Drop claims table
- `platform/migrations/006_delegations.up.sql` - Delegations table with delegator/delegate, scope, unique constraint
- `platform/migrations/006_delegations.down.sql` - Drop delegations table
- `platform/migrations/007_push_tokens.up.sql` - Push tokens table with entity_id PK, platform check
- `platform/migrations/007_push_tokens.down.sql` - Drop push_tokens table
- `platform/internal/models/models.go` - Added Claim, Delegation, PushToken structs and request/response types
- `platform/internal/crypto/crypto.go` - Added NewClaimID, NewDelegationID, GenerateClaimCode
- `platform/internal/store/store.go` - Added 9 store methods with scanClaim/scanDelegation helpers

## Decisions Made
- ErrClaimNotAvailable sentinel error for RedeemClaim (distinguishes "not found" from "already redeemed" at the DB level by checking rows affected)
- Push tokens use entity_id as primary key (one active token per entity, upsert pattern)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Data layer complete for claims, delegations, and push tokens
- Ready for API endpoint implementation in Plan 03-02/03-03
- Migrations need to be applied to any running database before API endpoints can be used

---
*Phase: 03-mobile-app*
*Completed: 2026-03-11*
