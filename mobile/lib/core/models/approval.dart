/// Approval model for approval cards and management.
///
/// Maps to the approval JSON returned by GET /v1/approvals
/// and extracted from DIDComm approval request message bodies.
class Approval {
  final String id;
  final String from;
  final String to;
  final String? via;
  final String state;
  final ApprovalSubject subject;
  final String? templateUrl;
  final Map<String, String> signatures;
  final DateTime createdAt;
  final DateTime? validUntil;

  const Approval({
    required this.id,
    required this.from,
    required this.to,
    this.via,
    required this.state,
    required this.subject,
    this.templateUrl,
    this.signatures = const {},
    required this.createdAt,
    this.validUntil,
  });

  /// Whether this is a persistent (standing) approval with an expiry.
  bool get isPersistent => validUntil != null;

  factory Approval.fromJson(Map<String, dynamic> json) {
    return Approval(
      id: json['id'] as String,
      from: json['from'] as String,
      to: json['to'] as String,
      via: json['via'] as String?,
      state: json['state'] as String,
      subject: ApprovalSubject.fromJson(
          json['subject'] as Map<String, dynamic>),
      templateUrl: json['template_url'] as String?,
      signatures: (json['signatures'] as Map<String, dynamic>?)
              ?.map((k, v) => MapEntry(k, v as String)) ??
          {},
      createdAt: DateTime.parse(json['created_at'] as String),
      validUntil: json['valid_until'] != null
          ? DateTime.parse(json['valid_until'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'from': from,
        'to': to,
        if (via != null) 'via': via,
        'state': state,
        'subject': subject.toJson(),
        if (templateUrl != null) 'template_url': templateUrl,
        'signatures': signatures,
        'created_at': createdAt.toIso8601String(),
        if (validUntil != null) 'valid_until': validUntil!.toIso8601String(),
      };
}

/// Subject of an approval request.
class ApprovalSubject {
  final String type;
  final String label;
  final bool reversible;
  final Map<String, dynamic> payload;

  const ApprovalSubject({
    required this.type,
    required this.label,
    this.reversible = false,
    this.payload = const {},
  });

  factory ApprovalSubject.fromJson(Map<String, dynamic> json) {
    return ApprovalSubject(
      type: json['type'] as String,
      label: json['label'] as String,
      reversible: json['reversible'] as bool? ?? false,
      payload: (json['payload'] as Map<String, dynamic>?) ?? {},
    );
  }

  Map<String, dynamic> toJson() => {
        'type': type,
        'label': label,
        'reversible': reversible,
        'payload': payload,
      };
}
