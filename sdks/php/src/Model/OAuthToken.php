<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * An OAuth 2.1 token response.
 */
final class OAuthToken
{
    public function __construct(
        public readonly string $accessToken,
        public readonly string $tokenType = 'DPoP',
        public readonly int $expiresIn = 3600,
        public readonly string $scope = '',
        public readonly ?string $refreshToken = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            accessToken: (string) ($data['access_token'] ?? ''),
            tokenType: (string) ($data['token_type'] ?? 'DPoP'),
            expiresIn: (int) ($data['expires_in'] ?? 3600),
            scope: (string) ($data['scope'] ?? ''),
            refreshToken: $data['refresh_token'] ?? null,
        );
    }
}
