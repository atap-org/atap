/// Entity data model matching Go platform models.
class Entity {
  final String id;
  final String type;
  final String uri;
  final String publicKey;
  final String keyId;
  final String? metadata;
  final DateTime createdAt;

  const Entity({
    required this.id,
    required this.type,
    required this.uri,
    required this.publicKey,
    required this.keyId,
    this.metadata,
    required this.createdAt,
  });

  factory Entity.fromJson(Map<String, dynamic> json) {
    return Entity(
      id: json['id'] as String,
      type: json['type'] as String,
      uri: json['uri'] as String,
      publicKey: json['public_key'] as String,
      keyId: json['key_id'] as String,
      metadata: json['metadata'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': type,
        'uri': uri,
        'public_key': publicKey,
        'key_id': keyId,
        if (metadata != null) 'metadata': metadata,
        'created_at': createdAt.toIso8601String(),
      };
}

/// Claim data model for invite flow.
class Claim {
  final String id;
  final String code;
  final String creatorId;
  final String? redeemedBy;
  final String status;
  final DateTime createdAt;
  final DateTime? redeemedAt;
  final DateTime? expiresAt;

  const Claim({
    required this.id,
    required this.code,
    required this.creatorId,
    this.redeemedBy,
    required this.status,
    required this.createdAt,
    this.redeemedAt,
    this.expiresAt,
  });

  factory Claim.fromJson(Map<String, dynamic> json) {
    return Claim(
      id: json['id'] as String,
      code: json['code'] as String,
      creatorId: json['creator_id'] as String,
      redeemedBy: json['redeemed_by'] as String?,
      status: json['status'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
      redeemedAt: json['redeemed_at'] != null
          ? DateTime.parse(json['redeemed_at'] as String)
          : null,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'code': code,
        'creator_id': creatorId,
        if (redeemedBy != null) 'redeemed_by': redeemedBy,
        'status': status,
        'created_at': createdAt.toIso8601String(),
        if (redeemedAt != null) 'redeemed_at': redeemedAt!.toIso8601String(),
        if (expiresAt != null) 'expires_at': expiresAt!.toIso8601String(),
      };
}

/// Delegation data model for trust chain.
class Delegation {
  final String id;
  final String delegatorId;
  final String delegateId;
  final List<dynamic>? scope;
  final DateTime createdAt;
  final DateTime? revokedAt;

  const Delegation({
    required this.id,
    required this.delegatorId,
    required this.delegateId,
    this.scope,
    required this.createdAt,
    this.revokedAt,
  });

  factory Delegation.fromJson(Map<String, dynamic> json) {
    return Delegation(
      id: json['id'] as String,
      delegatorId: json['delegator_id'] as String,
      delegateId: json['delegate_id'] as String,
      scope: json['scope'] as List<dynamic>?,
      createdAt: DateTime.parse(json['created_at'] as String),
      revokedAt: json['revoked_at'] != null
          ? DateTime.parse(json['revoked_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'delegator_id': delegatorId,
        'delegate_id': delegateId,
        if (scope != null) 'scope': scope,
        'created_at': createdAt.toIso8601String(),
        if (revokedAt != null) 'revoked_at': revokedAt!.toIso8601String(),
      };
}
