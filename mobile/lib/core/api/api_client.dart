import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;

import '../crypto/ed25519_service.dart';
import '../models/approval.dart';
import '../models/credential.dart';
import '../models/didcomm_message.dart';

/// RFC 7807 Problem Details exception.
class ApiException implements Exception {
  final int statusCode;
  final String type;
  final String title;
  final String? detail;
  final String? instance;

  const ApiException({
    required this.statusCode,
    required this.type,
    required this.title,
    this.detail,
    this.instance,
  });

  factory ApiException.fromResponse(http.Response response) {
    try {
      final body = jsonDecode(response.body) as Map<String, dynamic>;
      return ApiException(
        statusCode: response.statusCode,
        type: body['type'] as String? ?? 'about:blank',
        title: body['title'] as String? ?? 'Unknown error',
        detail: body['detail'] as String?,
        instance: body['instance'] as String?,
      );
    } catch (_) {
      return ApiException(
        statusCode: response.statusCode,
        type: 'about:blank',
        title: 'HTTP ${response.statusCode}',
        detail: response.body,
      );
    }
  }

  @override
  String toString() => 'ApiException($statusCode): $title${detail != null ? ' - $detail' : ''}';
}

/// HTTP client with Ed25519 signed request authentication.
///
/// Signs every authenticated request using the user's private key,
/// matching the Go platform's auth middleware format.
class ApiClient {
  final String baseUrl;
  final http.Client _httpClient;

  Uint8List? _privateKey;
  String? _keyId;

  ApiClient({
    required this.baseUrl,
    http.Client? httpClient,
  }) : _httpClient = httpClient ?? http.Client();

  /// Sets authentication credentials for signed requests.
  void setCredentials({
    required Uint8List privateKey,
    required String keyId,
  }) {
    _privateKey = privateKey;
    _keyId = keyId;
  }

  /// Clears authentication credentials.
  void clearCredentials() {
    _privateKey = null;
    _keyId = null;
  }

  /// Whether the client has authentication credentials set.
  bool get isAuthenticated => _privateKey != null && _keyId != null;

  /// Returns the private key, if set.
  Uint8List? get privateKey => _privateKey;

  /// Returns the key ID, if set.
  String? get keyId => _keyId;

  /// Sends an authenticated GET request.
  Future<Map<String, dynamic>> get(String path) async {
    final response = await _signedRequest('GET', path);
    return _handleResponse(response);
  }

  /// Sends an authenticated POST request.
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body,
  ) async {
    final response = await _signedRequest('POST', path, body: body);
    return _handleResponse(response);
  }

  /// Sends an authenticated DELETE request.
  Future<Map<String, dynamic>> delete(String path) async {
    final response = await _signedRequest('DELETE', path);
    return _handleResponse(response);
  }

  /// Registers a human entity with public key.
  Future<Map<String, dynamic>> registerEntity({
    required String type,
    required String publicKey,
  }) async {
    final uri = Uri.parse('$baseUrl/v1/entities');
    final response = await _httpClient.post(
      uri,
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'type': type,
        'public_key': publicKey,
      }),
    );
    return _handleResponse(response);
  }

  // --- DIDComm ---

  /// Fetches DIDComm inbox messages.
  Future<List<DIDCommMessage>> getInbox() async {
    final response = await get('/v1/didcomm/inbox');
    final messages = (response['messages'] as List<dynamic>?)
            ?.map((m) => DIDCommMessage.fromJson(m as Map<String, dynamic>))
            .toList() ??
        [];
    return messages;
  }

  // --- Approvals ---

  /// Responds to an approval request (approve or decline).
  Future<void> respondApproval(
    String id, {
    required String status,
    required String signature,
  }) async {
    await post('/v1/approvals/$id/respond', {
      'status': status,
      'signature': signature,
    });
  }

  /// Lists persistent approvals for the authenticated entity.
  Future<List<Approval>> getApprovals() async {
    final response = await get('/v1/approvals');
    final approvals = (response['approvals'] as List<dynamic>?)
            ?.map((a) => Approval.fromJson(a as Map<String, dynamic>))
            .toList() ??
        [];
    return approvals;
  }

  /// Revokes a persistent approval.
  Future<void> revokeApproval(String id) async {
    await delete('/v1/approvals/$id');
  }

  // --- Credentials ---

  /// Lists credentials for the authenticated entity.
  Future<List<Credential>> getCredentials() async {
    final response = await get('/v1/credentials');
    final credentials = (response['credentials'] as List<dynamic>?)
            ?.map((c) => Credential.fromJson(c as Map<String, dynamic>))
            .toList() ??
        [];
    return credentials;
  }

  // --- Templates ---

  /// Fetches an approval template by URL, returns parsed JSON.
  Future<Map<String, dynamic>> fetchTemplate(String url) async {
    final uri = Uri.parse(url);
    final response = await _httpClient.get(uri);
    if (response.statusCode >= 200 && response.statusCode < 300) {
      return jsonDecode(response.body) as Map<String, dynamic>;
    }
    throw ApiException.fromResponse(response);
  }

  // --- Internal ---

  /// Sends a signed HTTP request.
  Future<http.Response> _signedRequest(
    String method,
    String path, {
    Map<String, dynamic>? body,
  }) async {
    if (_privateKey == null || _keyId == null) {
      throw StateError('No authentication credentials set. Call setCredentials() first.');
    }

    final uri = Uri.parse('$baseUrl$path');
    final now = DateTime.now().toUtc();

    final authHeaders = Ed25519Service.signRequest(
      _privateKey!,
      _keyId!,
      method,
      path,
      now,
    );

    final headers = {
      ...authHeaders,
      'Content-Type': 'application/json',
    };

    switch (method) {
      case 'GET':
        return _httpClient.get(uri, headers: headers);
      case 'POST':
        return _httpClient.post(
          uri,
          headers: headers,
          body: body != null ? jsonEncode(body) : null,
        );
      case 'DELETE':
        return _httpClient.delete(uri, headers: headers);
      default:
        throw ArgumentError('Unsupported HTTP method: $method');
    }
  }

  /// Handles HTTP response, throwing ApiException on error.
  Map<String, dynamic> _handleResponse(http.Response response) {
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (response.body.isEmpty) return {};
      return jsonDecode(response.body) as Map<String, dynamic>;
    }
    throw ApiException.fromResponse(response);
  }

  /// Closes the underlying HTTP client.
  void close() {
    _httpClient.close();
  }
}
