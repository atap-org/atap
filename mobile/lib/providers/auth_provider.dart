import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/api_client.dart';
import '../core/crypto/ed25519_service.dart';
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

  bool get isAuthenticated => currentEntity != null && keyId != null;

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
      defaultValue: 'http://localhost:3000',
    ),
  );
});

/// Authentication notifier managing login, registration, and session.
class AuthNotifier extends Notifier<AuthState> {
  @override
  AuthState build() => const AuthState();

  SecureStorage get _storage => ref.read(secureStorageProvider);
  ApiClient get _apiClient => ref.read(apiClientProvider);

  /// Registers a new human entity via claim code.
  ///
  /// Generates an Ed25519 keypair on device, sends the public key
  /// to the platform, and stores the private key securely.
  Future<void> register({
    required String email,
    required String claimCode,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      // Generate keypair on device
      final keyPair = Ed25519Service.generateKeyPair();

      // Register with platform
      final response = await _apiClient.registerHuman(
        publicKey: base64.encode(keyPair.publicKey),
        email: email,
        claimCode: claimCode,
      );

      final entity =
          Entity.fromJson(response['entity'] as Map<String, dynamic>);
      final keyId = response['key_id'] as String;

      // Store private key securely
      await _storage.savePrivateKey(keyId, keyPair.privateKey);
      await _storage.saveKeyId(keyId);
      await _storage.saveEntityId(entity.id);

      // Set API client credentials
      _apiClient.setCredentials(
        privateKey: keyPair.privateKey,
        keyId: keyId,
      );

      state = AuthState(
        currentEntity: entity,
        keyId: keyId,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
    }
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
      if (privateKey == null) {
        state = const AuthState();
        return;
      }

      // Set API client credentials
      _apiClient.setCredentials(
        privateKey: privateKey,
        keyId: keyId,
      );

      // Fetch current entity from platform
      try {
        final response = await _apiClient.get('/v1/me');
        final entity = Entity.fromJson(response);
        state = AuthState(
          currentEntity: entity,
          keyId: keyId,
        );
      } catch (_) {
        // Could not reach platform, but we have local credentials
        // Set a minimal state so the app can still navigate
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
