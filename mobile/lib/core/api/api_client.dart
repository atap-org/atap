import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:crypto/crypto.dart' as crypto_pkg;
import 'package:http/http.dart' as http;

import '../crypto/dpop_service.dart';
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
  String toString() =>
      'ApiException($statusCode): $title${detail != null ? ' - $detail' : ''}';
}

/// HTTP client with OAuth 2.1 + DPoP authentication.
///
/// After registration, the client obtains an OAuth access token via
/// Authorization Code + PKCE + DPoP, then includes `Authorization: DPoP <token>`
/// and a fresh `DPoP` proof JWT on every authenticated request.
class ApiClient {
  final String baseUrl;

  /// The platform domain used for DPoP htu URLs.
  /// The server always constructs htu as https://{domain}{path},
  /// so we must match that even when connecting over http locally.
  final String platformDomain;

  final http.Client _httpClient;

  Uint8List? _privateKey;
  Uint8List? _publicKey;
  String? _keyId;
  String? _accessToken;
  String? _refreshToken;
  DPoPService? _dpop;
  String? _entityDID;

  ApiClient({
    required this.baseUrl,
    String? platformDomain,
    http.Client? httpClient,
  })  : platformDomain = platformDomain ?? _extractDomain(baseUrl),
        _httpClient = httpClient ?? http.Client();

  /// Extracts domain:port from a base URL for DPoP htu construction.
  static String _extractDomain(String baseUrl) {
    final uri = Uri.parse(baseUrl);
    if (uri.hasPort && uri.port != 443 && uri.port != 80) {
      return '${uri.host}:${uri.port}';
    }
    return uri.host;
  }

  /// Sets authentication credentials for signed requests.
  void setCredentials({
    required Uint8List privateKey,
    required String keyId,
    Uint8List? publicKey,
  }) {
    _privateKey = privateKey;
    _keyId = keyId;
    if (publicKey != null) {
      _publicKey = publicKey;
    }
  }

  /// Sets the public key (needed for DPoP JWK).
  void setPublicKey(Uint8List publicKey) {
    _publicKey = publicKey;
  }

  /// Sets the entity DID (needed for OAuth authorize).
  void setEntityDID(String did) {
    _entityDID = did;
  }

  /// Clears authentication credentials.
  void clearCredentials() {
    _privateKey = null;
    _publicKey = null;
    _keyId = null;
    _accessToken = null;
    _refreshToken = null;
    _dpop = null;
    _entityDID = null;
  }

  /// Whether the client has authentication credentials set.
  bool get isAuthenticated =>
      _privateKey != null && _keyId != null && _publicKey != null;

  /// Returns the private key, if set.
  Uint8List? get privateKey => _privateKey;

  /// Returns the key ID, if set.
  String? get keyId => _keyId;

  /// Performs the OAuth Authorization Code + PKCE + DPoP flow to obtain tokens.
  ///
  /// For human entities, this is required before making authenticated API calls.
  /// The flow:
  /// 1. GET /v1/oauth/authorize with DPoP proof → redirect with auth code
  /// 2. POST /v1/oauth/token with auth code + code_verifier + DPoP → tokens
  Future<void> authenticate({String scope = 'atap:inbox atap:send atap:revoke atap:manage'}) async {
    if (_privateKey == null || _publicKey == null) {
      throw StateError('Credentials not set. Call setCredentials() first.');
    }
    if (_entityDID == null) {
      throw StateError('Entity DID not set. Call setEntityDID() first.');
    }

    _dpop = DPoPService(privateKey: _privateKey!, publicKey: _publicKey!);

    // Generate PKCE code verifier and challenge
    final codeVerifier = _generateCodeVerifier();
    final codeChallenge = _pkceS256Challenge(codeVerifier);

    // Step 1: Authorization request
    final authorizeUrl =
        'https://$platformDomain/v1/oauth/authorize';
    final dpopProof = _dpop!.createProof('GET', authorizeUrl);

    final authorizeUri = Uri.parse('$baseUrl/v1/oauth/authorize').replace(
      queryParameters: {
        'response_type': 'code',
        'client_id': _entityDID!,
        'redirect_uri': 'atap://callback',
        'scope': scope,
        'code_challenge': codeChallenge,
        'code_challenge_method': 'S256',
      },
    );

    // Use a non-redirecting request to capture the 302 Location header
    final authRequest = http.Request('GET', authorizeUri);
    authRequest.headers['DPoP'] = dpopProof;
    authRequest.followRedirects = false;

    final authStreamedResponse = await _httpClient.send(authRequest);
    final authResponse =
        await http.Response.fromStream(authStreamedResponse);

    // Extract auth code from 302 Location header
    String authCode;
    if (authResponse.statusCode == 302) {
      final location = authResponse.headers['location'] ?? '';
      final locationUri = Uri.parse(location);
      authCode = locationUri.queryParameters['code'] ?? '';
    } else {
      throw ApiException.fromResponse(authResponse);
    }

    if (authCode.isEmpty) {
      throw const ApiException(
        statusCode: 500,
        type: 'auth_failed',
        title: 'Authentication Failed',
        detail: 'Failed to obtain authorization code from redirect',
      );
    }

    // Step 2: Token exchange
    final tokenUrl = 'https://$platformDomain/v1/oauth/token';
    final tokenDpopProof = _dpop!.createProof('POST', tokenUrl);

    final tokenResponse = await _httpClient.post(
      Uri.parse('$baseUrl/v1/oauth/token'),
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        'DPoP': tokenDpopProof,
      },
      body: Uri(queryParameters: {
        'grant_type': 'authorization_code',
        'code': authCode,
        'redirect_uri': 'atap://callback',
        'code_verifier': codeVerifier,
      }).query,
    );

    if (tokenResponse.statusCode != 200) {
      throw ApiException.fromResponse(tokenResponse);
    }

    final tokenBody =
        jsonDecode(tokenResponse.body) as Map<String, dynamic>;
    _accessToken = tokenBody['access_token'] as String;
    _refreshToken = tokenBody['refresh_token'] as String?;
  }

  /// Sends an authenticated GET request.
  Future<Map<String, dynamic>> get(String path) async {
    final response = await _authenticatedRequest('GET', path);
    return _handleResponse(response);
  }

  /// Sends an authenticated POST request.
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> body,
  ) async {
    final response = await _authenticatedRequest('POST', path, body: body);
    return _handleResponse(response);
  }

  /// Sends an authenticated DELETE request.
  Future<Map<String, dynamic>> delete(String path) async {
    final response = await _authenticatedRequest('DELETE', path);
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
    final response = await _authenticatedRequest('GET', '/v1/approvals');
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException.fromResponse(response);
    }
    if (response.body.isEmpty) return [];
    final decoded = jsonDecode(response.body);
    final list = (decoded is List) ? decoded : (decoded['approvals'] as List<dynamic>?) ?? [];
    return list
        .map((a) => Approval.fromJson(a as Map<String, dynamic>))
        .toList();
  }

  /// Revokes a persistent approval.
  Future<void> revokeApproval(String id) async {
    await delete('/v1/approvals/$id');
  }

  // --- Credentials ---

  /// Lists credentials for the authenticated entity.
  Future<List<Credential>> getCredentials() async {
    final response = await _authenticatedRequest('GET', '/v1/credentials');
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw ApiException.fromResponse(response);
    }
    if (response.body.isEmpty) return [];
    final decoded = jsonDecode(response.body);
    final list = (decoded is List) ? decoded : (decoded['credentials'] as List<dynamic>?) ?? [];
    return list
        .map((c) => Credential.fromJson(c as Map<String, dynamic>))
        .toList();
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

  /// Sends an authenticated HTTP request with DPoP proof.
  Future<http.Response> _authenticatedRequest(
    String method,
    String path, {
    Map<String, dynamic>? body,
  }) async {
    if (_accessToken == null) {
      // Auto-authenticate if we have credentials but no token
      if (isAuthenticated && _entityDID != null) {
        await authenticate();
      } else {
        throw StateError(
            'Not authenticated. Call authenticate() after setCredentials().');
      }
    }

    _dpop ??= DPoPService(privateKey: _privateKey!, publicKey: _publicKey!);

    final uri = Uri.parse('$baseUrl$path');
    final htu = 'https://$platformDomain$path';
    final dpopProof = _dpop!.createProof(method, htu);

    final headers = {
      'Authorization': 'DPoP $_accessToken',
      'DPoP': dpopProof,
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

  /// Generates a PKCE code verifier (43-128 chars, URL-safe).
  static String _generateCodeVerifier() {
    final random = Random.secure();
    final bytes = List<int>.generate(32, (_) => random.nextInt(256));
    return base64Url.encode(bytes).replaceAll('=', '');
  }

  /// Computes PKCE S256 challenge from verifier.
  static String _pkceS256Challenge(String verifier) {
    final hash = crypto_pkg.sha256.convert(utf8.encode(verifier));
    return base64Url.encode(hash.bytes).replaceAll('=', '');
  }

  /// Closes the underlying HTTP client.
  void close() {
    _httpClient.close();
  }
}
