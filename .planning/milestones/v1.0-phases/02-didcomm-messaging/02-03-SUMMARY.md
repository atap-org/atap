---
phase: 02-didcomm-messaging
plan: 03
subsystem: api
tags: [didcomm, jwe, ecdh-1pu, message-queue, postgres, redis, fiber]

# Dependency graph
requires:
  - phase: 02-didcomm-messaging/02-01
    provides: ECDH-1PU JWE encrypt/decrypt and PlaintextMessage types
  - phase: 02-didcomm-messaging/02-02
    provides: Entity X25519 keys, DID Document, server DID identity

provides:
  - POST /v1/didcomm — public endpoint accepting DIDComm v2.1 JWE, queues for recipient
  - GET /v1/didcomm/inbox — DPoP-authenticated inbox pickup with mark-delivered semantics
  - store.MessageStore — PostgreSQL CRUD for didcomm_messages table
  - didcomm.ExtractRecipientKID / ExtractSenderKID — JWE metadata extraction without decryption
  - didcomm.ValidateRecipientDomain — anti-forwarding domain check for did:web DIDs

affects: [phase-3-credentials, phase-4-federation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - JWE KID extraction without decryption (inspect recipients[0].header.kid and base64url-decoded protected.skid)
    - Anti-forwarding check: extract domain from did:web:domain:path#fragment before accepting message
    - Public send endpoint (no auth) + authenticated inbox pickup (DPoP + scope)
    - Redis PUBLISH to inbox:{entity_id} for live notification (best-effort, non-fatal)
    - Base64-encoded JWE payload in inbox response for safe JSON transport

key-files:
  created:
    - platform/internal/api/didcomm_handler.go
    - platform/internal/api/didcomm_handler_test.go
    - platform/internal/store/messages.go
    - platform/internal/store/messages_test.go
    - platform/internal/didcomm/mediator.go
    - platform/internal/didcomm/mediator_test.go
    - platform/internal/didcomm/queue.go
  modified:
    - platform/internal/api/api.go
    - platform/cmd/server/main.go

key-decisions:
  - "POST /v1/didcomm requires no auth — DIDComm is self-authenticating via ECDH-1PU; Content-Type application/didcomm-encrypted+json is the gating check"
  - "Foreign DID check: strip #fragment from KID, split did:web:{domain}:... and compare domain segment to PlatformDomain"
  - "Inbox response encodes payload with base64.StdEncoding (not RawURL) for broad client compatibility"
  - "MarkDelivered failure is non-fatal (logged as warning) — message was already returned to client"
  - "Redis PUBLISH failure is non-fatal — message queued in DB is the authoritative delivery record"

patterns-established:
  - "Pattern: MessageStore interface in api package mirrors didcomm.MessageStore — allows store.Store to satisfy both"
  - "Pattern: Handler field messageStore injected through NewHandler alongside entityStore/oauthTokenStore"

requirements-completed: [MSG-01, MSG-02, MSG-03, MSG-04, MSG-05, API-05]

# Metrics
duration: 6min
completed: 2026-03-13
---

# Phase 2 Plan 03: DIDComm HTTP Endpoint, Message Queue, and Inbox Summary

**ECDH-1PU JWE mediator with PostgreSQL queue: POST /v1/didcomm routes encrypted messages to recipients, GET /v1/didcomm/inbox delivers and marks them consumed**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-03-13T21:57:32Z
- **Completed:** 2026-03-13T22:03:21Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments

- Full send-receive DIDComm cycle: POST queues JWE into `didcomm_messages`, GET picks up and marks delivered
- Mediator KID extraction without decryption (reads `recipients[0].header.kid` and base64url-decodes `protected.skid`)
- Anti-forwarding domain validation: rejects JWEs addressed to foreign DIDs before any DB lookup
- Redis PUBLISH to `inbox:{entity_id}` for live SSE notification (best-effort, skipped if Redis down)

## Task Commits

1. **Task 1: Message queue store and mediator routing** - `40143b1` (feat)
2. **Task 2: POST /v1/didcomm and GET /v1/didcomm/inbox handlers** - `26b0b58` (feat)

**Plan metadata:** (pending)

## Files Created/Modified

- `platform/internal/didcomm/mediator.go` - ExtractRecipientKID, ExtractSenderKID, ValidateRecipientDomain
- `platform/internal/didcomm/mediator_test.go` - KID extraction and domain validation tests
- `platform/internal/didcomm/queue.go` - MessageStore interface definition
- `platform/internal/store/messages.go` - QueueMessage, GetPendingMessages, MarkDelivered, CleanupExpiredMessages
- `platform/internal/store/messages_test.go` - In-memory contract tests for MessageStore
- `platform/internal/api/didcomm_handler.go` - HandleDIDComm (public) and HandleInbox (DPoP auth)
- `platform/internal/api/didcomm_handler_test.go` - Handler integration tests
- `platform/internal/api/api.go` - MessageStore interface, Handler field, route registration
- `platform/cmd/server/main.go` - Wire store.Store as MessageStore in NewHandler

## Decisions Made

- POST /v1/didcomm requires no OAuth/DPoP — DIDComm is self-authenticating via ECDH-1PU; the Content-Type header `application/didcomm-encrypted+json` is the required format check
- Foreign DID validation strips the `#fragment` from the KID, then checks that `did:web:{domain}:...` third segment equals PlatformDomain
- Inbox response uses `base64.StdEncoding` for payload encoding (not RawURL) for broad client compatibility
- `MarkDelivered` failure on inbox pickup is non-fatal — message was already returned to client; idempotent re-pickup is acceptable
- Redis PUBLISH failure is non-fatal — the PostgreSQL queue is the authoritative delivery record

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Minor: `ecdh.X25519()` returns an interface value (not `*Curve`), causing a compile error in the initial test draft when using a helper function return type. Fixed inline by calling `ecdh.X25519().NewPublicKey(...)` directly.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 2 (DIDComm Messaging) is now complete — all 3 plans done
- Phase 3 (Credentials + Mobile) can begin
- The DIDComm message channel is fully operational end-to-end

---
*Phase: 02-didcomm-messaging*
*Completed: 2026-03-13*
