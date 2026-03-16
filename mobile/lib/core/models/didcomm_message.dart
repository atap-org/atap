/// DIDComm message model replacing the old Signal model.
///
/// Maps to DIDComm v2 messages received from GET /v1/didcomm/inbox.
class DIDCommMessage {
  final String id;
  final String messageType;
  final String senderDID;
  final Map<String, dynamic> body;
  final DateTime createdAt;

  const DIDCommMessage({
    required this.id,
    required this.messageType,
    required this.senderDID,
    required this.body,
    required this.createdAt,
  });

  /// Whether this message is an approval request or co-signed approval.
  bool get isApprovalRequest =>
      messageType == 'https://atap.dev/protocols/approval/1.0/request' ||
      messageType == 'https://atap.dev/protocols/approval/1.0/cosigned';

  factory DIDCommMessage.fromJson(Map<String, dynamic> json) {
    return DIDCommMessage(
      id: json['id'] as String,
      messageType: json['type'] as String,
      senderDID: json['from'] as String? ?? '',
      body: (json['body'] as Map<String, dynamic>?) ?? {},
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': messageType,
        'from': senderDID,
        'body': body,
        'created_at': createdAt.toIso8601String(),
      };
}
