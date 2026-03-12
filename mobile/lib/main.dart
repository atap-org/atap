import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/router.dart';
import 'app/theme.dart';
import 'providers/auth_provider.dart';

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
