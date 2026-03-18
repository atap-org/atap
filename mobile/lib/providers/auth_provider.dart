import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import '../core/crypto/secure_storage.dart';
import '../core/models/entity.dart';

/// Authentication state for the app.
class AuthState {
  final Entity? currentEntity;
  final String? keyId;
  final bool isLoading;
  final String? error;

  const AuthState({
    this.currentEntity,
    this.keyId,
    this.isLoading = false,
    this.error,
  });

  bool get isAuthenticated => keyId != null;

  AuthState copyWith({
    Entity? currentEntity,
    String? keyId,
    bool? isLoading,
    String? error,
    bool clearEntity = false,
    bool clearError = false,
  }) {
    return AuthState(
      currentEntity: clearEntity ? null : (currentEntity ?? this.currentEntity),
      keyId: clearEntity ? null : (keyId ?? this.keyId),
      isLoading: isLoading ?? this.isLoading,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

/// Provider for secure storage instance.
final secureStorageProvider = Provider<SecureStorage>((ref) {
  return SecureStorage();
});

/// Provider for API client instance.
final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(
    baseUrl: const String.fromEnvironment(
      'PLATFORM_URL',
      defaultValue: 'http://localhost:8080',
    ),
  );
});

/// Authentication notifier managing login, registration, and session.
class AuthNotifier extends Notifier<AuthState> {
  @override
  AuthState build() => const AuthState();

  SecureStorage get _storage => ref.read(secureStorageProvider);
  ApiClient get _apiClient => ref.read(apiClientProvider);

  /// Sets auth state after registration without fetching entity from server.
  void setAuthenticated({required String keyId, required String entityId}) {
    state = AuthState(keyId: keyId);
  }

  /// Loads saved authentication state from secure storage on app start.
  Future<void> loadSavedAuth() async {
    state = state.copyWith(isLoading: true);

    try {
      final keyId = await _storage.getKeyId();
      final entityId = await _storage.getEntityId();

      if (keyId == null || entityId == null) {
        state = const AuthState();
        return;
      }

      final privateKey = await _storage.getPrivateKey(keyId);
      final publicKey = await _storage.getPublicKey(keyId);
      final entityDID = await _storage.getEntityDID();
      if (privateKey == null || publicKey == null) {
        state = const AuthState();
        return;
      }

      // Set API client credentials with public key for DPoP
      _apiClient.setCredentials(
        privateKey: privateKey,
        keyId: keyId,
        publicKey: publicKey,
      );
      if (entityDID != null) {
        _apiClient.setEntityDID(entityDID);
      }

      // Obtain fresh OAuth token
      try {
        await _apiClient.authenticate();
        state = AuthState(keyId: keyId);
      } catch (_) {
        // Could not reach platform, but we have local credentials
        state = AuthState(keyId: keyId);
      }
    } catch (e) {
      state = AuthState(error: e.toString());
    }
  }

  /// Logs out by clearing secure storage and API credentials.
  Future<void> logout() async {
    await _storage.deleteAll();
    _apiClient.clearCredentials();
    state = const AuthState();
  }
}

/// Provider for auth state.
final authProvider = NotifierProvider<AuthNotifier, AuthState>(
  AuthNotifier.new,
);
