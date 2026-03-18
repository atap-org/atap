import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:local_auth/local_auth.dart';

import '../../core/crypto/jws_service.dart';
import '../../core/models/didcomm_message.dart';
import '../../providers/auth_provider.dart';

/// Renders an approval request from a DIDComm message.
///
/// If the message body has a `template_url`, fetches and renders a branded
/// card. Otherwise renders a fallback card with subject label and payload.
/// Approve/decline triggers biometric confirmation then JWS signing.
class ApprovalCard extends ConsumerStatefulWidget {
  final DIDCommMessage message;

  const ApprovalCard({super.key, required this.message});

  @override
  ConsumerState<ApprovalCard> createState() => _ApprovalCardState();
}

class _ApprovalCardState extends ConsumerState<ApprovalCard> {
  Map<String, dynamic>? _template;
  bool _isLoadingTemplate = false;
  bool _isResponding = false;
  String? _responseStatus;

  @override
  void initState() {
    super.initState();
    final templateUrl = widget.message.body['template_url'] as String?;
    if (templateUrl != null) {
      _loadTemplate(templateUrl);
    }
  }

  Future<void> _loadTemplate(String url) async {
    setState(() => _isLoadingTemplate = true);
    try {
      final apiClient = ref.read(apiClientProvider);
      final template = await apiClient.fetchTemplate(url);
      setState(() {
        _template = template;
        _isLoadingTemplate = false;
      });
    } catch (_) {
      setState(() => _isLoadingTemplate = false);
    }
  }

  Future<void> _respond(String status) async {
    // Biometric gate
    final localAuth = LocalAuthentication();
    final canAuth = await localAuth.canCheckBiometrics ||
        await localAuth.isDeviceSupported();

    if (canAuth) {
      final didAuth = await localAuth.authenticate(
        localizedReason: 'Confirm $status approval',
        options: const AuthenticationOptions(biometricOnly: false),
      );
      if (!didAuth) return;
    }

    setState(() => _isResponding = true);

    try {
      final storage = ref.read(secureStorageProvider);
      final keyId = await storage.getKeyId();
      if (keyId == null) throw StateError('No key ID');

      final privateKey = await storage.getPrivateKey(keyId);
      if (privateKey == null) throw StateError('No private key');

      // Build approval response payload
      final approvalId = widget.message.body['approval_id'] as String? ??
          widget.message.id;
      final responsePayload = jsonEncode({
        'approval_id': approvalId,
        'status': status,
      });

      // Sign with detached JWS
      final signature = JwsService.signDetached(
        privateKey,
        responsePayload,
        kid: keyId,
      );

      // Send response
      final apiClient = ref.read(apiClientProvider);
      await apiClient.respondApproval(
        approvalId,
        status: status,
        signature: signature,
      );

      setState(() {
        _responseStatus = status;
        _isResponding = false;
      });

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Approval ${status}d')),
        );
      }
    } catch (e) {
      setState(() => _isResponding = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed: $e')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final body = widget.message.body;

    // Already responded
    if (_responseStatus != null) {
      return Card(
        margin: const EdgeInsets.only(bottom: 8.0),
        color: _responseStatus == 'approve'
            ? theme.colorScheme.primaryContainer
            : theme.colorScheme.surfaceContainerHighest,
        child: Padding(
          padding: const EdgeInsets.all(16.0),
          child: Row(
            children: [
              Icon(
                _responseStatus == 'approve'
                    ? Icons.check_circle
                    : Icons.cancel,
                color: _responseStatus == 'approve'
                    ? theme.colorScheme.primary
                    : theme.colorScheme.error,
              ),
              const SizedBox(width: 12),
              Text(
                _responseStatus == 'approve' ? 'Approved' : 'Declined',
                style: theme.textTheme.titleMedium,
              ),
            ],
          ),
        ),
      );
    }

    // Template-branded card
    if (_template != null) {
      return _buildBrandedCard(theme, body);
    }

    // Fallback card
    return _buildFallbackCard(theme, body);
  }

  Widget _buildBrandedCard(ThemeData theme, Map<String, dynamic> body) {
    final brand = _template!['brand'] as Map<String, dynamic>? ?? {};
    final display = _template!['display'] as Map<String, dynamic>? ?? {};
    final brandName = brand['name'] as String? ?? 'Unknown';
    final title = display['title'] as String? ?? 'Approval Request';
    final fields = (display['fields'] as List<dynamic>?) ?? [];

    return Card(
      margin: const EdgeInsets.only(bottom: 8.0),
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Brand header
            Row(
              children: [
                Icon(Icons.business, color: theme.colorScheme.primary),
                const SizedBox(width: 8),
                Text(brandName, style: theme.textTheme.labelLarge),
              ],
            ),
            const Divider(),
            Text(title, style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            // Display fields
            for (final field in fields) ...[
              _buildField(theme, field as Map<String, dynamic>),
              const SizedBox(height: 4),
            ],
            const SizedBox(height: 12),
            _buildActions(theme),
          ],
        ),
      ),
    );
  }

  Widget _buildField(ThemeData theme, Map<String, dynamic> field) {
    final label = field['label'] as String? ?? '';
    final value = field['value']?.toString() ?? '';
    final type = field['type'] as String? ?? 'text';

    String displayValue = value;
    if (type == 'currency') {
      final currency = field['currency'] as String? ?? '';
      displayValue = '$currency $value';
    }

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label,
            style: theme.textTheme.bodySmall
                ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
        Text(displayValue, style: theme.textTheme.bodyMedium),
      ],
    );
  }

  Widget _buildFallbackCard(ThemeData theme, Map<String, dynamic> body) {
    final subject = body['subject'] as Map<String, dynamic>?;
    final label = subject?['label'] as String? ?? 'Approval Request';
    final payload = subject?['payload'] as Map<String, dynamic>?;

    return Card(
      margin: const EdgeInsets.only(bottom: 8.0),
      child: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.approval, color: theme.colorScheme.primary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(label, style: theme.textTheme.titleMedium),
                ),
              ],
            ),
            if (body['from'] != null) ...[
              const SizedBox(height: 4),
              Text(
                'From: ${body['from']}',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
            if (payload != null) ...[
              const SizedBox(height: 8),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: theme.colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  const JsonEncoder.withIndent('  ').convert(payload),
                  style: theme.textTheme.bodySmall?.copyWith(
                    fontFamily: 'monospace',
                    fontSize: 11,
                  ),
                  maxLines: 6,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
            const SizedBox(height: 12),
            _buildActions(theme),
          ],
        ),
      ),
    );
  }

  Widget _buildActions(ThemeData theme) {
    if (_isResponding || _isLoadingTemplate) {
      return const Center(child: CircularProgressIndicator());
    }

    return Row(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        OutlinedButton.icon(
          onPressed: () => _respond('decline'),
          icon: const Icon(Icons.close),
          label: const Text('Decline'),
        ),
        const SizedBox(width: 8),
        FilledButton.icon(
          onPressed: () => _respond('approve'),
          icon: const Icon(Icons.check),
          label: const Text('Approve'),
        ),
      ],
    );
  }
}
