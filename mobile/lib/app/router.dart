import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:go_router/go_router.dart';

import '../features/approvals/approvals_screen.dart';
import '../features/credentials/credentials_screen.dart';
import '../features/inbox/inbox_screen.dart';
import '../features/onboarding/recovery_passphrase_screen.dart';
import '../features/onboarding/register_screen.dart';
import '../providers/auth_provider.dart';

/// Listenable that notifies GoRouter when auth state changes,
/// without recreating the entire router.
class _AuthNotifierListenable extends ChangeNotifier {
  _AuthNotifierListenable(this._ref) {
    _ref.listen(authProvider, (_, __) => notifyListeners());
  }
  final Ref _ref;
}

/// GoRouter configuration for DIDComm-based app.
///
/// Routes:
/// - /onboarding: initial screen for unauthenticated users
/// - /register: human registration with keypair generation
/// - /recovery-passphrase: post-registration passphrase setup
/// - /home: main shell with bottom navigation (Inbox, Credentials, Approvals, Settings)
final routerProvider = Provider<GoRouter>((ref) {
  final refreshListenable = _AuthNotifierListenable(ref);

  return GoRouter(
    initialLocation: '/onboarding',
    refreshListenable: refreshListenable,
    redirect: (context, state) {
      final isAuthenticated = ref.read(authProvider).isAuthenticated;
      final isOnboarding = state.matchedLocation == '/onboarding';
      final isRegister = state.matchedLocation == '/register';
      final isRecovery = state.matchedLocation == '/recovery-passphrase';

      // Allow registration and recovery flows
      if (isRegister || isRecovery) return null;

      // Redirect to home if authenticated and on onboarding
      if (isAuthenticated && isOnboarding) return '/home';

      // Redirect to onboarding if not authenticated (except register/recovery)
      if (!isAuthenticated && !isOnboarding) return '/onboarding';

      return null;
    },
    routes: [
      GoRoute(
        path: '/onboarding',
        builder: (context, state) => const _OnboardingScreen(),
      ),
      GoRoute(
        path: '/register',
        builder: (context, state) => const RegisterScreen(),
      ),
      GoRoute(
        path: '/recovery-passphrase',
        builder: (context, state) => const RecoveryPassphraseScreen(),
      ),
      ShellRoute(
        builder: (context, state, child) =>
            _MainShell(child: child),
        routes: [
          GoRoute(
            path: '/home',
            redirect: (context, state) => '/inbox',
          ),
          GoRoute(
            path: '/inbox',
            builder: (context, state) => const InboxScreen(),
          ),
          GoRoute(
            path: '/credentials',
            builder: (context, state) => const CredentialsScreen(),
          ),
          GoRoute(
            path: '/approvals',
            builder: (context, state) => const ApprovalsScreen(),
          ),
          GoRoute(
            path: '/settings',
            builder: (context, state) => const _SettingsScreen(),
          ),
        ],
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
                'Create your cryptographic identity to manage approvals and credentials.',
                style: theme.textTheme.bodyLarge?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 32),
              FilledButton(
                onPressed: () => context.go('/register'),
                child: const Text('Create Identity'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Main shell with bottom navigation.
class _MainShell extends ConsumerWidget {
  final Widget child;

  const _MainShell({required this.child});

  static const _tabs = [
    '/inbox',
    '/credentials',
    '/approvals',
    '/settings',
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final location = GoRouterState.of(context).matchedLocation;
    final currentIndex = _tabs.indexWhere((t) => location.startsWith(t));

    return FutureBuilder<String?>(
      future: const FlutterSecureStorage().read(key: 'has_recovery_passphrase'),
      builder: (context, snapshot) {
        final hasPassphrase = snapshot.data == 'true';

        return Scaffold(
          body: child,
          bottomNavigationBar: NavigationBar(
            selectedIndex: currentIndex >= 0 ? currentIndex : 0,
            onDestinationSelected: (index) {
              context.go(_tabs[index]);
            },
            destinations: [
              const NavigationDestination(
                icon: Icon(Icons.inbox_outlined),
                selectedIcon: Icon(Icons.inbox),
                label: 'Inbox',
              ),
              const NavigationDestination(
                icon: Icon(Icons.verified_outlined),
                selectedIcon: Icon(Icons.verified),
                label: 'Credentials',
              ),
              const NavigationDestination(
                icon: Icon(Icons.handshake_outlined),
                selectedIcon: Icon(Icons.handshake),
                label: 'Approvals',
              ),
              NavigationDestination(
                icon: hasPassphrase
                    ? const Icon(Icons.settings_outlined)
                    : Badge(
                        smallSize: 8,
                        child: const Icon(Icons.settings_outlined),
                      ),
                selectedIcon: const Icon(Icons.settings),
                label: 'Settings',
              ),
            ],
          ),
        );
      },
    );
  }
}

/// Settings screen placeholder.
class _SettingsScreen extends ConsumerWidget {
  const _SettingsScreen();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final authState = ref.watch(authProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        padding: const EdgeInsets.all(16.0),
        children: [
          if (authState.keyId != null)
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16.0),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Identity', style: theme.textTheme.titleSmall),
                    const SizedBox(height: 4),
                    Text(
                      'Key ID: ${authState.keyId}',
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontFamily: 'monospace',
                      ),
                    ),
                  ],
                ),
              ),
            ),
          const SizedBox(height: 16),
          OutlinedButton.icon(
            onPressed: () async {
              await ref.read(authProvider.notifier).logout();
              if (context.mounted) {
                context.go('/onboarding');
              }
            },
            icon: const Icon(Icons.logout),
            label: const Text('Sign Out'),
            style: OutlinedButton.styleFrom(
              foregroundColor: theme.colorScheme.error,
            ),
          ),
        ],
      ),
    );
  }
}
