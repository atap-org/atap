# Phase 5: Mobile Push Notifications - Research

**Researched:** 2026-03-12
**Domain:** Flutter FCM integration, push notification pipeline, notification tap navigation
**Confidence:** HIGH

## Summary

Phase 5 completes the mobile push notification pipeline. The platform side is already built (PushService with FCM client, push token endpoint, migration). The Flutter side has a stubbed `PushNotifier` with commented-out Firebase code that needs activation. The work is: add `firebase_core` + `firebase_messaging` dependencies, configure Firebase for both Android and iOS (manual setup, no FlutterFire CLI), activate the push provider code, fix `_detectPlatform()`, handle notification taps via GoRouter, and add foreground in-app banners.

The project already uses GoRouter with a `/inbox/:signalId` route, making notification tap navigation straightforward. The app uses Riverpod 3.x Notifier pattern consistently. The existing `PushNotifier` stub closely matches the final implementation shape -- it just needs Firebase imports activated and navigation handling added.

**Primary recommendation:** Activate existing push_provider.dart stub with Firebase imports, add notification tap handling via GoRouter navigation to `/inbox/{signalId}`, use SnackBar for foreground in-app notifications (simplest, consistent with existing clipboard feedback pattern), and track badge count client-side.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- No Firebase project exists yet -- must be created from scratch
- Phase writes all Flutter/platform code; Firebase console setup is documented in `docs/FIREBASE-SETUP.md`
- Setup doc covers both Android (google-services.json) and iOS (GoogleService-Info.plist + APNs key upload to Firebase)
- App requires Firebase config files to compile -- no graceful degradation / stub mode
- FlutterFire CLI not used -- manual setup with checklist
- Tapping a push notification navigates to the signal detail screen (uses `signal_id` from notification data payload)
- Handle cold start: check for initial notification on app launch, navigate to signal after loading
- Handle background resume: notification tap while app is backgrounded navigates to signal
- Foreground: show in-app banner (toast/snackbar) when a signal arrives while app is open; tapping the banner navigates to signal detail
- App badge (unread count on icon) is managed -- set on push, clear when user opens inbox
- Both iOS and Android supported from day one
- Fix `_detectPlatform()` to use `Platform.isIOS` / `Platform.isAndroid` instead of hardcoded 'android'
- All tokens treated as FCM tokens -- Firebase handles APNs routing for iOS
- `platform` field in push token registration is informational only (no routing logic difference)

### Claude's Discretion
- Badge count implementation (server-side count in push payload vs client-side tracking)
- In-app banner widget choice (SnackBar, overlay, or local notification)
- Navigation approach for notification taps (GoRouter, Navigator, or deep link handler)
- Exact Firebase initialization placement in app startup

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MOB-02 | Inbox view displaying received signals with pull-to-refresh | Inbox screen already exists with pull-to-refresh and infinite scroll. Phase 5 confirms Signal.fromJson robustness (Phase 4 fix verified in tests). No new inbox work needed -- requirement is already met. |
| MOB-03 | Push notification setup (FCM for Android, APNs for iOS) -- token registered with platform | Full Firebase manual setup documented below. Push token registration endpoint already exists. PushNotifier stub has the registration flow ready. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| firebase_core | ^4.3.0 | Firebase initialization | Required dependency for all Firebase plugins |
| firebase_messaging | ^16.1.2 | FCM token acquisition, message handling, notification taps | Official Flutter FCM plugin, only option for Firebase push |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go_router | ^17.1.0 | Notification tap navigation | Already in project; use for `/inbox/{signalId}` routing on tap |
| flutter_riverpod | ^3.3.1 | State management for push state | Already in project; PushNotifier follows Notifier pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| SnackBar for foreground banner | flutter_local_notifications | Local notifications are more native-feeling but add complexity and another dependency; SnackBar is simpler and consistent with existing clipboard feedback pattern |
| Client-side badge count | Server-side badge count in push payload | Server-side requires platform changes (count query + payload field); client-side works with existing SSE signal count -- no platform changes needed |
| GoRouter for notification nav | Navigator.pushNamed | GoRouter already handles all routing; mixing navigators creates confusion |

**Installation:**
```bash
cd mobile && flutter pub add firebase_core firebase_messaging
```

## Architecture Patterns

### Recommended Project Structure
```
mobile/
  lib/
    providers/
      push_provider.dart         # Activate existing stub
    app/
      router.dart                # Add notification tap handler integration
    main.dart                    # Add Firebase.initializeApp()
  android/
    app/
      google-services.json       # Firebase config (gitignored, documented)
      build.gradle.kts           # Add google-services plugin
    build.gradle.kts             # Add google-services classpath (already has repositories)
    settings.gradle.kts          # Add google-services plugin version
  ios/
    Runner/
      GoogleService-Info.plist   # Firebase config (gitignored, documented)
      AppDelegate.swift          # No changes needed (method swizzling handles it)
      Info.plist                 # Add background modes
    Podfile                      # No changes needed (flutter_install_all_ios_pods handles Firebase)
  docs/
    FIREBASE-SETUP.md            # Step-by-step Firebase console setup guide
```

### Pattern 1: Firebase Initialization in main.dart
**What:** Initialize Firebase before runApp, after WidgetsFlutterBinding
**When to use:** Always -- Firebase must initialize before any Firebase service is used

```dart
// Source: https://firebase.flutter.dev/docs/messaging/usage
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';

// Top-level background message handler (must be top-level, not a class method)
@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  await Firebase.initializeApp();
  // No-op: background messages handled by system notification tray
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp();
  FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
  runApp(const ProviderScope(child: AtapApp()));
}
```

### Pattern 2: Notification Tap Handling with GoRouter
**What:** Handle notification taps across three app states (terminated, background, foreground)
**When to use:** After Firebase init, in the root app widget

```dart
// Source: https://firebase.flutter.dev/docs/messaging/notifications
// In the AtapApp widget's initState or a dedicated setup method:

// 1. Terminated: app was closed, user tapped notification
final initialMessage = await FirebaseMessaging.instance.getInitialMessage();
if (initialMessage != null) {
  _handleNotificationTap(initialMessage);
}

// 2. Background: app was in background, user tapped notification
FirebaseMessaging.onMessageOpenedApp.listen(_handleNotificationTap);

// 3. Foreground: app is open, show SnackBar
FirebaseMessaging.onMessage.listen((message) {
  // Show in-app banner via SnackBar
});

void _handleNotificationTap(RemoteMessage message) {
  final signalId = message.data['signal_id'];
  if (signalId != null) {
    router.go('/inbox/$signalId');
  }
}
```

### Pattern 3: PushNotifier Activation
**What:** Replace stub code with real Firebase calls
**When to use:** The existing commented-out code in push_provider.dart is almost correct

```dart
// Source: existing push_provider.dart + Firebase docs
import 'dart:io' show Platform;
import 'package:firebase_messaging/firebase_messaging.dart';

Future<void> initialize() async {
  final messaging = FirebaseMessaging.instance;

  final settings = await messaging.requestPermission(
    alert: true,
    badge: true,
    sound: true,
  );

  if (settings.authorizationStatus != AuthorizationStatus.authorized &&
      settings.authorizationStatus != AuthorizationStatus.provisional) {
    state = state.copyWith(error: 'Push notification permission denied');
    return;
  }

  final token = await messaging.getToken();
  if (token != null) {
    await registerToken(token);
  }

  // Listen for token refresh
  messaging.onTokenRefresh.listen(registerToken);
}

String _detectPlatform() {
  if (Platform.isIOS) return 'ios';
  if (Platform.isAndroid) return 'android';
  return 'unknown';
}
```

### Anti-Patterns to Avoid
- **Calling Firebase.initializeApp() inside a provider:** It must happen before runApp in main(). Providers initialize lazily and could race.
- **Using onMessage to create local notifications on iOS:** iOS shows foreground notifications automatically when `setForegroundNotificationPresentationOptions` is configured. Only Android needs manual foreground display.
- **Putting background handler inside a class:** `@pragma('vm:entry-point')` background handler must be a top-level function, not a method. It runs in a separate isolate.
- **Navigating before router is ready:** On cold start from notification tap, `getInitialMessage()` returns the message, but GoRouter may not be mounted yet. Check in `initState` with `Future.microtask` or `addPostFrameCallback`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| FCM token management | Custom token refresh logic | `FirebaseMessaging.instance.onTokenRefresh` | Firebase handles token lifecycle, rotation, and platform differences |
| iOS notification permission | Custom permission dialog | `FirebaseMessaging.instance.requestPermission()` | Handles iOS permission states (authorized, denied, provisional, notDetermined) |
| APNs token bridging | Custom APNs-to-FCM mapping | Firebase SDK handles this automatically | Firebase maps APNs tokens to FCM tokens transparently |
| Background message isolate | Custom isolate for background processing | `FirebaseMessaging.onBackgroundMessage()` | Firebase manages isolate lifecycle and plugin registration |

**Key insight:** Firebase Messaging abstracts all platform differences between FCM (Android) and APNs (iOS). The Flutter code is identical for both platforms -- only the native project configuration differs.

## Common Pitfalls

### Pitfall 1: Firebase Config Files Missing at Build Time
**What goes wrong:** App fails to compile without google-services.json (Android) or GoogleService-Info.plist (iOS)
**Why it happens:** These files come from Firebase Console and are often gitignored
**How to avoid:** Document setup in `docs/FIREBASE-SETUP.md` with exact steps. Include placeholder check in CI.
**Warning signs:** "No matching client found for package name" or "GoogleService-Info.plist not found"

### Pitfall 2: iOS Background Modes Not Enabled
**What goes wrong:** Push notifications arrive but app doesn't wake up in background
**Why it happens:** Missing "Remote notifications" background mode in Xcode capabilities
**How to avoid:** Add to Info.plist: `UIBackgroundModes` array with `remote-notification`
**Warning signs:** Notifications show but `onMessageOpenedApp` never fires

### Pitfall 3: Android Notification Channel Missing (Android 8+)
**What goes wrong:** Notifications silently dropped on Android 8+ (API 26+)
**Why it happens:** Android requires a notification channel for all notifications
**How to avoid:** Firebase Messaging creates a default channel, but for custom channels use `AndroidNotificationChannel`
**Warning signs:** `getToken()` succeeds but no notifications appear on device

### Pitfall 4: GoRouter Navigation on Cold Start Race Condition
**What goes wrong:** `getInitialMessage()` returns a message but navigation fails because router isn't mounted
**Why it happens:** Firebase returns the initial message before the widget tree is ready
**How to avoid:** Process initial message in `AtapApp.initState` using `Future.microtask` (same pattern used for `loadSavedAuth`)
**Warning signs:** App opens to inbox list instead of signal detail on notification tap

### Pitfall 5: Kotlin DSL (build.gradle.kts) Plugin Syntax
**What goes wrong:** Build fails with "Plugin not found" errors
**Why it happens:** Project uses `.kts` (Kotlin DSL) not `.gradle` (Groovy). Firebase docs show Groovy syntax.
**How to avoid:** Use `id("com.google.gms.google-services")` syntax in plugins block, not `apply plugin:` syntax
**Warning signs:** "Plugin with id 'com.google.gms.google-services' not found"

## Code Examples

### Android build.gradle.kts Changes (settings.gradle.kts)
```kotlin
// Source: https://firebase.google.com/docs/android/setup
// In android/settings.gradle.kts, add google-services plugin:
plugins {
    id("dev.flutter.flutter-plugin-loader") version "1.0.0"
    id("com.android.application") version "8.11.1" apply false
    id("org.jetbrains.kotlin.android") version "2.2.20" apply false
    id("com.google.gms.google-services") version "4.4.4" apply false  // ADD THIS
}
```

### Android app/build.gradle.kts Changes
```kotlin
// In android/app/build.gradle.kts, add google-services plugin:
plugins {
    id("com.android.application")
    id("kotlin-android")
    id("dev.flutter.flutter-gradle-plugin")
    id("com.google.gms.google-services")  // ADD THIS
}
```

### iOS Info.plist Background Modes
```xml
<!-- Add to ios/Runner/Info.plist -->
<key>UIBackgroundModes</key>
<array>
    <string>fetch</string>
    <string>remote-notification</string>
</array>
```

### iOS Foreground Notification Display
```dart
// Source: https://firebase.flutter.dev/docs/messaging/notifications
// Call after Firebase.initializeApp() in main.dart or push provider:
await FirebaseMessaging.instance.setForegroundNotificationPresentationOptions(
  alert: true,
  badge: true,
  sound: true,
);
```

### SnackBar Foreground Banner Pattern
```dart
// In AtapApp or a dedicated notification handler widget:
FirebaseMessaging.onMessage.listen((RemoteMessage message) {
  final notification = message.notification;
  if (notification == null) return;

  final signalId = message.data['signal_id'];
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text('${notification.title}: ${notification.body}'),
      action: signalId != null
          ? SnackBarAction(
              label: 'View',
              onPressed: () => context.go('/inbox/$signalId'),
            )
          : null,
      duration: const Duration(seconds: 4),
    ),
  );
});
```

### Badge Count (Client-Side)
```dart
// Clear badge when inbox opens (in InboxScreen.initState):
FlutterAppBadger.removeBadge(); // or use firebase_messaging to reset

// Platform-side already sends badge count? No -- client tracks via inbox signal count.
// Simple approach: clear on inbox open, rely on OS badge from push payload.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| FlutterFire CLI mandatory | Manual setup still supported | Always | Project uses manual setup per CONTEXT.md decision |
| build.gradle (Groovy) | build.gradle.kts (Kotlin DSL) | Flutter 3.29 (2025) | Must use KTS plugin syntax, not `apply plugin:` |
| onMessage + local_notifications (Android) | Android handles notification display from FCM automatically | firebase_messaging 14+ | Simpler: only need local_notifications for custom UI, not required for basic display |
| V1 Android embedding | V2 Android embedding | Flutter 2.0+ | No extra background handler registration needed |

**Deprecated/outdated:**
- `FirebaseAppDelegateProxyEnabled = NO`: Not recommended unless you have a specific reason. Method swizzling is the default and works for most setups.
- FlutterFire CLI generated `firebase_options.dart`: Not used in this project (manual setup). Don't reference `DefaultFirebaseOptions.currentPlatform`.

## Open Questions

1. **Firebase project creation timing**
   - What we know: No Firebase project exists yet. Code requires config files to compile.
   - What's unclear: Whether config files should be committed or gitignored (security vs convenience)
   - Recommendation: Gitignore config files, provide `FIREBASE-SETUP.md` with exact steps. Add `.example` placeholder files for discoverability.

2. **Badge count number source**
   - What we know: Platform push payload contains `signal_id` and `type` but no unread count
   - What's unclear: Whether to add unread count to push payload or manage client-side
   - Recommendation: Client-side for now. The inbox already loads signal count. Adding server-side count requires platform changes (out of phase scope). Use iOS `setForegroundNotificationPresentationOptions(badge: true)` which auto-increments.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | flutter_test (built-in) |
| Config file | none (uses pubspec.yaml test section) |
| Quick run command | `cd mobile && flutter test test/signal_model_test.dart` |
| Full suite command | `cd mobile && flutter test` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MOB-02 | Inbox displays signals with pull-to-refresh | widget | `cd mobile && flutter test test/inbox_widget_test.dart` | Yes |
| MOB-03 | Push notification token registration | unit | `cd mobile && flutter test test/push_provider_test.dart -x` | No -- Wave 0 |
| MOB-03 | Platform detection (iOS/Android) | unit | `cd mobile && flutter test test/push_provider_test.dart -x` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `cd mobile && flutter test`
- **Per wave merge:** `cd mobile && flutter test`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `mobile/test/push_provider_test.dart` -- covers MOB-03 (token registration call, platform detection, permission denied state)
- Note: Firebase initialization cannot be unit tested without Firebase project -- test the PushNotifier state transitions with mocked dependencies

## Sources

### Primary (HIGH confidence)
- [FlutterFire Messaging Overview](https://firebase.flutter.dev/docs/messaging/overview/) - setup steps, platform requirements
- [FlutterFire Messaging Usage](https://firebase.flutter.dev/docs/messaging/usage) - permissions, tokens, message handlers
- [FlutterFire Messaging Notifications](https://firebase.flutter.dev/docs/messaging/notifications) - notification tap handling, foreground display
- [FlutterFire Apple Integration](https://firebase.flutter.dev/docs/messaging/apple-integration) - iOS APNs setup, Xcode capabilities
- [FlutterFire Manual Installation Android](https://firebase.flutter.dev/docs/manual-installation/android/) - google-services.json, build.gradle setup
- [FlutterFire Manual Installation iOS](https://firebase.flutter.dev/docs/manual-installation/ios/) - GoogleService-Info.plist setup

### Secondary (MEDIUM confidence)
- [Firebase Android Setup](https://firebase.google.com/docs/android/setup) - KTS plugin syntax (`com.google.gms.google-services` version 4.4.4)
- [Code With Andrea - Kotlin DSL in Flutter 3.29](https://codewithandrea.com/articles/flutter-android-gradle-kts/) - KTS migration patterns

### Tertiary (LOW confidence)
- pub.dev version numbers (firebase_core ^4.3.0, firebase_messaging ^16.1.2) -- verified via web search but should confirm at `flutter pub add` time

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Firebase Messaging is the only option for FCM on Flutter; versions verified from pub.dev
- Architecture: HIGH - Existing code patterns (GoRouter, Riverpod Notifier, ApiClient) are well established; notification tap handling follows official FlutterFire docs
- Pitfalls: HIGH - KTS build file format confirmed from project files; background mode requirements from official docs

**Research date:** 2026-03-12
**Valid until:** 2026-04-12 (stable -- Firebase Messaging API surface is mature)
