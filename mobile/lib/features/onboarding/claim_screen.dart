import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;

import '../../core/api/api_client.dart';
import '../../core/models/entity.dart';
import '../../providers/auth_provider.dart';

/// Claim screen that handles deep link claim codes.
///
/// Verifies the claim code with the platform, shows invite details,
/// and navigates to the registration screen on confirmation.
class ClaimScreen extends ConsumerStatefulWidget {
  final String claimCode;

  const ClaimScreen({super.key, required this.claimCode});

  @override
  ConsumerState<ClaimScreen> createState() => _ClaimScreenState();
}

class _ClaimScreenState extends ConsumerState<ClaimScreen> {
  bool _isLoading = true;
  Claim? _claim;
  String? _error;

  @override
  void initState() {
    super.initState();
    _verifyClaim();
  }

  Future<void> _verifyClaim() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final apiClient = ref.read(apiClientProvider);
      final uri = Uri.parse('${apiClient.baseUrl}/v1/claims/${widget.claimCode}');
      final response = await http.get(uri);

      if (response.statusCode >= 200 && response.statusCode < 300) {
        final json = jsonDecode(response.body) as Map<String, dynamic>;
        setState(() {
          _claim = Claim.fromJson(json);
          _isLoading = false;
        });
      } else {
        final error = ApiException.fromResponse(response);
        setState(() {
          _error = error.detail ?? error.title;
          _isLoading = false;
        });
      }
    } catch (e) {
      setState(() {
        _error = 'Could not verify claim. Check your connection.';
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Claim Invite'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: _isLoading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? _buildErrorState(theme)
                : _buildClaimDetails(theme),
      ),
    );
  }

  Widget _buildErrorState(ThemeData theme) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.error_outline,
            size: 64,
            color: theme.colorScheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            'Invalid Claim',
            style: theme.textTheme.headlineSmall,
          ),
          const SizedBox(height: 8),
          Text(
            _error!,
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),
          OutlinedButton(
            onPressed: _verifyClaim,
            child: const Text('Try Again'),
          ),
        ],
      ),
    );
  }

  Widget _buildClaimDetails(ThemeData theme) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.verified_user,
            size: 64,
            color: theme.colorScheme.primary,
          ),
          const SizedBox(height: 16),
          Text(
            'You\'ve been invited!',
            style: theme.textTheme.headlineSmall,
          ),
          const SizedBox(height: 8),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16.0),
              child: Column(
                children: [
                  _detailRow('Claim Code', widget.claimCode),
                  if (_claim != null) ...[
                    const Divider(),
                    _detailRow('Created by', _claim!.creatorId),
                    const Divider(),
                    _detailRow('Status', _claim!.status),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 32),
          FilledButton.icon(
            onPressed: () {
              context.go('/register', extra: widget.claimCode);
            },
            icon: const Icon(Icons.arrow_forward),
            label: const Text('Get Started'),
          ),
        ],
      ),
    );
  }

  Widget _detailRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4.0),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontWeight: FontWeight.w500)),
          Text(value),
        ],
      ),
    );
  }
}
