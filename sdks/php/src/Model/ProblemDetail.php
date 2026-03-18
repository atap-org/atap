<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * RFC 7807 Problem Details error response.
 */
final class ProblemDetail
{
    public function __construct(
        public readonly string $type,
        public readonly string $title,
        public readonly int $status,
        public readonly ?string $detail = null,
        public readonly ?string $instance = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            type: (string) ($data['type'] ?? ''),
            title: (string) ($data['title'] ?? ''),
            status: (int) ($data['status'] ?? 0),
            detail: isset($data['detail']) ? (string) $data['detail'] : null,
            instance: isset($data['instance']) ? (string) $data['instance'] : null,
        );
    }
}
