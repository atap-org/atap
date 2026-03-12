import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/router.dart';
import 'app/theme.dart';
import 'providers/auth_provider.dart';
import 'providers/push_provider.dart';

@pragma('vm:entry-point')
Future<void> _firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  await Firebase.initializeApp();
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp();
  FirebaseMessaging.onBackgroundMessage(_firebaseMessagingBackgroundHandler);
  runApp(const ProviderScope(child: AtapApp()));
}

/// Root application widget.
///
/// Wraps MaterialApp.router with GoRouter and Riverpod state management.
/// Initializes secure storage and checks for existing auth state on startup.
class AtapApp extends ConsumerStatefulWidget {
  const AtapApp({super.key});

  @override
  ConsumerState<AtapApp> createState() => _AtapAppState();
}

class _AtapAppState extends ConsumerState<AtapApp> {
  @override
  void initState() {
    super.initState();
    // Check for existing auth state on startup
    Future.microtask(() {
      ref.read(authProvider.notifier).loadSavedAuth();
    });

    // Initialize push notifications after auth loads
    Future.microtask(() {
      ref.read(pushProvider.notifier).initialize();
    });

    // Cold start: check if app was opened from a notification tap
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final initialMessage =
          await FirebaseMessaging.instance.getInitialMessage();
      if (initialMessage != null) {
        _handleNotificationTap(initialMessage);
      }
    });

    // Background resume: notification tap while app was in background
    FirebaseMessaging.onMessageOpenedApp.listen(_handleNotificationTap);

    // Foreground: show in-app SnackBar when signal arrives
    FirebaseMessaging.onMessage.listen(_handleForegroundMessage);
  }

  void _handleNotificationTap(RemoteMessage message) {
    final signalId = message.data['signal_id'];
    if (signalId != null) {
      final router = ref.read(routerProvider);
      router.go('/inbox/$signalId');
    }
  }

  void _handleForegroundMessage(RemoteMessage message) {
    final notification = message.notification;
    if (notification == null) return;

    final signalId = message.data['signal_id'];
    final messenger = ScaffoldMessenger.maybeOf(context);
    if (messenger == null) return;

    messenger.showSnackBar(
      SnackBar(
        content: Text(
          '${notification.title ?? "Signal"}: ${notification.body ?? ""}',
        ),
        action: signalId != null
            ? SnackBarAction(
                label: 'View',
                onPressed: () {
                  final router = ref.read(routerProvider);
                  router.go('/inbox/$signalId');
                },
              )
            : null,
        duration: const Duration(seconds: 4),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final router = ref.watch(routerProvider);

    return MaterialApp.router(
      title: 'ATAP',
      theme: AtapTheme.lightTheme,
      darkTheme: AtapTheme.darkTheme,
      routerConfig: router,
      debugShowCheckedModeBanner: false,
    );
  }
}
