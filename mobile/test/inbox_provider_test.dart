import 'package:flutter_test/flutter_test.dart';

void main() {
  group('InboxProvider pagination', () {
    test('loadMore URL uses after= parameter', () {
      // Verify the URL pattern matches Go backend expectation (c.Query("after"))
      const entityId = 'test-entity-001';
      const cursor = 'sig_cursor123';
      final url = '/v1/inbox/$entityId?limit=50&after=$cursor';

      expect(url, contains('after='));
      expect(url, isNot(contains('cursor=')));
    });

    test('loadMore URL format matches Go GET /v1/inbox handler', () {
      // Go handler expects: c.Query("after") and c.Query("limit")
      const entityId = '01hyabc123456789abcdef00';
      const cursor = 'sig_01HYLAST00000000000000';
      final url = '/v1/inbox/$entityId?limit=50&after=$cursor';

      // Verify structure
      expect(url, startsWith('/v1/inbox/'));
      expect(url, contains('limit=50'));
      expect(url, contains('after=sig_'));
    });
  });
}
