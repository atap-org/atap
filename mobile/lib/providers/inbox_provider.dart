import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import '../core/api/sse_client.dart';
import '../core/models/signal.dart';
import 'auth_provider.dart';

/// State for the inbox containing signals and loading metadata.
class InboxState {
  final List<Signal> signals;
  final bool isLoading;
  final bool hasMore;
  final bool isStreaming;
  final String? cursor;
  final String? error;

  const InboxState({
    this.signals = const [],
    this.isLoading = false,
    this.hasMore = false,
    this.isStreaming = false,
    this.cursor,
    this.error,
  });

  InboxState copyWith({
    List<Signal>? signals,
    bool? isLoading,
    bool? hasMore,
    bool? isStreaming,
    String? cursor,
    String? error,
    bool clearError = false,
  }) {
    return InboxState(
      signals: signals ?? this.signals,
      isLoading: isLoading ?? this.isLoading,
      hasMore: hasMore ?? this.hasMore,
      isStreaming: isStreaming ?? this.isStreaming,
      cursor: cursor ?? this.cursor,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Inbox notifier managing signal list with SSE streaming.
///
/// Provides:
/// - loadInbox() for initial/refresh load from REST API
/// - loadMore() for cursor-based pagination
/// - connectSSE() for real-time signal streaming
/// - disconnectSSE() for cleanup
/// - refresh() for pull-to-refresh (reload + reconnect SSE)
class InboxNotifier extends Notifier<InboxState> {
  SseClient? _sseClient;
  StreamSubscription<Signal>? _sseSubscription;

  @override
  InboxState build() => const InboxState();

  ApiClient get _apiClient => ref.read(apiClientProvider);

  /// Loads inbox signals from the REST API.
  Future<void> loadInbox() async {
    final authState = ref.read(authProvider);
    if (authState.currentEntity == null) return;

    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final entityId = authState.currentEntity!.id;
      final response = await _apiClient.get('/v1/inbox/$entityId?limit=50');

      final signalsList = (response['signals'] as List<dynamic>?)
              ?.map((s) => Signal.fromJson(s as Map<String, dynamic>))
              .toList() ??
          [];

      state = state.copyWith(
        signals: signalsList,
        isLoading: false,
        hasMore: response['has_more'] as bool? ?? false,
        cursor: response['cursor'] as String?,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
    }
  }

  /// Loads more signals for infinite scroll pagination.
  Future<void> loadMore() async {
    if (!state.hasMore || state.cursor == null || state.isLoading) return;

    final authState = ref.read(authProvider);
    if (authState.currentEntity == null) return;

    state = state.copyWith(isLoading: true);

    try {
      final entityId = authState.currentEntity!.id;
      final response = await _apiClient.get(
        '/v1/inbox/$entityId?limit=50&cursor=${state.cursor}',
      );

      final newSignals = (response['signals'] as List<dynamic>?)
              ?.map((s) => Signal.fromJson(s as Map<String, dynamic>))
              .toList() ??
          [];

      state = state.copyWith(
        signals: [...state.signals, ...newSignals],
        isLoading: false,
        hasMore: response['has_more'] as bool? ?? false,
        cursor: response['cursor'] as String?,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
    }
  }

  /// Opens an SSE connection for real-time signal streaming.
  void connectSSE() {
    final authState = ref.read(authProvider);
    if (authState.currentEntity == null) return;

    final apiClient = _apiClient;
    if (!apiClient.isAuthenticated) return;

    _sseClient = SseClient(
      baseUrl: apiClient.baseUrl,
      getPrivateKey: () => apiClient.privateKey!,
      getKeyId: () => apiClient.keyId!,
    );

    final stream = _sseClient!.connect(authState.currentEntity!.id);
    state = state.copyWith(isStreaming: true);

    _sseSubscription = stream.listen(
      (signal) {
        // Prepend new signals to the top of the list
        state = state.copyWith(
          signals: [signal, ...state.signals],
        );
      },
      onError: (error) {
        state = state.copyWith(isStreaming: false);
      },
      onDone: () {
        state = state.copyWith(isStreaming: false);
      },
    );
  }

  /// Disconnects the SSE stream.
  void disconnectSSE() {
    _sseSubscription?.cancel();
    _sseSubscription = null;
    _sseClient?.disconnect();
    _sseClient = null;
    state = state.copyWith(isStreaming: false);
  }

  /// Pull-to-refresh: disconnects SSE, reloads from API, reconnects SSE.
  Future<void> refresh() async {
    disconnectSSE();
    await loadInbox();
    connectSSE();
  }

}

/// Provider for inbox state.
final inboxProvider = NotifierProvider<InboxNotifier, InboxState>(
  InboxNotifier.new,
);
