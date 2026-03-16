import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/models/didcomm_message.dart';
import 'auth_provider.dart';

/// State for the inbox containing DIDComm messages.
class InboxState {
  final List<DIDCommMessage> messages;
  final bool isLoading;
  final String? error;

  const InboxState({
    this.messages = const [],
    this.isLoading = false,
    this.error,
  });

  InboxState copyWith({
    List<DIDCommMessage>? messages,
    bool? isLoading,
    String? error,
    bool clearError = false,
  }) {
    return InboxState(
      messages: messages ?? this.messages,
      isLoading: isLoading ?? this.isLoading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Inbox notifier managing DIDComm message list with polling.
///
/// Replaces the old SSE-based InboxNotifier. Polls GET /v1/didcomm/inbox
/// every 15 seconds when active.
class InboxNotifier extends Notifier<InboxState> {
  Timer? _pollTimer;

  @override
  InboxState build() => const InboxState();

  /// Fetches inbox messages from the API.
  Future<void> refresh() async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final apiClient = ref.read(apiClientProvider);
      if (!apiClient.isAuthenticated) {
        state = state.copyWith(isLoading: false);
        return;
      }

      final messages = await apiClient.getInbox();
      state = state.copyWith(
        messages: messages,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
    }
  }

  /// Starts auto-polling every 15 seconds.
  void startPolling() {
    stopPolling();
    _pollTimer = Timer.periodic(
      const Duration(seconds: 15),
      (_) => refresh(),
    );
  }

  /// Stops auto-polling.
  void stopPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }
}

/// Provider for inbox state.
final inboxProvider = NotifierProvider<InboxNotifier, InboxState>(
  InboxNotifier.new,
);
