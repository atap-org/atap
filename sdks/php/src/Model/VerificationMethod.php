<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A verification method in a DID Document.
 */
final class VerificationMethod
{
    public function __construct(
        public readonly string $id,
        public readonly string $type = '',
        public readonly string $controller = '',
        public readonly string $publicKeyMultibase = '',
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
            controller: (string) ($data['controller'] ?? ''),
            publicKeyMultibase: (string) ($data['publicKeyMultibase'] ?? ''),
        );
    }
}
