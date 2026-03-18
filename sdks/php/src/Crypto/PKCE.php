<?php

declare(strict_types=1);

namespace Atap\Sdk\Crypto;

/**
 * PKCE (Proof Key for Code Exchange) S256 challenge generation.
 */
final class PKCE
{
    /**
     * Generate PKCE code verifier and S256 challenge.
     *
     * @return array{verifier: string, challenge: string}
     */
    public static function generate(): array
    {
        $verifier = Base64Url::encode(random_bytes(32));
        $challenge = Base64Url::encode(hash('sha256', $verifier, true));

        return [
            'verifier' => $verifier,
            'challenge' => $challenge,
        ];
    }
}
