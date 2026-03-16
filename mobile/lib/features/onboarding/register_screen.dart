import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/api/api_client.dart';
import '../../core/crypto/ed25519_service.dart';
import '../../providers/auth_provider.dart';

/// Registration screen for creating a human entity.
///
/// Generates an Ed25519 keypair on device, registers with the platform
/// via POST /v1/entities, stores the private key in secure storage,
/// then navigates to the recovery passphrase screen.
class RegisterScreen extends ConsumerStatefulWidget {
  const RegisterScreen({super.key});

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  bool _isRegistering = false;
  String? _error;
  String? _registeredDID;

  Future<void> _register() async {
    setState(() {
      _isRegistering = true;
      _error = null;
    });

    try {
      // Generate keypair on device
      final keyPair = Ed25519Service.generateKeyPair();
      final publicKeyB64 = base64.encode(keyPair.publicKey);

      // Register with platform
      final apiClient = ref.read(apiClientProvider);
      final response = await apiClient.registerEntity(
        type: 'human',
        publicKey: publicKeyB64,
      );

      final entityId = response['id'] as String;
      final keyId = response['key_id'] as String;
      final did = response['did'] as String? ?? '';

      // Store credentials
      final storage = ref.read(secureStorageProvider);
      await storage.savePrivateKey(keyId, keyPair.privateKey);
      await storage.saveKeyId(keyId);
      await storage.saveEntityId(entityId);

      // Set API client credentials
      apiClient.setCredentials(
        privateKey: keyPair.privateKey,
        keyId: keyId,
      );

      // Show DID then navigate to recovery passphrase
      setState(() {
        _registeredDID = did;
        _isRegistering = false;
      });

      // Brief display of DID, then navigate
      await Future.delayed(const Duration(seconds: 2));
      if (mounted) {
        context.go('/recovery-passphrase');
      }
    } on ApiException catch (e) {
      setState(() {
        _error = e.detail ?? e.title;
        _isRegistering = false;
      });
    } catch (e) {
      setState(() {
        _error = 'Registration failed. Please try again.';
        _isRegistering = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Create Identity')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Icon(
              Icons.fingerprint,
              size: 64,
              color: theme.colorScheme.primary,
            ),
            const SizedBox(height: 16),
            Text(
              'Register as Human',
              style: theme.textTheme.headlineSmall,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              'Your identity is secured by an Ed25519 keypair generated on this device. '
              'Your private key never leaves your phone.',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            if (_registeredDID != null) ...[
              const SizedBox(height: 24),
              Card(
                color: theme.colorScheme.primaryContainer,
                child: Padding(
                  padding: const EdgeInsets.all(16.0),
                  child: Column(
                    children: [
                      Icon(Icons.check_circle,
                          size: 48, color: theme.colorScheme.primary),
                      const SizedBox(height: 8),
                      Text('Your DID',
                          style: theme.textTheme.labelLarge),
                      const SizedBox(height: 4),
                      SelectableText(
                        _registeredDID!,
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontFamily: 'monospace',
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Setting up recovery...',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 16),
              Card(
                color: theme.colorScheme.errorContainer,
                child: Padding(
                  padding: const EdgeInsets.all(12.0),
                  child: Row(
                    children: [
                      Icon(Icons.error,
                          color: theme.colorScheme.onErrorContainer),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Text(
                          _error!,
                          style: TextStyle(
                            color: theme.colorScheme.onErrorContainer,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
            if (_registeredDID == null) ...[
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _isRegistering ? null : _register,
                child: _isRegistering
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Text('Generate Identity & Register'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
