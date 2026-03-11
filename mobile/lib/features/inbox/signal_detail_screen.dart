import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/models/signal.dart';
import '../../providers/inbox_provider.dart';

/// Full signal detail view with structured fields and formatted JSON data.
///
/// Displays:
/// - Route information (origin, target, reply_to, channel, thread, ref)
/// - Trust metadata (signature, key_id, algorithm)
/// - Context metadata (source, idempotency_key, tags, ttl, priority)
/// - Signal data as formatted JSON with copy-to-clipboard
class SignalDetailScreen extends ConsumerWidget {
  final String signalId;

  const SignalDetailScreen({super.key, required this.signalId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final inboxState = ref.watch(inboxProvider);
    final signal = _findSignal(inboxState.signals);
    final theme = Theme.of(context);

    if (signal == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Signal')),
        body: Center(
          child: Text(
            'Signal not found',
            style: theme.textTheme.bodyLarge?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      );
    }

    return Scaffold(
      appBar: AppBar(
        title: Text(signal.signal.type),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _SectionHeader(title: 'Route'),
            _buildRouteSection(signal.route, theme),
            const SizedBox(height: 16),
            if (signal.trust != null) ...[
              _SectionHeader(title: 'Trust'),
              _buildTrustSection(signal.trust!, theme),
              const SizedBox(height: 16),
            ],
            if (signal.context != null) ...[
              _SectionHeader(title: 'Context'),
              _buildContextSection(signal.context!, theme),
              const SizedBox(height: 16),
            ],
            _SectionHeader(
              title: 'Signal Data',
              trailing: signal.signal.data != null
                  ? IconButton(
                      icon: const Icon(Icons.copy, size: 18),
                      onPressed: () => _copyData(context, signal.signal.data),
                      tooltip: 'Copy to clipboard',
                    )
                  : null,
            ),
            _buildDataSection(signal.signal, theme),
            const SizedBox(height: 16),
            // Signal metadata
            _SectionHeader(title: 'Metadata'),
            _DetailCard(children: [
              _DetailRow(label: 'ID', value: signal.id),
              _DetailRow(
                label: 'Created',
                value: signal.createdAt.toIso8601String(),
              ),
            ]),
          ],
        ),
      ),
    );
  }

  Signal? _findSignal(List<Signal> signals) {
    try {
      return signals.firstWhere((s) => s.id == signalId);
    } catch (_) {
      return null;
    }
  }

  Widget _buildRouteSection(SignalRoute route, ThemeData theme) {
    return _DetailCard(children: [
      _DetailRow(label: 'Origin', value: route.origin),
      _DetailRow(label: 'Target', value: route.target),
      if (route.replyTo != null)
        _DetailRow(label: 'Reply To', value: route.replyTo!),
      if (route.channel != null)
        _DetailRow(label: 'Channel', value: route.channel!),
      if (route.thread != null)
        _DetailRow(label: 'Thread', value: route.thread!),
      if (route.ref != null)
        _DetailRow(label: 'Ref', value: route.ref!),
    ]);
  }

  Widget _buildTrustSection(SignalTrust trust, ThemeData theme) {
    return _DetailCard(children: [
      if (trust.keyId != null)
        _DetailRow(label: 'Key ID', value: trust.keyId!),
      if (trust.algorithm != null)
        _DetailRow(label: 'Algorithm', value: trust.algorithm!),
      if (trust.signature != null)
        _DetailRow(
          label: 'Signature',
          value: '${trust.signature!.substring(0, 20)}...',
        ),
    ]);
  }

  Widget _buildContextSection(SignalContext ctx, ThemeData theme) {
    return _DetailCard(children: [
      if (ctx.source != null)
        _DetailRow(label: 'Source', value: ctx.source!),
      if (ctx.idempotencyKey != null)
        _DetailRow(label: 'Idempotency Key', value: ctx.idempotencyKey!),
      if (ctx.priority != null)
        _DetailRow(label: 'Priority', value: ctx.priority!),
      if (ctx.ttl != null)
        _DetailRow(label: 'TTL', value: '${ctx.ttl}s'),
      if (ctx.tags != null && ctx.tags!.isNotEmpty)
        _DetailRow(label: 'Tags', value: ctx.tags!.join(', ')),
    ]);
  }

  Widget _buildDataSection(SignalBody body, ThemeData theme) {
    if (body.data == null) {
      return _DetailCard(children: [
        Padding(
          padding: const EdgeInsets.all(12.0),
          child: Text(
            'No data',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontStyle: FontStyle.italic,
            ),
          ),
        ),
      ]);
    }

    final formattedJson = const JsonEncoder.withIndent('  ').convert(body.data);

    return Card(
      child: SizedBox(
        width: double.infinity,
        child: Padding(
          padding: const EdgeInsets.all(12.0),
          child: SelectableText(
            formattedJson,
            style: theme.textTheme.bodySmall?.copyWith(
              fontFamily: 'monospace',
              fontSize: 12,
            ),
          ),
        ),
      ),
    );
  }

  void _copyData(BuildContext context, dynamic data) {
    final text = const JsonEncoder.withIndent('  ').convert(data);
    Clipboard.setData(ClipboardData(text: text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Signal data copied to clipboard'),
        duration: Duration(seconds: 2),
      ),
    );
  }
}

/// Section header with optional trailing widget.
class _SectionHeader extends StatelessWidget {
  final String title;
  final Widget? trailing;

  const _SectionHeader({required this.title, this.trailing});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 8.0),
      child: Row(
        children: [
          Text(
            title,
            style: theme.textTheme.titleSmall?.copyWith(
              color: theme.colorScheme.primary,
              fontWeight: FontWeight.bold,
            ),
          ),
          const Spacer(),
          if (trailing != null) trailing!,
        ],
      ),
    );
  }
}

/// Card containing detail rows.
class _DetailCard extends StatelessWidget {
  final List<Widget> children;

  const _DetailCard({required this.children});

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: children,
        ),
      ),
    );
  }
}

/// Label-value pair row.
class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4.0),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: theme.textTheme.bodySmall,
            ),
          ),
        ],
      ),
    );
  }
}
