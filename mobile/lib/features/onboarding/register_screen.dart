import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/api/api_client.dart';
import '../../providers/auth_provider.dart';

/// Registration screen for creating a human entity.
///
/// Receives a claim code from navigation, collects email,
/// generates an Ed25519 keypair on device, registers with the
/// platform, and stores the private key in biometric-protected
/// secure storage.
class RegisterScreen extends ConsumerStatefulWidget {
  final String claimCode;

  const RegisterScreen({super.key, required this.claimCode});

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  bool _isRegistering = false;
  String? _error;

  @override
  void dispose() {
    _emailController.dispose();
    super.dispose();
  }

  Future<void> _register() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _isRegistering = true;
      _error = null;
    });

    try {
      await ref.read(authProvider.notifier).register(
            email: _emailController.text.trim(),
            claimCode: widget.claimCode,
          );

      // Check if registration succeeded
      final authState = ref.read(authProvider);
      if (authState.isAuthenticated) {
        if (mounted) {
          context.go('/inbox');
        }
      } else if (authState.error != null) {
        setState(() {
          _error = _formatError(authState.error!);
          _isRegistering = false;
        });
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

  /// Formats API error messages for display.
  String _formatError(String error) {
    if (error.contains('already redeemed')) {
      return 'This claim has already been used.';
    }
    if (error.contains('expired')) {
      return 'This claim has expired.';
    }
    if (error.contains('not found')) {
      return 'Invalid claim code.';
    }
    return error;
  }

  String? _validateEmail(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Email is required';
    }
    if (!value.contains('@')) {
      return 'Enter a valid email address';
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Create Account'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Icon(
                Icons.person_add,
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
              const SizedBox(height: 8),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(12.0),
                  child: Row(
                    children: [
                      Icon(Icons.confirmation_number,
                          color: theme.colorScheme.primary),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('Claim Code',
                                style: theme.textTheme.labelSmall),
                            Text(widget.claimCode,
                                style: theme.textTheme.bodyLarge),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 24),
              TextFormField(
                controller: _emailController,
                decoration: const InputDecoration(
                  labelText: 'Email',
                  hintText: 'you@example.com',
                  prefixIcon: Icon(Icons.email_outlined),
                  border: OutlineInputBorder(),
                ),
                keyboardType: TextInputType.emailAddress,
                autocorrect: false,
                enabled: !_isRegistering,
                validator: _validateEmail,
              ),
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
                    : const Text('Register'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
