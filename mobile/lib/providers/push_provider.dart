import 'dart:developer' as dev;
import 'dart:io' show Platform;

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import 'auth_provider.dart';

/// Push notification state.
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

/// Push notification notifier managing FCM token registration.
///
/// Handles:
/// 1. Requesting notification permission on first launch
/// 2. Getting FCM token via FirebaseMessaging.instance.getToken()
/// 3. POSTing token to /v1/entities/{entityId}/push-token
/// 4. Listening to onTokenRefresh and re-registering
class PushNotifier extends Notifier<PushState> {
  @override
  PushState build() => const PushState();

  ApiClient get _apiClient => ref.read(apiClientProvider);

  /// Initializes push notifications.
  ///
  /// Requests permission, gets FCM token, registers with platform,
  /// and sets up token refresh listener.
  Future<void> initialize() async {
    final messaging = FirebaseMessaging.instance;

    // Request permission
    final settings = await messaging.requestPermission(
      alert: true,
      badge: true,
      sound: true,
    );

    if (settings.authorizationStatus != AuthorizationStatus.authorized &&
        settings.authorizationStatus != AuthorizationStatus.provisional) {
      state = state.copyWith(
        error: 'Push notification permission denied',
      );
      dev.log(
        'Push notification permission denied: ${settings.authorizationStatus}',
        name: 'PushProvider',
      );
      return;
    }

    // Get FCM token
    final token = await messaging.getToken();
    if (token != null) {
      await registerToken(token);
    }

    // Listen for token refresh
    messaging.onTokenRefresh.listen(registerToken);

    // iOS foreground notification display options
    await messaging.setForegroundNotificationPresentationOptions(
      alert: true,
      badge: true,
      sound: true,
    );

    dev.log('Push notifications initialized', name: 'PushProvider');
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

      dev.log('FCM token registered', name: 'PushProvider');
    } catch (e) {
      state = state.copyWith(
        error: 'Failed to register push token: $e',
      );
      dev.log('Failed to register push token: $e', name: 'PushProvider');
    }
  }

  /// Detects the current platform for push token registration.
  String _detectPlatform() {
    if (Platform.isIOS) return 'ios';
    if (Platform.isAndroid) return 'android';
    return 'unknown';
  }
}

/// Provider for push notification state.
final pushProvider = NotifierProvider<PushNotifier, PushState>(
  PushNotifier.new,
);
