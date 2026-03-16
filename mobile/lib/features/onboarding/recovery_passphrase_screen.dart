import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto_pkg;
import 'package:encrypt/encrypt.dart' as encrypt_pkg;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:go_router/go_router.dart';

import '../../providers/auth_provider.dart';

/// Recovery passphrase screen shown after DID registration.
///
/// Encrypts the Ed25519 private key with a PBKDF2-derived AES-256 key
/// and stores the encrypted backup in secure storage. This enables future
/// key recovery if the device is lost.
class RecoveryPassphraseScreen extends ConsumerStatefulWidget {
  const RecoveryPassphraseScreen({super.key});

  @override
  ConsumerState<RecoveryPassphraseScreen> createState() =>
      _RecoveryPassphraseScreenState();
}

class _RecoveryPassphraseScreenState
    extends ConsumerState<RecoveryPassphraseScreen> {
  final _formKey = GlobalKey<FormState>();
  final _passphraseController = TextEditingController();
  final _confirmController = TextEditingController();
  bool _isProcessing = false;
  String? _error;
  bool _obscurePassphrase = true;
  bool _obscureConfirm = true;

  @override
  void dispose() {
    _passphraseController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  Future<void> _setPassphrase() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isProcessing = true;
      _error = null;
    });

    try {
      final storage = ref.read(secureStorageProvider);
      final keyId = await storage.getKeyId();
      if (keyId == null) throw StateError('No key ID found');

      final privateKey = await storage.getPrivateKey(keyId);
      if (privateKey == null) throw StateError('No private key found');

      // Generate random salt
      final random = Random.secure();
      final salt = Uint8List(32);
      for (var i = 0; i < 32; i++) {
        salt[i] = random.nextInt(256);
      }

      // Derive AES-256 key from passphrase using PBKDF2
      final passphrase = _passphraseController.text;
      final derivedKey = _deriveKey(passphrase, salt);

      // Encrypt private key with AES-256-CBC
      final iv = encrypt_pkg.IV.fromSecureRandom(16);
      final encrypter = encrypt_pkg.Encrypter(
        encrypt_pkg.AES(encrypt_pkg.Key(derivedKey)),
      );
      final encrypted = encrypter.encryptBytes(privateKey, iv: iv);

      // Store encrypted backup: salt + iv + ciphertext
      final backup = {
        'salt': base64.encode(salt),
        'iv': base64.encode(iv.bytes),
        'ciphertext': encrypted.base64,
      };

      const secureStorage = FlutterSecureStorage();
      await secureStorage.write(
        key: 'recovery_backup',
        value: jsonEncode(backup),
      );
      await secureStorage.write(
        key: 'has_recovery_passphrase',
        value: 'true',
      );

      if (mounted) {
        context.go('/inbox');
      }
    } catch (e) {
      setState(() {
        _error = 'Failed to set passphrase: $e';
        _isProcessing = false;
      });
    }
  }

  /// Derives a 32-byte AES key from passphrase + salt using PBKDF2-HMAC-SHA256.
  Uint8List _deriveKey(String passphrase, Uint8List salt) {
    // PBKDF2 with 100,000 iterations
    final passphraseBytes = utf8.encode(passphrase);
    var block = Uint8List(0);
    var result = Uint8List(32);

    // PBKDF2-HMAC-SHA256 manual implementation
    final hmac = crypto_pkg.Hmac(crypto_pkg.sha256, passphraseBytes);

    // Block 1 (we only need 32 bytes = 1 block for SHA-256)
    final blockInput = Uint8List(salt.length + 4);
    blockInput.setAll(0, salt);
    blockInput[salt.length + 3] = 1; // block index = 1

    block = Uint8List.fromList(hmac.convert(blockInput).bytes);
    result = Uint8List.fromList(block);

    for (var i = 1; i < 100000; i++) {
      block = Uint8List.fromList(hmac.convert(block).bytes);
      for (var j = 0; j < 32; j++) {
        result[j] ^= block[j];
      }
    }

    return result;
  }

  Future<void> _skip() async {
    const secureStorage = FlutterSecureStorage();
    await secureStorage.write(
      key: 'has_recovery_passphrase',
      value: 'false',
    );
    if (mounted) {
      context.go('/inbox');
    }
  }

  String? _validatePassphrase(String? value) {
    if (value == null || value.length < 12) {
      return 'Passphrase must be at least 12 characters';
    }
    return null;
  }

  String? _validateConfirm(String? value) {
    if (value != _passphraseController.text) {
      return 'Passphrases do not match';
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Recovery Passphrase')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Icon(
                Icons.lock_outline,
                size: 64,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(height: 16),
              Text(
                'Protect Your Identity',
                style: theme.textTheme.headlineSmall,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                'Set a recovery passphrase to encrypt a backup of your private key. '
                'If you lose this device, you can recover your identity with this passphrase.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              TextFormField(
                controller: _passphraseController,
                obscureText: _obscurePassphrase,
                decoration: InputDecoration(
                  labelText: 'Passphrase',
                  hintText: 'At least 12 characters',
                  prefixIcon: const Icon(Icons.key),
                  suffixIcon: IconButton(
                    icon: Icon(_obscurePassphrase
                        ? Icons.visibility_off
                        : Icons.visibility),
                    onPressed: () =>
                        setState(() => _obscurePassphrase = !_obscurePassphrase),
                  ),
                  border: const OutlineInputBorder(),
                ),
                enabled: !_isProcessing,
                validator: _validatePassphrase,
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _confirmController,
                obscureText: _obscureConfirm,
                decoration: InputDecoration(
                  labelText: 'Confirm Passphrase',
                  prefixIcon: const Icon(Icons.key),
                  suffixIcon: IconButton(
                    icon: Icon(_obscureConfirm
                        ? Icons.visibility_off
                        : Icons.visibility),
                    onPressed: () =>
                        setState(() => _obscureConfirm = !_obscureConfirm),
                  ),
                  border: const OutlineInputBorder(),
                ),
                enabled: !_isProcessing,
                validator: _validateConfirm,
              ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Card(
                  color: theme.colorScheme.errorContainer,
                  child: Padding(
                    padding: const EdgeInsets.all(12.0),
                    child: Text(
                      _error!,
                      style: TextStyle(
                        color: theme.colorScheme.onErrorContainer,
                      ),
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _isProcessing ? null : _setPassphrase,
                child: _isProcessing
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Text('Set Passphrase'),
              ),
              const SizedBox(height: 12),
              TextButton(
                onPressed: _isProcessing ? null : _skip,
                child: const Text('Skip for now'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
