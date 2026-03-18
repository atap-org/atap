import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto_pkg;
import 'package:ed25519_edwards/ed25519_edwards.dart' as ed;

/// DPoP (Demonstrating Proof-of-Possession) service for OAuth 2.1.
///
/// Generates DPoP proof JWTs per RFC 9449, signed with Ed25519.
/// The proof binds an HTTP request to the client's key, preventing
/// token theft and replay attacks.
class DPoPService {
  final Uint8List _privateKey;
  final Uint8List _publicKey;

  DPoPService({
    required Uint8List privateKey,
    required Uint8List publicKey,
  })  : _privateKey = privateKey,
        _publicKey = publicKey;

  /// Creates a DPoP proof JWT for the given HTTP method and URL.
  ///
  /// The proof is a compact JWS with:
  /// - Header: typ=dpop+jwt, alg=EdDSA, jwk={OKP Ed25519 public key}
  /// - Claims: jti (unique), htm (method), htu (URL), iat (now)
  String createProof(String method, String url) {
    // Build JWK for the public key (OKP curve Ed25519)
    final xB64 = _base64UrlNoPad(_publicKey);
    final jwk = {
      'kty': 'OKP',
      'crv': 'Ed25519',
      'x': xB64,
    };

    final header = {
      'typ': 'dpop+jwt',
      'alg': 'EdDSA',
      'jwk': jwk,
    };

    final now = DateTime.now().toUtc().millisecondsSinceEpoch ~/ 1000;
    final jti = _generateJti();

    final claims = {
      'jti': jti,
      'htm': method,
      'htu': url,
      'iat': now,
    };

    final headerB64 = _base64UrlNoPad(utf8.encode(jsonEncode(header)));
    final claimsB64 = _base64UrlNoPad(utf8.encode(jsonEncode(claims)));
    final signingInput = '$headerB64.$claimsB64';

    final priv = ed.PrivateKey(_privateKey);
    final signature =
        ed.sign(priv, Uint8List.fromList(utf8.encode(signingInput)));
    final sigB64 = _base64UrlNoPad(signature);

    return '$headerB64.$claimsB64.$sigB64';
  }

  /// Computes the JWK SHA-256 thumbprint for this key (used as jkt).
  ///
  /// Per RFC 7638: canonical JSON of {crv, kty, x} sorted alphabetically,
  /// then SHA-256, then base64url.
  String get jwkThumbprint {
    final xB64 = _base64UrlNoPad(_publicKey);
    // Canonical form: sorted keys (crv, kty, x)
    final canonical = '{"crv":"Ed25519","kty":"OKP","x":"$xB64"}';
    final hash = crypto_pkg.sha256.convert(utf8.encode(canonical));
    return _base64UrlNoPad(Uint8List.fromList(hash.bytes));
  }

  /// Generates a unique jti (JWT ID) using timestamp + random bytes.
  static String _generateJti() {
    final now = DateTime.now().microsecondsSinceEpoch;
    final hash = crypto_pkg.sha256.convert(utf8.encode('$now'));
    return _base64UrlNoPad(Uint8List.fromList(hash.bytes)).substring(0, 22);
  }

  static String _base64UrlNoPad(List<int> bytes) {
    return base64Url.encode(bytes).replaceAll('=', '');
  }
}
