import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/auth_provider.dart';

/// GoRouter configuration with deep link support for claim URLs.
///
/// Routes:
/// - /onboarding: initial screen for unauthenticated users
/// - /claim/:code: deep link handler for claim codes
/// - /inbox: main inbox view (authenticated)
/// - /inbox/:signalId: signal detail view
/// - /settings: app settings
final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authProvider);

  return GoRouter(
    initialLocation: '/onboarding',
    redirect: (context, state) {
      final isAuthenticated = authState.isAuthenticated;
      final isOnboarding = state.matchedLocation == '/onboarding';
      final isClaim = state.matchedLocation.startsWith('/claim');

      // Allow claim deep links regardless of auth state
      if (isClaim) return null;

      // Redirect to inbox if authenticated and on onboarding
      if (isAuthenticated && isOnboarding) return '/inbox';

      // Redirect to onboarding if not authenticated
      if (!isAuthenticated && !isOnboarding) return '/onboarding';

      return null;
    },
    routes: [
      GoRoute(
        path: '/onboarding',
        builder: (context, state) => const _PlaceholderScreen(
          title: 'Welcome to ATAP',
          message: 'Scan a claim link to get started.',
        ),
      ),
      GoRoute(
        path: '/claim/:code',
        builder: (context, state) {
          final code = state.pathParameters['code'] ?? '';
          return _PlaceholderScreen(
            title: 'Claim',
            message: 'Claiming: $code',
          );
        },
      ),
      GoRoute(
        path: '/inbox',
        builder: (context, state) => const _PlaceholderScreen(
          title: 'Inbox',
          message: 'Your signals will appear here.',
        ),
        routes: [
          GoRoute(
            path: ':signalId',
            builder: (context, state) {
              final signalId = state.pathParameters['signalId'] ?? '';
              return _PlaceholderScreen(
                title: 'Signal',
                message: 'Signal: $signalId',
              );
            },
          ),
        ],
      ),
      GoRoute(
        path: '/settings',
        builder: (context, state) => const _PlaceholderScreen(
          title: 'Settings',
          message: 'App settings.',
        ),
      ),
    ],
  );
});

/// Placeholder screen used during initial app shell setup.
///
/// Will be replaced with actual feature screens in subsequent plans.
class _PlaceholderScreen extends StatelessWidget {
  final String title;
  final String message;

  const _PlaceholderScreen({
    required this.title,
    required this.message,
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Text(
            message,
            style: Theme.of(context).textTheme.bodyLarge,
            textAlign: TextAlign.center,
          ),
        ),
      ),
    );
  }
}
