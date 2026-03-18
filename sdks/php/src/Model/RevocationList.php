<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A list of active revocations for an entity.
 */
final class RevocationList
{
    /**
     * @param Revocation[] $revocations
     */
    public function __construct(
        public readonly string $entity,
        public readonly array $revocations = [],
        public readonly string $checkedAt = '',
    ) {
    }
}
