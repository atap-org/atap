import 'dart:convert';
import 'dart:typed_data';

import 'package:atap/core/crypto/ed25519_service.dart';
import 'package:flutter_test/flutter_test.dart';

/// Cross-language Ed25519 compatibility tests.
///
/// These test vectors are shared with the Go implementation in
/// platform/internal/crypto/crypto_test.go (TestDeriveHumanIDKnownVector
/// and TestSignRequestKnownVector). Both must produce identical results.
void main() {
  // Fixed 32-byte seed: 0x00..0x1f (shared with Go test vectors)
  final seed = Uint8List.fromList(
    List.generate(32, (i) => i),
  );

  // Expected values from Go implementation
  const expectedPubHex =
      '03a107bff3ce10be1d70dd18e74bc09967e4d6309ba50d5f1ddc8664125531b8';
  const expectedHumanID = 'kzdvvj2umnduyauf';
  const expectedSigB64 =
      '1ERwmMB-ThYieQXMTZ4naGuIvroq9kYQ6Jn2TV7OGrSCmoWrmG2ThsteyTL98zzR2bAkPD2GLW0F1I7aE17sBg';

  group('Ed25519 cross-language compatibility', () {
    late Uint8List publicKey;
    late Uint8List privateKey;

    setUp(() {
      final keyPair = Ed25519Service.generateKeyPairFromSeed(seed);
      publicKey = keyPair.publicKey;
      privateKey = keyPair.privateKey;
    });

    test('deterministic keypair from seed matches Go output', () {
      final pubHex = _hexEncode(publicKey);
      expect(pubHex, equals(expectedPubHex));
    });

    test('deriveHumanID matches Go DeriveHumanID', () {
      final humanID = Ed25519Service.deriveHumanID(publicKey);
      expect(humanID, equals(expectedHumanID));
      expect(humanID.length, equals(16));
      expect(humanID, equals(humanID.toLowerCase()));
    });

    test('sign and verify round-trip', () {
      final message = Uint8List.fromList(utf8.encode('hello atap'));
      final signature = Ed25519Service.sign(privateKey, message);
      expect(Ed25519Service.verify(publicKey, message, signature), isTrue);
    });

    test('signature of known payload matches Go output', () {
      final payload = 'GET /v1/health 2024-01-01T00:00:00Z';
      final message = Uint8List.fromList(utf8.encode(payload));
      final signature = Ed25519Service.sign(privateKey, message);

      // Base64url encode without padding (match Go RawURLEncoding)
      final sigB64 = base64Url.encode(signature).replaceAll('=', '');
      expect(sigB64, equals(expectedSigB64));

      // Also verify the signature is valid
      expect(Ed25519Service.verify(publicKey, message, signature), isTrue);
    });

    test('signRequest produces correctly formatted auth headers', () {
      final timestamp = DateTime.utc(2024, 1, 1, 0, 0, 0);
      final headers = Ed25519Service.signRequest(
        privateKey,
        'key_test_1234',
        'GET',
        '/v1/health',
        timestamp,
      );

      expect(headers.containsKey('authorization'), isTrue);
      expect(headers.containsKey('x-atap-timestamp'), isTrue);
      expect(headers['x-atap-timestamp'], equals('2024-01-01T00:00:00Z'));

      final auth = headers['authorization']!;
      expect(auth, startsWith('Signature '));
      expect(auth, contains('keyId="key_test_1234"'));
      expect(auth, contains('algorithm="ed25519"'));
      expect(auth, contains('headers="(request-target) x-atap-timestamp"'));
      expect(auth, contains('signature="'));

      // Verify the signature matches the Go-produced value
      expect(auth, contains('signature="$expectedSigB64"'));
    });

    test('wrong key fails verification', () {
      final otherKeyPair = Ed25519Service.generateKeyPair();
      final message = Uint8List.fromList(utf8.encode('hello'));
      final signature = Ed25519Service.sign(privateKey, message);
      expect(
        Ed25519Service.verify(otherKeyPair.publicKey, message, signature),
        isFalse,
      );
    });

    test('random keypair generation works', () {
      final keyPair = Ed25519Service.generateKeyPair();
      expect(keyPair.publicKey.length, equals(32));
      expect(keyPair.privateKey.length, equals(64));

      // Sign and verify with random keypair
      final message = Uint8List.fromList(utf8.encode('test message'));
      final sig = Ed25519Service.sign(keyPair.privateKey, message);
      expect(Ed25519Service.verify(keyPair.publicKey, message, sig), isTrue);
    });
  });
}

/// Encodes bytes to lowercase hex string.
String _hexEncode(Uint8List bytes) {
  return bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
}
