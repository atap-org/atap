import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/inbox/inbox_screen.dart';
import '../features/inbox/signal_detail_screen.dart';
import '../features/onboarding/claim_screen.dart';
import '../features/onboarding/register_screen.dart';
import '../providers/auth_provider.dart';

/// GoRouter configuration with deep link support for claim URLs.
///
/// Routes:
/// - /onboarding: initial screen for unauthenticated users
/// - /claim/:code: deep link handler for claim codes
/// - /register: human registration form
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
      final isRegister = state.matchedLocation == '/register';

      // Allow claim deep links and registration regardless of auth state
      if (isClaim || isRegister) return null;

      // Redirect to inbox if authenticated and on onboarding
      if (isAuthenticated && isOnboarding) return '/inbox';

      // Redirect to onboarding if not authenticated
      if (!isAuthenticated && !isOnboarding) return '/onboarding';

      return null;
    },
    routes: [
      GoRoute(
        path: '/onboarding',
        builder: (context, state) => const _OnboardingScreen(),
      ),
      GoRoute(
        path: '/claim/:code',
        builder: (context, state) {
          final code = state.pathParameters['code'] ?? '';
          return ClaimScreen(claimCode: code);
        },
      ),
      GoRoute(
        path: '/register',
        builder: (context, state) {
          final claimCode = state.extra as String? ?? '';
          return RegisterScreen(claimCode: claimCode);
        },
      ),
      GoRoute(
        path: '/inbox',
        builder: (context, state) => const InboxScreen(),
        routes: [
          GoRoute(
            path: ':signalId',
            builder: (context, state) {
              final signalId = state.pathParameters['signalId'] ?? '';
              return SignalDetailScreen(signalId: signalId);
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

/// Onboarding screen for unauthenticated users.
class _OnboardingScreen extends StatelessWidget {
  const _OnboardingScreen();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Welcome to ATAP')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.security,
                size: 80,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(height: 24),
              Text(
                'Agent Trust and Authority Protocol',
                style: theme.textTheme.headlineSmall,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 12),
              Text(
                'Scan a claim link or enter a claim code to get started.',
                style: theme.textTheme.bodyLarge?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Placeholder screen for routes not yet implemented.
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
