<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A versioned public key for an entity.
 */
final class KeyVersion
{
    public function __construct(
        public readonly string $id,
        public readonly string $entityId = '',
        public readonly int $keyIndex = 0,
        public readonly string $validFrom = '',
        public readonly ?string $validUntil = null,
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
            entityId: (string) ($data['entity_id'] ?? ''),
            keyIndex: (int) ($data['key_index'] ?? 0),
            validFrom: (string) ($data['valid_from'] ?? ''),
            validUntil: $data['valid_until'] ?? null,
            createdAt: (string) ($data['created_at'] ?? ''),
        );
    }
}
