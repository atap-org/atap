---
phase: 03-mobile-app
plan: 03
subsystem: api
tags: [firebase, fcm, push-notifications, claims, delegations, human-registration]

# Dependency graph
requires:
  - phase: 03-01
    provides: Store methods (claims, delegations, push_tokens), models, migrations
provides:
  - Claim creation and lookup API endpoints
  - Human registration with claim redemption and delegation creation
  - Push token registration endpoint
  - Firebase push notification service integrated into signal pipeline
affects: [03-04, 03-05]

# Tech tracking
tech-stack:
  added: [firebase.google.com/go/v4, firebase.google.com/go/v4/messaging]
  patterns: [PushNotifier interface for testability, setter-based dependency injection for optional services]

key-files:
  created:
    - platform/internal/api/claims.go
    - platform/internal/api/human.go
    - platform/internal/api/push.go
    - platform/internal/push/push.go
    - platform/internal/push/push_test.go
  modified:
    - platform/internal/api/api.go
    - platform/internal/api/api_test.go
    - platform/cmd/server/main.go

key-decisions:
  - "PushNotifier interface on Handler enables nil-safe push dispatch and test mocking"
  - "Setter-based injection (SetClaimStore, SetPushService) preserves backward-compatible NewHandler signature"
  - "Firebase initialization is conditional on GOOGLE_APPLICATION_CREDENTIALS env var"

patterns-established:
  - "Optional service pattern: nil-check before dispatch (pushService, webhookWorker)"
  - "Fire-and-forget goroutine with context.Background() for push notifications"

requirements-completed: [MOB-03, MOB-04]

# Metrics
duration: 4min
completed: 2026-03-11
---

# Phase 3 Plan 3: Platform API Endpoints Summary

**Claim creation, human registration with delegation, push token management, and Firebase push notification service integrated into signal pipeline**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-11T22:31:46Z
- **Completed:** 2026-03-11T22:36:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Four new API endpoints: POST /v1/claims, GET /v1/claims/:code, POST /v1/register/human, POST /v1/entities/:entityId/push-token
- Push notification service with Firebase Admin SDK (FCMClient interface for testability)
- Full TDD cycle for push service: 4 table-driven tests covering valid token, no token, FCM error, content format
- Signal pipeline integration: push notifications dispatched alongside SSE and webhooks

## Task Commits

Each task was committed atomically:

1. **Task 1: Create claim, human registration, and push token API endpoints** - `a40e67f` (feat)
2. **Task 2: Push notification service (RED)** - `d0105bb` (test)
3. **Task 2: Push notification service (GREEN)** - `52f7e8d` (feat)

_Note: Task 2 followed TDD with RED and GREEN commits._

## Files Created/Modified
- `platform/internal/api/claims.go` - CreateClaim and GetClaim endpoints
- `platform/internal/api/human.go` - RegisterHuman endpoint with claim redemption + delegation
- `platform/internal/api/push.go` - RegisterPushToken endpoint
- `platform/internal/api/api.go` - New store interfaces, Handler extensions, route registration, push dispatch in SendSignal
- `platform/internal/api/api_test.go` - fakeStore extended with claim/delegation/pushToken support
- `platform/internal/push/push.go` - PushService with FCMClient interface, fire-and-forget SendNotification
- `platform/internal/push/push_test.go` - 4 table-driven tests with mock FCM client
- `platform/cmd/server/main.go` - Firebase conditional init, store wiring

## Decisions Made
- PushNotifier interface on Handler enables nil-safe push dispatch and mock testing without Firebase
- Setter-based injection (SetClaimStore, SetDelegationStore, SetPushTokenStore, SetPushService) preserves backward-compatible NewHandler signature
- Firebase initialization conditional on GOOGLE_APPLICATION_CREDENTIALS env var with graceful degradation
- Push notifications use fire-and-forget goroutine with context.Background() to avoid request context cancellation

## Deviations from Plan

None - plan executed exactly as written.

## User Setup Required

**External services require manual configuration.** For Firebase push notifications:
- Set `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to Firebase service account JSON file
- Create Firebase project at console.firebase.google.com
- Enable Cloud Messaging in Firebase Console
- Upload APNs auth key for iOS push support

Without these, the platform runs normally with push notifications disabled.

## Next Phase Readiness
- Platform API complete for mobile app onboarding flow (claim -> register -> delegation)
- Push notification infrastructure ready for Flutter app integration
- All endpoints tested via unit tests with fakeStore

---
*Phase: 03-mobile-app*
*Completed: 2026-03-11*
