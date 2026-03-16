import 'dart:convert';
import 'dart:typed_data';

import 'package:ed25519_edwards/ed25519_edwards.dart' as ed;

/// JWS (JSON Web Signature) service for approval response signing.
///
/// Produces JWS Compact Serialization with detached payload (RFC 7797):
/// base64url(header) + ".." + base64url(signature)
class JwsService {
  /// Creates a detached JWS signature over the given payload.
  ///
  /// The header includes `alg: EdDSA` and `kid` pointing to the entity's
  /// DID key. The payload is NOT included in the JWS (detached).
  static String signDetached(
    Uint8List privateKey,
    String payload, {
    required String kid,
  }) {
    final header = {'alg': 'EdDSA', 'kid': kid};
    final headerB64 = _base64UrlNoPad(utf8.encode(jsonEncode(header)));
    final payloadB64 = _base64UrlNoPad(utf8.encode(payload));

    // Sign: base64url(header) + "." + base64url(payload)
    final signingInput = '$headerB64.$payloadB64';
    final priv = ed.PrivateKey(privateKey);
    final signature = ed.sign(priv, Uint8List.fromList(utf8.encode(signingInput)));

    final sigB64 = _base64UrlNoPad(signature);

    // Detached: header + ".." + signature (empty payload segment)
    return '$headerB64..$sigB64';
  }

  /// Verifies a detached JWS signature against a known payload.
  static bool verifyDetached(
    Uint8List publicKey,
    String jws,
    String payload,
  ) {
    final parts = jws.split('..');
    if (parts.length != 2) return false;

    final headerB64 = parts[0];
    final sigB64 = parts[1];
    final payloadB64 = _base64UrlNoPad(utf8.encode(payload));

    final signingInput = '$headerB64.$payloadB64';
    final signature = _base64UrlDecode(sigB64);

    final pub = ed.PublicKey(publicKey);
    return ed.verify(pub, Uint8List.fromList(utf8.encode(signingInput)), signature);
  }

  /// Base64url encode without padding.
  static String _base64UrlNoPad(List<int> bytes) {
    return base64Url.encode(bytes).replaceAll('=', '');
  }

  /// Base64url decode with optional padding restoration.
  static Uint8List _base64UrlDecode(String input) {
    var padded = input;
    final remainder = padded.length % 4;
    if (remainder != 0) {
      padded = padded.padRight(padded.length + (4 - remainder), '=');
    }
    return base64Url.decode(padded);
  }
}
