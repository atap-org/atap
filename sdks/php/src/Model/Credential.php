<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A W3C Verifiable Credential.
 */
final class Credential
{
    public function __construct(
        public readonly string $id = '',
        public readonly string $type = '',
        public readonly string $credential = '',
        public readonly string $issuedAt = '',
        public readonly ?string $revokedAt = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            id: (string) ($data['id'] ?? ''),
            type: (string) ($data['type'] ?? ''),
            credential: (string) ($data['credential'] ?? ''),
            issuedAt: (string) ($data['issued_at'] ?? ''),
            revokedAt: $data['revoked_at'] ?? null,
        );
    }
}
