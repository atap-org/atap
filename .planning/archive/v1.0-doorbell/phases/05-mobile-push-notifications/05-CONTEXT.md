# Phase 5: Mobile Push Notifications - Context

**Gathered:** 2026-03-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete the mobile push notification pipeline — add `firebase_messaging` to the Flutter app, acquire FCM tokens, register them with the platform, and deliver push notifications when new signals arrive. Includes Signal.fromJson robustness confirmation (Phase 4 fix). No new platform API work — push token endpoint and PushService already exist.

</domain>

<decisions>
## Implementation Decisions

### Firebase project setup
- No Firebase project exists yet — must be created from scratch
- Phase writes all Flutter/platform code; Firebase console setup is documented in `docs/FIREBASE-SETUP.md`
- Setup doc covers both Android (google-services.json) and iOS (GoogleService-Info.plist + APNs key upload to Firebase)
- App requires Firebase config files to compile — no graceful degradation / stub mode
- FlutterFire CLI not used — manual setup with checklist

### Notification tap behavior
- Tapping a push notification navigates to the signal detail screen (uses `signal_id` from notification data payload)
- Handle cold start: check for initial notification on app launch, navigate to signal after loading
- Handle background resume: notification tap while app is backgrounded navigates to signal
- Foreground: show in-app banner (toast/snackbar) when a signal arrives while app is open; tapping the banner navigates to signal detail
- App badge (unread count on icon) is managed — set on push, clear when user opens inbox

### Platform detection
- Both iOS and Android supported from day one (carries forward from Phase 3 decision)
- Fix `_detectPlatform()` to use `Platform.isIOS` / `Platform.isAndroid` instead of hardcoded 'android'
- All tokens treated as FCM tokens — Firebase handles APNs routing for iOS
- `platform` field in push token registration is informational only (no routing logic difference)

### Claude's Discretion
- Badge count implementation (server-side count in push payload vs client-side tracking)
- In-app banner widget choice (SnackBar, overlay, or local notification)
- Navigation approach for notification taps (GoRouter, Navigator, or deep link handler)
- Exact Firebase initialization placement in app startup

</decisions>

<specifics>
## Specific Ideas

- Existing `push_provider.dart` has commented-out Firebase code — activate and extend rather than rewrite
- Platform-side PushService already sends `signal_id` and `type` in notification data payload — use these for navigation
- Phase 3 decided notification content: "signal type + sender" with no payload preview for privacy

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `mobile/lib/providers/push_provider.dart`: PushNotifier stub with commented-out Firebase flow (permission → token → register → refresh listener)
- `mobile/lib/core/models/signal.dart`: Signal.fromJson already parses `ts` field (Phase 4 fix applied)
- `platform/internal/push/push.go`: PushService with FCMClient interface, sends `signal_id` + `type` in data payload
- `platform/internal/api/push.go`: RegisterPushToken handler — POST /v1/entities/{id}/push-token (fully implemented)
- `platform/migrations/007_push_tokens.up.sql`: Push token table with entity_id primary key (upsert pattern)
- `mobile/lib/features/inbox/signal_detail_screen.dart`: Signal detail screen exists — navigation target for notification taps

### Established Patterns
- Riverpod 3.x Notifier pattern (PushNotifier already follows this)
- ApiClient with Ed25519 signed requests — push token registration uses same auth
- SSE streaming for real-time inbox updates — push notifications complement this for background delivery

### Integration Points
- `push_provider.dart` → activate Firebase code, add navigation handling
- `pubspec.yaml` → add `firebase_messaging`, `firebase_core` dependencies
- `main.dart` → add Firebase.initializeApp() before runApp
- App router/navigator → handle notification tap routing to signal detail screen
- Inbox screen → clear badge count on open

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-mobile-push-notifications*
*Context gathered: 2026-03-12*
