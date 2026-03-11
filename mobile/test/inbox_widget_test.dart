import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:atap/core/models/signal.dart';
import 'package:atap/features/inbox/inbox_screen.dart';
import 'package:atap/providers/inbox_provider.dart';

void main() {
  group('InboxScreen', () {
    testWidgets('shows empty state when no signals', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            inboxProvider.overrideWith(() => _TestInboxNotifier(
                  const InboxState(signals: [], isLoading: false),
                )),
          ],
          child: const MaterialApp(home: InboxScreen()),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('No signals yet'), findsOneWidget);
      expect(find.text('Pull down to refresh'), findsOneWidget);
    });

    testWidgets('renders signal cards with correct fields', (tester) async {
      final testSignals = [
        Signal(
          id: 'sig_test1',
          route: const SignalRoute(
            origin: 'agent://test-bot',
            target: 'human://alice',
          ),
          signal: const SignalBody(type: 'task.completed', data: {'result': 'ok'}),
          context: const SignalContext(priority: 'high'),
          createdAt: DateTime.now().subtract(const Duration(minutes: 2)),
        ),
        Signal(
          id: 'sig_test2',
          route: const SignalRoute(
            origin: 'agent://notifier',
            target: 'human://alice',
          ),
          signal: const SignalBody(type: 'notification', data: 'hello'),
          context: const SignalContext(priority: 'low'),
          createdAt: DateTime.now().subtract(const Duration(hours: 3)),
        ),
      ];

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            inboxProvider.overrideWith(() => _TestInboxNotifier(
                  InboxState(signals: testSignals, isLoading: false),
                )),
          ],
          child: const MaterialApp(home: InboxScreen()),
        ),
      );
      await tester.pumpAndSettle();

      // Verify signal card content
      expect(find.text('agent://test-bot'), findsOneWidget);
      expect(find.text('task.completed'), findsOneWidget);
      expect(find.text('2m ago'), findsOneWidget);

      expect(find.text('agent://notifier'), findsOneWidget);
      expect(find.text('notification'), findsOneWidget);
      expect(find.text('3h ago'), findsOneWidget);
    });

    testWidgets('shows loading indicator when loading', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            inboxProvider.overrideWith(() => _TestInboxNotifier(
                  const InboxState(
                    signals: [],
                    isLoading: true,
                  ),
                )),
          ],
          child: const MaterialApp(home: InboxScreen()),
        ),
      );
      // Don't settle -- loading indicator is spinning
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });
  });
}

/// Test notifier that returns a fixed InboxState.
class _TestInboxNotifier extends InboxNotifier {
  final InboxState _initialState;

  _TestInboxNotifier(this._initialState);

  @override
  InboxState build() => _initialState;

  @override
  Future<void> loadInbox() async {}

  @override
  void connectSSE() {}

  @override
  void disconnectSSE() {}

  @override
  Future<void> refresh() async {}

  @override
  Future<void> loadMore() async {}
}
