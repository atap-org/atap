<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A DIDComm message from the inbox.
 */
final class DIDCommMessage
{
    public function __construct(
        public readonly string $id,
        public readonly string $senderDid = '',
        public readonly string $messageType = '',
        public readonly string $payload = '',
        public readonly string $createdAt = '',
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            id: (string) ($data['id'] ?? ''),
            senderDid: (string) ($data['sender_did'] ?? ''),
            messageType: (string) ($data['message_type'] ?? ''),
            payload: (string) ($data['payload'] ?? ''),
            createdAt: (string) ($data['created_at'] ?? ''),
        );
    }
}
