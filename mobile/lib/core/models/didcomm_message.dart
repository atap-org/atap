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
    // Backend returns message_type/sender_did/payload;
    // DIDComm plaintext uses type/from/body — support both
    final type = (json['message_type'] ?? json['type']) as String? ?? '';
    final sender = (json['sender_did'] ?? json['from']) as String? ?? '';

    // Backend sends base64-encoded JWE as 'payload'; try to decode body from it
    Map<String, dynamic> body = {};
    if (json['body'] is Map<String, dynamic>) {
      body = json['body'] as Map<String, dynamic>;
    }

    return DIDCommMessage(
      id: json['id'] as String,
      messageType: type,
      senderDID: sender,
      body: body,
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
