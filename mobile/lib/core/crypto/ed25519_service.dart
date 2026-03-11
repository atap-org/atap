import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto_pkg;
import 'package:ed25519_edwards/ed25519_edwards.dart' as ed;

/// Ed25519 cryptographic service for ATAP.
///
/// Provides key generation, signing, verification, human ID derivation,
/// and HTTP request signing compatible with the Go platform.
class Ed25519Service {
  /// Standard base32 alphabet (RFC 4648).
  static const _base32Alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

  /// Generates a new Ed25519 keypair.
  ///
  /// Returns a record of (publicKey, privateKey) as raw byte arrays.
  static ({Uint8List publicKey, Uint8List privateKey}) generateKeyPair() {
    final keyPair = ed.generateKey();
    return (
      publicKey: Uint8List.fromList(keyPair.publicKey.bytes),
      privateKey: Uint8List.fromList(keyPair.privateKey.bytes),
    );
  }

  /// Generates a keypair from a known 32-byte seed (deterministic).
  ///
  /// Used for testing cross-language compatibility.
  static ({Uint8List publicKey, Uint8List privateKey}) generateKeyPairFromSeed(
    Uint8List seed,
  ) {
    final privateKey = ed.newKeyFromSeed(seed);
    final publicKey = ed.public(privateKey);
    return (
      publicKey: Uint8List.fromList(publicKey.bytes),
      privateKey: Uint8List.fromList(privateKey.bytes),
    );
  }

  /// Signs a message with an Ed25519 private key.
  static Uint8List sign(Uint8List privateKey, Uint8List message) {
    final priv = ed.PrivateKey(privateKey);
    final sig = ed.sign(priv, message);
    return Uint8List.fromList(sig);
  }

  /// Verifies an Ed25519 signature.
  static bool verify(
    Uint8List publicKey,
    Uint8List message,
    Uint8List signature,
  ) {
    final pub = ed.PublicKey(publicKey);
    return ed.verify(pub, message, signature);
  }

  /// Derives a human entity ID from an Ed25519 public key.
  ///
  /// Formula: lowercase(base32(sha256(pubkey))[:16])
  /// Must produce identical output to Go's crypto.DeriveHumanID.
  static String deriveHumanID(Uint8List publicKeyBytes) {
    final hash = crypto_pkg.sha256.convert(publicKeyBytes);
    final encoded = _base32Encode(Uint8List.fromList(hash.bytes));
    return encoded.substring(0, 16).toLowerCase();
  }

  /// Signs an HTTP request for Ed25519 authentication.
  ///
  /// Returns a map with 'authorization' and 'x-atap-timestamp' headers.
  /// The signed payload is: "{METHOD} {path} {timestamp_rfc3339}"
  ///
  /// Signature is base64url encoded WITHOUT padding (matches Go's
  /// base64.RawURLEncoding).
  static Map<String, String> signRequest(
    Uint8List privateKey,
    String keyId,
    String method,
    String path,
    DateTime timestamp,
  ) {
    final ts = _formatRfc3339(timestamp.toUtc());
    final payload = '$method $path $ts';
    final sig = sign(privateKey, Uint8List.fromList(utf8.encode(payload)));
    final sigB64 = base64Url.encode(sig).replaceAll('=', '');
    final authHeader =
        'Signature keyId="$keyId",algorithm="ed25519",'
        'headers="(request-target) x-atap-timestamp",'
        'signature="$sigB64"';
    return {
      'authorization': authHeader,
      'x-atap-timestamp': ts,
    };
  }

  /// Formats a DateTime as RFC 3339 (ISO 8601) UTC string.
  ///
  /// Go's time.RFC3339 format: "2006-01-02T15:04:05Z07:00"
  /// For UTC, this produces: "2024-01-01T00:00:00Z"
  static String _formatRfc3339(DateTime dt) {
    final utc = dt.toUtc();
    final year = utc.year.toString().padLeft(4, '0');
    final month = utc.month.toString().padLeft(2, '0');
    final day = utc.day.toString().padLeft(2, '0');
    final hour = utc.hour.toString().padLeft(2, '0');
    final minute = utc.minute.toString().padLeft(2, '0');
    final second = utc.second.toString().padLeft(2, '0');
    return '${year}-${month}-${day}T${hour}:${minute}:${second}Z';
  }

  /// Standard base32 encoding (RFC 4648) with padding.
  ///
  /// Matches Go's encoding/base32.StdEncoding.EncodeToString.
  static String _base32Encode(Uint8List data) {
    final buffer = StringBuffer();
    var bits = 0;
    var value = 0;

    for (final byte in data) {
      value = (value << 8) | byte;
      bits += 8;
      while (bits >= 5) {
        bits -= 5;
        buffer.write(_base32Alphabet[(value >> bits) & 0x1F]);
      }
    }

    // Handle remaining bits
    if (bits > 0) {
      buffer.write(_base32Alphabet[(value << (5 - bits)) & 0x1F]);
    }

    // Add padding to make length a multiple of 8
    while (buffer.length % 8 != 0) {
      buffer.write('=');
    }

    return buffer.toString();
  }
}
