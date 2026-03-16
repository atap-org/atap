import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/models/credential.dart';
import '../../providers/auth_provider.dart';

/// Displays the entity's Verifiable Credentials from GET /v1/credentials.
class CredentialsScreen extends ConsumerStatefulWidget {
  const CredentialsScreen({super.key});

  @override
  ConsumerState<CredentialsScreen> createState() => _CredentialsScreenState();
}

class _CredentialsScreenState extends ConsumerState<CredentialsScreen> {
  List<Credential> _credentials = [];
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadCredentials();
  }

  Future<void> _loadCredentials() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final apiClient = ref.read(apiClientProvider);
      final credentials = await apiClient.getCredentials();
      setState(() {
        _credentials = credentials;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Credentials')),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text('Error: $_error'))
              : _credentials.isEmpty
                  ? _buildEmptyState(theme)
                  : RefreshIndicator(
                      onRefresh: _loadCredentials,
                      child: ListView.builder(
                        padding: const EdgeInsets.all(16.0),
                        itemCount: _credentials.length,
                        itemBuilder: (context, index) {
                          return _CredentialCard(
                              credential: _credentials[index]);
                        },
                      ),
                    ),
    );
  }

  Widget _buildEmptyState(ThemeData theme) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.verified_outlined,
              size: 64, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(height: 16),
          Text(
            'No credentials yet',
            style: theme.textTheme.titleMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Verify your email or phone to get started',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

/// Card displaying a single Verifiable Credential.
class _CredentialCard extends StatelessWidget {
  final Credential credential;

  const _CredentialCard({required this.credential});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Card(
      margin: const EdgeInsets.only(bottom: 8.0),
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Row(
          children: [
            Icon(
              _typeIcon,
              size: 32,
              color: credential.isRevoked
                  ? theme.colorScheme.error
                  : theme.colorScheme.primary,
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    _displayType,
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    'Issued ${_formatDate(credential.issuedAt)}',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            if (credential.isRevoked)
              Chip(
                label: const Text('Revoked'),
                backgroundColor: theme.colorScheme.errorContainer,
                labelStyle: TextStyle(
                  color: theme.colorScheme.onErrorContainer,
                  fontSize: 12,
                ),
              ),
          ],
        ),
      ),
    );
  }

  String get _displayType {
    // Convert type like "ATAPEmailVerification" to "Email Verified"
    final type = credential.type;
    if (type.contains('Email')) return 'Email Verified';
    if (type.contains('Phone')) return 'Phone Verified';
    if (type.contains('Personhood')) return 'Personhood Verified';
    return type;
  }

  IconData get _typeIcon {
    final type = credential.type;
    if (type.contains('Email')) return Icons.email;
    if (type.contains('Phone')) return Icons.phone;
    if (type.contains('Personhood')) return Icons.person;
    return Icons.verified;
  }

  String _formatDate(DateTime dt) {
    return '${dt.year}-${dt.month.toString().padLeft(2, '0')}-${dt.day.toString().padLeft(2, '0')}';
  }
}
