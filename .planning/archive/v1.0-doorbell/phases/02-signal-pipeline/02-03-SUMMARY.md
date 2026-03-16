---
phase: 02-signal-pipeline
plan: 03
subsystem: api
tags: [webhooks, channels, ed25519, bcrypt, basic-auth, exponential-backoff, goroutine-pool, fiber]

# Dependency graph
requires:
  - phase: 02-signal-pipeline
    provides: Signal/Channel/WebhookConfig/DeliveryAttempt types, store methods, migrations 002-004
  - phase: 01-foundation
    provides: Entity model, Ed25519 crypto, auth middleware
provides:
  - WebhookWorker with bounded goroutine pool and exponential backoff retry
  - SetWebhook endpoint for webhook URL registration
  - CreateChannel, ListChannels, RevokeChannel endpoints
  - ChannelInbound handler for external webhook reception
  - ChannelStore and WebhookStore interfaces
  - Webhook payload signing with Ed25519 platform key
  - Open channel Basic Auth with bcrypt-hashed credentials
affects: [02-04-integration-tests]

# Tech tracking
tech-stack:
  added: [golang.org/x/crypto/bcrypt]
  patterns: [WebhookWorker goroutine pool with bounded channel, exponential backoff retry with configurable delays, channel-type polymorphic auth (Ed25519 for trusted, Basic Auth for open)]

key-files:
  created:
    - platform/internal/api/webhook.go
  modified:
    - platform/internal/api/api.go
    - platform/internal/api/api_test.go
    - platform/cmd/server/main.go

key-decisions:
  - "WebhookWorker uses bounded channel (1000 capacity) with non-blocking send to avoid backpressure on API handlers"
  - "Open channels use bcrypt-hashed Basic Auth credentials, returned once at creation"
  - "Channel inbound wraps external payloads with source=external and trust_level=0 for open channels"
  - "Webhook payload signed with platform Ed25519 key via X-ATAP-Signature header"
  - "Handler struct expanded with ChannelStore, WebhookStore, WebhookWorker dependencies"

patterns-established:
  - "Store interface segregation: EntityStore, SignalStore, ChannelStore, WebhookStore all satisfied by single Store implementation"
  - "Channel-type polymorphic auth: trusted channels use Ed25519 signature, open channels use Basic Auth"
  - "Background worker pattern: WebhookWorker with Start/Enqueue/StartRetryPoller/StartCleanupJob lifecycle"

requirements-completed: [WHK-01, WHK-02, WHK-03, WHK-04, CHN-01, CHN-02, CHN-03, CHN-04]

# Metrics
duration: 9min
completed: 2026-03-11
---

# Phase 2 Plan 03: Webhook Push Delivery and Inbound Channels Summary

**WebhookWorker with Ed25519-signed push delivery and exponential backoff retry, plus inbound channels with polymorphic auth (Ed25519 trusted / bcrypt Basic Auth open) wrapping external payloads into ATAP signals**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-11T20:17:29Z
- **Completed:** 2026-03-11T20:27:15Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- WebhookWorker with bounded goroutine pool (4 workers), 10s HTTP timeout, exponential backoff (1s/5s/30s/5m/30m), max 5 attempts
- Full webhook/channel API: SetWebhook, CreateChannel (trusted+open), ListChannels, RevokeChannel, ChannelInbound
- Channel inbound correctly wraps external payloads into ATAP signals with proper source/trust metadata
- SendSignal handler wired to enqueue webhook delivery after Redis publish
- 11 new unit tests covering all webhook and channel scenarios (28 total API tests pass)

## Task Commits

Each task was committed atomically:

1. **Task 1: WebhookWorker and webhook/channel store interfaces** - `3d44700` (feat)
2. **Task 2: Unit tests for webhooks and channels** - `806bb44` (test)

## Files Created/Modified
- `platform/internal/api/webhook.go` - WebhookWorker struct with goroutine pool, deliver with Ed25519 signing, retry poller, cleanup job, channelInboundFromPayload helper
- `platform/internal/api/api.go` - Added ChannelStore/WebhookStore interfaces, SetWebhook/CreateChannel/ListChannels/RevokeChannel/ChannelInbound handlers, webhook delivery in SendSignal
- `platform/internal/api/api_test.go` - Extended fakeStore with full channel/webhook implementations, 11 new tests
- `platform/cmd/server/main.go` - Updated NewHandler call with all 4 store interfaces, WebhookWorker initialization and startup

## Decisions Made
- WebhookWorker uses bounded channel (1000 capacity) with non-blocking send -- drops jobs when full to avoid API handler backpressure
- Open channels generate 32-byte random password, base64url encoded, bcrypt hashed -- password returned once at creation
- Channel inbound wraps payloads with source="external" and trust_level=0 for open channels; source="agent" for trusted
- Webhook payload signed with platform Ed25519 key in X-ATAP-Signature header (base64 encoded)
- Handler struct uses interface segregation (4 separate store interfaces) all satisfied by single Store

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test entity keyID collisions**
- **Found during:** Task 2 (unit tests)
- **Issue:** Test entity pairs with IDs sharing the same first 8 characters produced identical keyIDs, causing fakeStore keyIndex overwrite and signature verification failures
- **Fix:** Changed test entity IDs to have unique 8-char prefixes
- **Files modified:** platform/internal/api/api_test.go
- **Verification:** All 28 tests pass
- **Committed in:** 806bb44 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Test-only fix for keyID collision in fakeStore. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Webhook push delivery and inbound channels fully operational
- All API endpoints tested and verified
- Ready for integration tests (Plan 04)

## Self-Check: PASSED

All 4 files verified present. Both task commits verified in git log.

---
*Phase: 02-signal-pipeline*
*Completed: 2026-03-11*
