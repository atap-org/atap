import 'dart:developer' as dev;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import 'auth_provider.dart';

/// Push notification provider managing FCM token registration.
///
/// Note: Full Firebase Messaging integration requires:
/// 1. Firebase project setup (console.firebase.google.com)
/// 2. google-services.json (Android) / GoogleService-Info.plist (iOS)
/// 3. firebase_messaging package in pubspec.yaml
///
/// This provider is structured to handle the full flow but operates
/// as a stub until Firebase is configured. When firebase_messaging
/// is available, uncomment the Firebase-specific code.
class PushState {
  final bool isRegistered;
  final String? fcmToken;
  final String? error;

  const PushState({
    this.isRegistered = false,
    this.fcmToken,
    this.error,
  });

  PushState copyWith({
    bool? isRegistered,
    String? fcmToken,
    String? error,
    bool clearError = false,
  }) {
    return PushState(
      isRegistered: isRegistered ?? this.isRegistered,
      fcmToken: fcmToken ?? this.fcmToken,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Push notification notifier.
///
/// When Firebase is configured, this will:
/// 1. Request notification permission on first launch
/// 2. Get FCM token via FirebaseMessaging.instance.getToken()
/// 3. POST token to /v1/entities/{entityId}/push-token
/// 4. Listen to onTokenRefresh and re-register
/// 5. Handle foreground messages via onMessage stream
class PushNotifier extends Notifier<PushState> {
  @override
  PushState build() => const PushState();

  ApiClient get _apiClient => ref.read(apiClientProvider);

  /// Initializes push notifications.
  ///
  /// Currently a stub that logs a message. When firebase_messaging
  /// is added as a dependency, this will:
  /// - Request permission
  /// - Get FCM token
  /// - Register with platform
  /// - Set up token refresh listener
  Future<void> initialize() async {
    dev.log(
      'Push notifications not configured. '
      'Add firebase_messaging to pubspec.yaml and configure Firebase project.',
      name: 'PushProvider',
    );

    // Uncomment when firebase_messaging is available:
    //
    // final messaging = FirebaseMessaging.instance;
    //
    // // Request permission
    // final settings = await messaging.requestPermission(
    //   alert: true,
    //   badge: true,
    //   sound: true,
    // );
    //
    // if (settings.authorizationStatus != AuthorizationStatus.authorized) {
    //   state = state.copyWith(
    //     error: 'Push notification permission denied',
    //   );
    //   return;
    // }
    //
    // // Get FCM token
    // final token = await messaging.getToken();
    // if (token != null) {
    //   await _registerToken(token);
    // }
    //
    // // Listen for token refresh
    // messaging.onTokenRefresh.listen(_registerToken);
    //
    // // Handle foreground messages
    // FirebaseMessaging.onMessage.listen((message) {
    //   dev.log('Foreground message: ${message.messageId}', name: 'PushProvider');
    // });
  }

  /// Registers an FCM token with the platform.
  Future<void> registerToken(String token) async {
    final authState = ref.read(authProvider);
    if (authState.currentEntity == null) return;

    try {
      final entityId = authState.currentEntity!.id;
      await _apiClient.post(
        '/v1/entities/$entityId/push-token',
        {
          'token': token,
          'platform': _detectPlatform(),
        },
      );

      state = PushState(
        isRegistered: true,
        fcmToken: token,
      );
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to register push token: $e',
      );
    }
  }

  /// Detects the current platform for push token registration.
  String _detectPlatform() {
    // Platform detection without dart:io (works in tests)
    // In production, use Platform.isAndroid / Platform.isIOS
    return 'android'; // Default for now
  }
}

/// Provider for push notification state.
final pushProvider = NotifierProvider<PushNotifier, PushState>(
  PushNotifier.new,
);
