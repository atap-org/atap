import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/models/signal.dart';
import '../../providers/inbox_provider.dart';

/// Card-based inbox view showing signals with real-time SSE streaming.
///
/// Features:
/// - Signal cards with sender, type, timestamp, data preview, priority
/// - Pull-to-refresh
/// - Infinite scroll pagination
/// - SSE streaming for real-time updates
class InboxScreen extends ConsumerStatefulWidget {
  const InboxScreen({super.key});

  @override
  ConsumerState<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends ConsumerState<InboxScreen> {
  final _scrollController = ScrollController();
  late final InboxNotifier _inboxNotifier;

  @override
  void initState() {
    super.initState();
    _inboxNotifier = ref.read(inboxProvider.notifier);
    // Load inbox and connect SSE on screen init
    Future.microtask(() {
      _inboxNotifier.loadInbox();
      _inboxNotifier.connectSSE();
    });

    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    // Disconnect SSE on screen dispose (use saved reference)
    _inboxNotifier.disconnectSSE();
    super.dispose();
  }

  /// Infinite scroll: load more when reaching the end of the list.
  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      final inboxState = ref.read(inboxProvider);
      if (inboxState.hasMore && !inboxState.isLoading) {
        ref.read(inboxProvider.notifier).loadMore();
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final inboxState = ref.watch(inboxProvider);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Inbox'),
        actions: [
          // SSE connection indicator
          Padding(
            padding: const EdgeInsets.only(right: 12.0),
            child: Icon(
              Icons.circle,
              size: 10,
              color: inboxState.isStreaming ? Colors.green : Colors.grey,
            ),
          ),
        ],
      ),
      body: inboxState.signals.isEmpty && !inboxState.isLoading
          ? _buildEmptyState(theme)
          : RefreshIndicator(
              onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
              child: ListView.builder(
                controller: _scrollController,
                padding: const EdgeInsets.symmetric(
                  horizontal: 16.0,
                  vertical: 8.0,
                ),
                itemCount: inboxState.signals.length +
                    (inboxState.isLoading ? 1 : 0),
                itemBuilder: (context, index) {
                  if (index == inboxState.signals.length) {
                    return const Center(
                      child: Padding(
                        padding: EdgeInsets.all(16.0),
                        child: CircularProgressIndicator(),
                      ),
                    );
                  }
                  return _SignalCard(signal: inboxState.signals[index]);
                },
              ),
            ),
    );
  }

  Widget _buildEmptyState(ThemeData theme) {
    return RefreshIndicator(
      onRefresh: () => ref.read(inboxProvider.notifier).refresh(),
      child: ListView(
        children: [
          SizedBox(height: MediaQuery.of(context).size.height * 0.3),
          Icon(
            Icons.inbox_outlined,
            size: 64,
            color: theme.colorScheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            'No signals yet',
            style: theme.textTheme.titleMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          Text(
            'Pull down to refresh',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

/// Individual signal card displaying summary information.
class _SignalCard extends StatelessWidget {
  final Signal signal;

  const _SignalCard({required this.signal});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Card(
      margin: const EdgeInsets.only(bottom: 8.0),
      child: InkWell(
        borderRadius: BorderRadius.circular(12.0),
        onTap: () => context.go('/inbox/${signal.id}'),
        child: Padding(
          padding: const EdgeInsets.all(16.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Top row: sender + priority + timestamp
              Row(
                children: [
                  _PriorityDot(
                    priority: signal.context?.priority,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      signal.route.origin,
                      style: theme.textTheme.titleSmall,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  Text(
                    _formatTimestamp(signal.createdAt),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              // Signal type
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 8,
                  vertical: 2,
                ),
                decoration: BoxDecoration(
                  color: theme.colorScheme.secondaryContainer,
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  signal.signal.type,
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSecondaryContainer,
                  ),
                ),
              ),
              const SizedBox(height: 8),
              // Data preview
              if (signal.signal.data != null)
                Text(
                  _formatDataPreview(signal.signal.data),
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
            ],
          ),
        ),
      ),
    );
  }

  /// Formats timestamp as relative time (e.g., "2m ago") or date.
  String _formatTimestamp(DateTime timestamp) {
    final now = DateTime.now();
    final diff = now.difference(timestamp);

    if (diff.inSeconds < 60) return 'now';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';

    return '${timestamp.month}/${timestamp.day}/${timestamp.year}';
  }

  /// Formats signal data as a preview string (first 100 chars of JSON).
  String _formatDataPreview(dynamic data) {
    try {
      final encoded = data is String ? data : jsonEncode(data);
      if (encoded.length <= 100) return encoded;
      return '${encoded.substring(0, 100)}...';
    } catch (_) {
      return data.toString();
    }
  }
}

/// Colored dot indicator for signal priority level.
class _PriorityDot extends StatelessWidget {
  final String? priority;

  const _PriorityDot({this.priority});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 8,
      height: 8,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: _priorityColor,
      ),
    );
  }

  Color get _priorityColor {
    switch (priority) {
      case 'high':
        return Colors.red;
      case 'normal':
        return Colors.blue;
      case 'low':
        return Colors.grey;
      default:
        return Colors.blue;
    }
  }
}
