<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * An ATAP entity (agent, machine, human, or org).
 */
final class Entity
{
    public function __construct(
        public readonly string $id,
        public readonly string $type,
        public readonly string $did = '',
        public readonly string $principalDid = '',
        public readonly string $name = '',
        public readonly string $keyId = '',
        public readonly string $publicKey = '',
        public readonly int $trustLevel = 0,
        public readonly string $registry = '',
        public readonly string $createdAt = '',
        public readonly string $updatedAt = '',
        public readonly ?string $clientSecret = null,
        public readonly ?string $privateKey = null,
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
            did: (string) ($data['did'] ?? ''),
            principalDid: (string) ($data['principal_did'] ?? ''),
            name: (string) ($data['name'] ?? ''),
            keyId: (string) ($data['key_id'] ?? ''),
            publicKey: (string) ($data['public_key'] ?? ''),
            trustLevel: (int) ($data['trust_level'] ?? 0),
            registry: (string) ($data['registry'] ?? ''),
            createdAt: (string) ($data['created_at'] ?? ''),
            updatedAt: (string) ($data['updated_at'] ?? ''),
            clientSecret: $data['client_secret'] ?? null,
            privateKey: $data['private_key'] ?? null,
        );
    }
}
