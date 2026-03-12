/// Signal data model matching Go platform models.
///
/// A signal is the core message unit in ATAP, containing routing,
/// payload, context, and trust metadata.
class Signal {
  final String id;
  final SignalRoute route;
  final SignalBody signal;
  final SignalContext? context;
  final SignalTrust? trust;
  final DateTime createdAt;

  const Signal({
    required this.id,
    required this.route,
    required this.signal,
    this.context,
    this.trust,
    required this.createdAt,
  });

  factory Signal.fromJson(Map<String, dynamic> json) {
    return Signal(
      id: json['id'] as String,
      route: SignalRoute.fromJson(json['route'] as Map<String, dynamic>),
      signal: SignalBody.fromJson(json['signal'] as Map<String, dynamic>),
      context: json['context'] != null
          ? SignalContext.fromJson(json['context'] as Map<String, dynamic>)
          : null,
      trust: json['trust'] != null
          ? SignalTrust.fromJson(json['trust'] as Map<String, dynamic>)
          : null,
      createdAt: DateTime.parse(json['ts'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'route': route.toJson(),
        'signal': signal.toJson(),
        if (context != null) 'context': context!.toJson(),
        if (trust != null) 'trust': trust!.toJson(),
        'ts': createdAt.toIso8601String(),
      };
}

/// Signal routing information.
class SignalRoute {
  final String origin;
  final String target;
  final String? replyTo;
  final String? channel;
  final String? thread;
  final String? ref;

  const SignalRoute({
    required this.origin,
    required this.target,
    this.replyTo,
    this.channel,
    this.thread,
    this.ref,
  });

  factory SignalRoute.fromJson(Map<String, dynamic> json) {
    return SignalRoute(
      origin: json['origin'] as String,
      target: json['target'] as String,
      replyTo: json['reply_to'] as String?,
      channel: json['channel'] as String?,
      thread: json['thread'] as String?,
      ref: json['ref'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'origin': origin,
        'target': target,
        if (replyTo != null) 'reply_to': replyTo,
        if (channel != null) 'channel': channel,
        if (thread != null) 'thread': thread,
        if (ref != null) 'ref': ref,
      };
}

/// Signal payload.
class SignalBody {
  final String type;
  final bool? encrypted;
  final dynamic data;

  const SignalBody({
    required this.type,
    this.encrypted,
    this.data,
  });

  factory SignalBody.fromJson(Map<String, dynamic> json) {
    return SignalBody(
      type: json['type'] as String,
      encrypted: json['encrypted'] as bool?,
      data: json['data'],
    );
  }

  Map<String, dynamic> toJson() => {
        'type': type,
        if (encrypted != null) 'encrypted': encrypted,
        if (data != null) 'data': data,
      };
}

/// Signal context metadata.
class SignalContext {
  final String? source;
  final String? idempotencyKey;
  final List<String>? tags;
  final int? ttl;
  final String? priority;

  const SignalContext({
    this.source,
    this.idempotencyKey,
    this.tags,
    this.ttl,
    this.priority,
  });

  factory SignalContext.fromJson(Map<String, dynamic> json) {
    return SignalContext(
      source: json['source'] as String?,
      idempotencyKey: json['idempotency_key'] as String?,
      tags: (json['tags'] as List<dynamic>?)?.cast<String>(),
      ttl: json['ttl'] as int?,
      priority: json['priority'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        if (source != null) 'source': source,
        if (idempotencyKey != null) 'idempotency_key': idempotencyKey,
        if (tags != null) 'tags': tags,
        if (ttl != null) 'ttl': ttl,
        if (priority != null) 'priority': priority,
      };
}

/// Signal trust metadata.
class SignalTrust {
  final String? signature;
  final String? keyId;
  final String? algorithm;

  const SignalTrust({
    this.signature,
    this.keyId,
    this.algorithm,
  });

  factory SignalTrust.fromJson(Map<String, dynamic> json) {
    return SignalTrust(
      signature: json['signature'] as String?,
      keyId: json['key_id'] as String?,
      algorithm: json['algorithm'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        if (signature != null) 'signature': signature,
        if (keyId != null) 'key_id': keyId,
        if (algorithm != null) 'algorithm': algorithm,
      };
}
