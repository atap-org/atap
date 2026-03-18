<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * Server discovery document from /.well-known/atap.json.
 */
final class DiscoveryDocument
{
    /**
     * @param string[] $claimTypes
     * @param array<string, mixed>|null $oauth
     */
    public function __construct(
        public readonly string $domain = '',
        public readonly string $apiBase = '',
        public readonly string $didcommEndpoint = '',
        public readonly array $claimTypes = [],
        public readonly string $maxApprovalTtl = '',
        public readonly int $trustLevel = 0,
        public readonly ?array $oauth = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            domain: (string) ($data['domain'] ?? ''),
            apiBase: (string) ($data['api_base'] ?? ''),
            didcommEndpoint: (string) ($data['didcomm_endpoint'] ?? ''),
            claimTypes: $data['claim_types'] ?? [],
            maxApprovalTtl: (string) ($data['max_approval_ttl'] ?? ''),
            trustLevel: (int) ($data['trust_level'] ?? 0),
            oauth: $data['oauth'] ?? null,
        );
    }
}
