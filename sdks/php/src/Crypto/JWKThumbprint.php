<?php

declare(strict_types=1);

namespace Atap\Sdk\Crypto;

/**
 * JWK Thumbprint computation (RFC 7638) for Ed25519 keys.
 */
final class JWKThumbprint
{
    /**
     * Compute the JWK thumbprint for an Ed25519 public key.
     *
     * @param string $publicKey 32-byte raw public key
     * @return string Base64url-encoded SHA-256 thumbprint
     */
    public static function compute(string $publicKey): string
    {
        $x = Base64Url::encode($publicKey);

        // Canonical JSON with sorted keys, no whitespace
        $canonical = json_encode([
            'crv' => 'Ed25519',
            'kty' => 'OKP',
            'x' => $x,
        ], JSON_UNESCAPED_SLASHES);

        $digest = hash('sha256', $canonical, true);

        return Base64Url::encode($digest);
    }
}
