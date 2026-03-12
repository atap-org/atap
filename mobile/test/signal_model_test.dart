import 'package:flutter_test/flutter_test.dart';
import 'package:atap/core/models/signal.dart';

void main() {
  group('Signal.fromJson', () {
    test('parses ts field as createdAt', () {
      final json = {
        'id': 'sig_01TESTID',
        'route': {'origin': 'agent://a', 'target': 'agent://b'},
        'signal': {'type': 'test', 'data': {}},
        'ts': '2026-03-12T00:00:00.000Z',
      };
      final signal = Signal.fromJson(json);
      expect(signal.createdAt, DateTime.utc(2026, 3, 12));
    });

    test('toJson emits ts field', () {
      final signal = Signal(
        id: 'sig_01TESTID',
        route: const SignalRoute(origin: 'agent://a', target: 'agent://b'),
        signal: const SignalBody(type: 'test'),
        createdAt: DateTime.utc(2026, 3, 12),
      );
      final json = signal.toJson();
      expect(json.containsKey('ts'), isTrue);
      expect(json.containsKey('created_at'), isFalse);
    });

    test('round-trips through fromJson/toJson', () {
      final original = {
        'id': 'sig_01ROUNDTRIP',
        'route': {'origin': 'agent://x', 'target': 'agent://y'},
        'signal': {'type': 'ping', 'data': {'value': 42}},
        'ts': '2026-01-15T10:30:00.000Z',
      };
      final signal = Signal.fromJson(original);
      final output = signal.toJson();
      expect(output['ts'], '2026-01-15T10:30:00.000Z');
      expect(output['id'], 'sig_01ROUNDTRIP');
    });
  });
}
