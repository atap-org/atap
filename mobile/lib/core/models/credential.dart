/// Credential model for W3C Verifiable Credentials.
///
/// Maps to the credential JSON returned by GET /v1/credentials.
class Credential {
  final String id;
  final String type;
  final String jwt;
  final DateTime issuedAt;
  final DateTime? revokedAt;

  const Credential({
    required this.id,
    required this.type,
    required this.jwt,
    required this.issuedAt,
    this.revokedAt,
  });

  bool get isRevoked => revokedAt != null;

  factory Credential.fromJson(Map<String, dynamic> json) {
    return Credential(
      id: json['id'] as String,
      type: json['type'] as String,
      jwt: json['jwt'] as String,
      issuedAt: DateTime.parse(json['issued_at'] as String),
      revokedAt: json['revoked_at'] != null
          ? DateTime.parse(json['revoked_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': type,
        'jwt': jwt,
        'issued_at': issuedAt.toIso8601String(),
        if (revokedAt != null) 'revoked_at': revokedAt!.toIso8601String(),
      };
}
