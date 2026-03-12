---
phase: 05-mobile-push-notifications
plan: 01
subsystem: mobile
tags: [firebase, fcm, push-notifications, flutter, dart]

# Dependency graph
requires:
  - phase: 03-mobile-app
    provides: Flutter app with push provider stub, router, auth, inbox
  - phase: 04-fix-signal-pipeline-bugs
    provides: Working signal pipeline with correct JSON serialization
provides:
  - FCM push notification pipeline (token acquisition, registration, tap handling)
  - Firebase native platform configuration (Android + iOS)
  - Firebase setup documentation
affects: [platform-push-dispatch, mobile-notification-ux]

# Tech tracking
tech-stack:
  added: [firebase_core, firebase_messaging]
  patterns: [top-level background handler isolate, addPostFrameCallback for cold start nav]

key-files:
  created:
    - docs/FIREBASE-SETUP.md
  modified:
    - mobile/pubspec.yaml
    - mobile/lib/main.dart
    - mobile/lib/providers/push_provider.dart
    - mobile/android/settings.gradle.kts
    - mobile/android/app/build.gradle.kts
    - mobile/ios/Runner/Info.plist
    - .gitignore

key-decisions:
  - "Firebase config files excluded from git via .gitignore (google-services.json, GoogleService-Info.plist)"
  - "No DefaultFirebaseOptions / FlutterFire CLI -- direct Firebase.initializeApp() per user decision"
  - "Skipped badge management for v1 simplicity -- iOS auto-manages from push payload"
  - "Notification handlers in _AtapAppState rather than separate widget for simplicity"

patterns-established:
  - "Top-level @pragma background handler: Firebase background messages run in separate isolate"
  - "addPostFrameCallback for cold-start navigation: ensures router is mounted before go()"
  - "Platform.isIOS/isAndroid detection: mobile-only, no web fallback needed"

requirements-completed: [MOB-02, MOB-03]

# Metrics
duration: 4min
completed: 2026-03-12
---

# Phase 05 Plan 01: Mobile Push Notifications Summary

**FCM push pipeline with token registration, three-state notification tap handling (cold/background/foreground), and Firebase native config for Android and iOS**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-12T15:01:04Z
- **Completed:** 2026-03-12T15:05:30Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Firebase dependencies added and native platforms configured (google-services plugin for Android, UIBackgroundModes for iOS)
- PushNotifier stub activated with permission request, FCM token acquisition, platform registration, and token refresh
- Notification tap navigation for all three app states: cold start (getInitialMessage), background resume (onMessageOpenedApp), foreground (onMessage with SnackBar)
- Platform detection fixed from hardcoded 'android' to runtime Platform.isIOS/Platform.isAndroid
- Complete Firebase setup guide created at docs/FIREBASE-SETUP.md

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Firebase dependencies and native platform configuration** - `f9fcc8b` (feat)
2. **Task 2: Activate PushNotifier and add notification tap handling** - `65e4cfb` (feat)

## Files Created/Modified
- `mobile/pubspec.yaml` - Added firebase_core and firebase_messaging dependencies
- `mobile/lib/main.dart` - Firebase init, background handler, notification tap handling in _AtapAppState
- `mobile/lib/providers/push_provider.dart` - Activated FCM permission, token, registration, platform detection
- `mobile/android/settings.gradle.kts` - google-services plugin (KTS syntax)
- `mobile/android/app/build.gradle.kts` - google-services plugin application
- `mobile/ios/Runner/Info.plist` - UIBackgroundModes with fetch and remote-notification
- `docs/FIREBASE-SETUP.md` - Step-by-step Firebase project setup guide
- `.gitignore` - Added google-services.json and GoogleService-Info.plist exclusions

## Decisions Made
- Firebase config files excluded from git -- they contain project-specific config and must be set up per docs/FIREBASE-SETUP.md
- No FlutterFire CLI / DefaultFirebaseOptions -- direct Firebase.initializeApp() keeps setup simpler
- Skipped badge management for v1 -- iOS auto-manages badge count from push payload
- Notification handlers placed directly in _AtapAppState.initState rather than a separate widget wrapper

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added Firebase config files to .gitignore**
- **Found during:** Task 1
- **Issue:** google-services.json and GoogleService-Info.plist should not be committed per Firebase best practices
- **Fix:** Added glob patterns to .gitignore
- **Files modified:** .gitignore
- **Verification:** Patterns present in .gitignore
- **Committed in:** f9fcc8b (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Essential security/configuration hygiene. No scope creep.

## Issues Encountered
None

## User Setup Required

**External services require manual configuration.** See [docs/FIREBASE-SETUP.md](../../../docs/FIREBASE-SETUP.md) for:
- Creating Firebase project and adding Android/iOS apps
- Downloading google-services.json and GoogleService-Info.plist
- Uploading APNs authentication key for iOS push
- Generating service account key for platform server (GOOGLE_APPLICATION_CREDENTIALS)

**Important:** The app will not compile without Firebase config files. This is intentional.

## Next Phase Readiness
- Push notification pipeline is complete from FCM token registration through notification tap navigation
- Runtime testing requires Firebase project setup (user action) and physical device
- Platform server push dispatch (sending FCM messages) was implemented in Phase 03

---
*Phase: 05-mobile-push-notifications*
*Completed: 2026-03-12*
