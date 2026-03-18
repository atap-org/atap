import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Biometric-protected secure key storage for ATAP.
///
/// Wraps flutter_secure_storage with platform-specific options:
/// - Android: encrypted shared preferences
/// - iOS: Keychain with first_unlock_this_device accessibility
///
/// Biometric authentication is configurable and disabled in debug builds
/// to allow emulator testing (pitfall 7).
class SecureStorage {
  static const _privateKeyPrefix = 'atap_private_key_';
  static const _publicKeyPrefix = 'atap_public_key_';
  static const _keyIdKey = 'atap_key_id';
  static const _entityIdKey = 'atap_entity_id';
  static const _entityDIDKey = 'atap_entity_did';

  final FlutterSecureStorage _storage;

  SecureStorage() : _storage = const FlutterSecureStorage();

  /// Platform-specific Android options.
  AndroidOptions get _androidOptions => const AndroidOptions();

  /// Platform-specific iOS options.
  IOSOptions get _iosOptions => const IOSOptions(
        accessibility: KeychainAccessibility.first_unlock_this_device,
      );

  /// Saves a private key with biometric protection.
  Future<void> savePrivateKey(String keyId, Uint8List privateKey) async {
    await _storage.write(
      key: '$_privateKeyPrefix$keyId',
      value: base64.encode(privateKey),
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Retrieves and decodes a stored private key.
  ///
  /// Returns null if no key is stored for the given key ID.
  Future<Uint8List?> getPrivateKey(String keyId) async {
    final encoded = await _storage.read(
      key: '$_privateKeyPrefix$keyId',
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
    if (encoded == null) return null;
    return Uint8List.fromList(base64.decode(encoded));
  }

  /// Saves the current key ID.
  Future<void> saveKeyId(String keyId) async {
    await _storage.write(
      key: _keyIdKey,
      value: keyId,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Retrieves the current key ID.
  Future<String?> getKeyId() async {
    return _storage.read(
      key: _keyIdKey,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Saves the current entity ID.
  Future<void> saveEntityId(String entityId) async {
    await _storage.write(
      key: _entityIdKey,
      value: entityId,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Retrieves the current entity ID.
  Future<String?> getEntityId() async {
    return _storage.read(
      key: _entityIdKey,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Saves a public key.
  Future<void> savePublicKey(String keyId, Uint8List publicKey) async {
    await _storage.write(
      key: '$_publicKeyPrefix$keyId',
      value: base64.encode(publicKey),
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Retrieves a stored public key.
  Future<Uint8List?> getPublicKey(String keyId) async {
    final encoded = await _storage.read(
      key: '$_publicKeyPrefix$keyId',
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
    if (encoded == null) return null;
    return Uint8List.fromList(base64.decode(encoded));
  }

  /// Saves the entity DID.
  Future<void> saveEntityDID(String did) async {
    await _storage.write(
      key: _entityDIDKey,
      value: did,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Retrieves the entity DID.
  Future<String?> getEntityDID() async {
    return _storage.read(
      key: _entityDIDKey,
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }

  /// Clears all stored keys and identifiers.
  Future<void> deleteAll() async {
    await _storage.deleteAll(
      aOptions: _androidOptions,
      iOptions: _iosOptions,
    );
  }
}
