<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A multi-signature approval document.
 */
final class Approval
{
    /**
     * @param array<string, string> $signatures
     */
    public function __construct(
        public readonly string $id,
        public readonly string $state = '',
        public readonly string $createdAt = '',
        public readonly ?string $validUntil = null,
        public readonly string $fromDid = '',
        public readonly string $toDid = '',
        public readonly string $via = '',
        public readonly string $parent = '',
        public readonly ?ApprovalSubject $subject = null,
        public readonly string $templateUrl = '',
        public readonly array $signatures = [],
        public readonly ?string $respondedAt = null,
        public readonly ?int $fanOut = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        $subject = null;
        if (isset($data['subject']) && is_array($data['subject'])) {
            $subject = ApprovalSubject::fromArray($data['subject']);
        }

        return new self(
            id: (string) ($data['id'] ?? ''),
            state: (string) ($data['state'] ?? ''),
            createdAt: (string) ($data['created_at'] ?? ''),
            validUntil: $data['valid_until'] ?? null,
            fromDid: (string) ($data['from'] ?? ''),
            toDid: (string) ($data['to'] ?? ''),
            via: (string) ($data['via'] ?? ''),
            parent: (string) ($data['parent'] ?? ''),
            subject: $subject,
            templateUrl: (string) ($data['template_url'] ?? ''),
            signatures: $data['signatures'] ?? [],
            respondedAt: $data['responded_at'] ?? null,
            fanOut: $data['fan_out'] ?? null,
        );
    }
}
