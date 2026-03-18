<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A revocation entry for a previously-granted approval.
 */
final class Revocation
{
    public function __construct(
        public readonly string $id,
        public readonly string $approvalId,
        public readonly string $approverDid = '',
        public readonly string $revokedAt = '',
        public readonly string $expiresAt = '',
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            id: (string) ($data['id'] ?? ''),
            approvalId: (string) ($data['approval_id'] ?? ''),
            approverDid: (string) ($data['approver_did'] ?? ''),
            revokedAt: (string) ($data['revoked_at'] ?? ''),
            expiresAt: (string) ($data['expires_at'] ?? ''),
        );
    }
}
