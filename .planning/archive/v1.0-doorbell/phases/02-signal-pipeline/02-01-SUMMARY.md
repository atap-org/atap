---
phase: 02-signal-pipeline
plan: 01
subsystem: database, api
tags: [postgres, pgx, signals, channels, webhooks, ulid, jsonb, cursor-pagination]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Entity model, Store with pgx pool, crypto ID generators, migrations/001_entities
provides:
  - Signal, SignalRoute, SignalTrust, SignalBody, SignalContext types
  - Channel, WebhookConfig, DeliveryAttempt types
  - NewSignalID() and NewDeliveryAttemptID() crypto helpers
  - Store methods for signal CRUD, inbox pagination, channel CRUD, webhook config, delivery tracking
  - Database migrations 002-004 (signals, channels, webhook_configs, delivery_attempts tables)
  - ErrDuplicateSignal sentinel for idempotency dedup
affects: [02-02-signal-api, 02-03-channel-api, 02-04-integration-tests]

# Tech tracking
tech-stack:
  added: []
  patterns: [scanSignal/scanChannel row scanner helpers, cursor-based pagination with hasMore, idempotency via ON CONFLICT DO NOTHING with sentinel error, JSONB for tags and signal data]

key-files:
  created:
    - platform/migrations/002_signals.up.sql
    - platform/migrations/002_signals.down.sql
    - platform/migrations/003_channels.up.sql
    - platform/migrations/003_channels.down.sql
    - platform/migrations/004_webhook_delivery.up.sql
    - platform/migrations/004_webhook_delivery.down.sql
  modified:
    - platform/internal/models/models.go
    - platform/internal/crypto/crypto.go
    - platform/internal/store/store.go

key-decisions:
  - "scanSignal/scanChannel private helpers to avoid repeating long column scan lists"
  - "GetSignalsAfter capped at 1000 rows for SSE replay memory safety"
  - "Channel tags and signal context.tags stored as JSONB arrays"

patterns-established:
  - "Row scanner pattern: private scanX(pgx.Row) functions for reuse across queries"
  - "Cursor pagination: fetch limit+1, slice to limit, set hasMore from overflow"
  - "Idempotency: ON CONFLICT DO NOTHING + check RowsAffected for sentinel error"

requirements-completed: [SIG-02, SIG-03, SIG-05, SIG-06, CHN-05]

# Metrics
duration: 3min
completed: 2026-03-11
---

# Phase 2 Plan 01: Data Models and Store Methods Summary

**Signal/Channel/Webhook data types, PostgreSQL migrations 002-004 with JSONB payloads and cursor indexes, and 16 store methods for signal inbox, channel CRUD, webhook config, and delivery tracking**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-11T20:11:24Z
- **Completed:** 2026-03-11T20:14:37Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- Complete Signal type hierarchy (Signal, SignalRoute, SignalTrust, SignalBody, SignalContext) matching CONTEXT.md spec
- Three migration pairs creating signals, channels, webhook_configs, and delivery_attempts tables with proper indexes
- 16 store methods covering full CRUD for signals, channels, webhook configs, and delivery attempts
- Cursor-based inbox pagination with hasMore flag and SSE replay support

## Task Commits

Each task was committed atomically:

1. **Task 1: Models and crypto helpers** - `b7d8a3b` (feat)
2. **Task 2: Database migrations** - `4fb8b15` (feat)
3. **Task 3: Store methods for signals, channels, webhooks, and delivery tracking** - `d9a53f5` (feat)

## Files Created/Modified
- `platform/internal/models/models.go` - Added Signal, Channel, WebhookConfig, DeliveryAttempt types and constants
- `platform/internal/crypto/crypto.go` - Added NewSignalID() and NewDeliveryAttemptID() generators
- `platform/internal/store/store.go` - Added 16 store methods with scanSignal/scanChannel helpers
- `platform/migrations/002_signals.up.sql` - Signals table with inbox, cursor, idempotency, TTL, thread indexes
- `platform/migrations/002_signals.down.sql` - Drop signals table
- `platform/migrations/003_channels.up.sql` - Channels and webhook_configs tables
- `platform/migrations/003_channels.down.sql` - Drop webhook_configs and channels tables
- `platform/migrations/004_webhook_delivery.up.sql` - Delivery attempts table with retry and cleanup indexes
- `platform/migrations/004_webhook_delivery.down.sql` - Drop delivery_attempts table

## Decisions Made
- Used private scanSignal/scanChannel helpers to centralize column scanning and avoid repetition
- Capped GetSignalsAfter at 1000 rows to prevent memory issues during SSE replay
- Stored tags as JSONB arrays (consistent with existing patterns), marshaled/unmarshaled in Go

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All data types and store methods ready for API handler implementation (Plans 02, 03)
- Migrations ready for database setup
- ErrDuplicateSignal sentinel ready for 409 response handling in signal API

## Self-Check: PASSED

All 9 files verified present. All 3 task commits verified in git log.

---
*Phase: 02-signal-pipeline*
*Completed: 2026-03-11*
