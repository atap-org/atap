import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:http/http.dart' as http;

import '../crypto/ed25519_service.dart';
import '../models/signal.dart';

/// Server-Sent Events client for real-time inbox streaming.
///
/// Connects to the platform SSE endpoint, parses event streams,
/// and manages lifecycle based on app foreground/background state.
class SseClient with WidgetsBindingObserver {
  final String baseUrl;
  final Uint8List Function() getPrivateKey;
  final String Function() getKeyId;

  http.Client? _httpClient;
  StreamController<Signal>? _controller;
  String? _lastEventId;
  String? _entityId;
  bool _isConnected = false;

  SseClient({
    required this.baseUrl,
    required this.getPrivateKey,
    required this.getKeyId,
  });

  /// Whether the SSE connection is currently active.
  bool get isConnected => _isConnected;

  /// The last event ID received, used for reconnection replay.
  String? get lastEventId => _lastEventId;

  /// Opens an SSE connection for the given entity's inbox.
  ///
  /// Returns a broadcast stream of Signal objects. Handles:
  /// - SSE format parsing (data/id/comment lines)
  /// - Heartbeat filtering
  /// - Last-Event-ID tracking for reconnection replay
  Stream<Signal> connect(String entityId, {String? lastEventId}) {
    _entityId = entityId;
    if (lastEventId != null) {
      _lastEventId = lastEventId;
    }

    _controller = StreamController<Signal>.broadcast(
      onCancel: disconnect,
    );

    _startConnection();

    // Register lifecycle observer for battery-saving disconnect/reconnect
    WidgetsBinding.instance.addObserver(this);

    return _controller!.stream;
  }

  /// Disconnects the SSE stream and cleans up resources.
  void disconnect() {
    _isConnected = false;
    _httpClient?.close();
    _httpClient = null;
    if (_controller != null && !_controller!.isClosed) {
      _controller!.close();
    }
    _controller = null;
    WidgetsBinding.instance.removeObserver(this);
  }

  /// Handles app lifecycle changes for battery-saving SSE management.
  ///
  /// Disconnects when app goes to background (paused/inactive),
  /// reconnects with Last-Event-ID when app resumes.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.inactive:
        _stopConnection();
        break;
      case AppLifecycleState.resumed:
        if (_entityId != null && _controller != null && !_controller!.isClosed) {
          _startConnection();
        }
        break;
      default:
        break;
    }
  }

  /// Starts the HTTP connection and processes the SSE stream.
  void _startConnection() async {
    if (_entityId == null) return;

    _httpClient?.close();
    _httpClient = http.Client();

    final path = '/v1/inbox/$_entityId/stream';
    final uri = Uri.parse('$baseUrl$path');
    final now = DateTime.now().toUtc();

    final authHeaders = Ed25519Service.signRequest(
      getPrivateKey(),
      getKeyId(),
      'GET',
      path,
      now,
    );

    final headers = {
      ...authHeaders,
      'Accept': 'text/event-stream',
      'Cache-Control': 'no-cache',
      if (_lastEventId != null) 'Last-Event-ID': _lastEventId!,
    };

    try {
      final request = http.Request('GET', uri);
      request.headers.addAll(headers);

      final response = await _httpClient!.send(request);

      if (response.statusCode != 200) {
        _controller?.addError(
          Exception('SSE connection failed: ${response.statusCode}'),
        );
        return;
      }

      _isConnected = true;

      // Process the byte stream as SSE events
      String buffer = '';
      String? currentId;
      String? currentData;

      response.stream.transform(utf8.decoder).listen(
        (chunk) {
          buffer += chunk;

          // Process complete lines
          while (buffer.contains('\n')) {
            final newlineIndex = buffer.indexOf('\n');
            final line = buffer.substring(0, newlineIndex).trimRight();
            buffer = buffer.substring(newlineIndex + 1);

            if (line.isEmpty) {
              // Empty line = end of event
              if (currentData != null) {
                _processEvent(currentId, currentData!);
                currentId = null;
                currentData = null;
              }
              continue;
            }

            if (line.startsWith(':')) {
              // Comment line (heartbeat), ignore
              continue;
            }

            if (line.startsWith('id:')) {
              currentId = line.substring(3).trim();
              continue;
            }

            if (line.startsWith('data:')) {
              final data = line.substring(5).trim();
              currentData = currentData != null ? '$currentData\n$data' : data;
              continue;
            }
          }
        },
        onError: (error) {
          _isConnected = false;
          if (_controller != null && !_controller!.isClosed) {
            _controller!.addError(error);
          }
        },
        onDone: () {
          _isConnected = false;
        },
      );
    } catch (e) {
      _isConnected = false;
      if (_controller != null && !_controller!.isClosed) {
        _controller!.addError(e);
      }
    }
  }

  /// Stops the HTTP connection without closing the controller.
  void _stopConnection() {
    _isConnected = false;
    _httpClient?.close();
    _httpClient = null;
  }

  /// Processes a complete SSE event, deserializing the signal JSON.
  void _processEvent(String? id, String data) {
    if (id != null) {
      _lastEventId = id;
    }

    try {
      final json = jsonDecode(data) as Map<String, dynamic>;
      final signal = Signal.fromJson(json);
      if (_controller != null && !_controller!.isClosed) {
        _controller!.add(signal);
      }
    } catch (e) {
      // Malformed event data, skip
    }
  }
}
