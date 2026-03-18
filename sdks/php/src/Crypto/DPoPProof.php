<?php

declare(strict_types=1);

namespace Atap\Sdk\Crypto;

/**
 * DPoP proof JWT creation (RFC 9449).
 */
final class DPoPProof
{
    /**
     * Create a DPoP proof JWT.
     *
     * @param string $privateKey 64-byte Ed25519 secret key
     * @param string $method HTTP method (GET, POST, etc.)
     * @param string $url Full URL for the htu claim (must use https://{platformDomain}/path)
     * @param string|null $accessToken If provided, includes ath (access token hash) claim
     * @return string Compact JWS (header.payload.signature)
     */
    public static function create(string $privateKey, string $method, string $url, ?string $accessToken = null): string
    {
        $publicKey = Ed25519::getPublicKey($privateKey);
        $x = Base64Url::encode($publicKey);

        $header = [
            'typ' => 'dpop+jwt',
            'alg' => 'EdDSA',
            'jwk' => [
                'kty' => 'OKP',
                'crv' => 'Ed25519',
                'x' => $x,
            ],
        ];

        $payload = [
            'jti' => self::generateUuid(),
            'htm' => $method,
            'htu' => $url,
            'iat' => time(),
        ];

        if ($accessToken !== null) {
            $ath = hash('sha256', $accessToken, true);
            $payload['ath'] = Base64Url::encode($ath);
        }

        $headerB64 = Base64Url::encode(json_encode($header, JSON_UNESCAPED_SLASHES));
        $payloadB64 = Base64Url::encode(json_encode($payload, JSON_UNESCAPED_SLASHES));
        $signingInput = $headerB64 . '.' . $payloadB64;

        $signature = Ed25519::sign($signingInput, $privateKey);
        $sigB64 = Base64Url::encode($signature);

        return $headerB64 . '.' . $payloadB64 . '.' . $sigB64;
    }

    /**
     * Generate a UUID v4 string.
     */
    private static function generateUuid(): string
    {
        $data = random_bytes(16);
        // Set version to 0100 (v4)
        $data[6] = chr(ord($data[6]) & 0x0f | 0x40);
        // Set variant to 10xx
        $data[8] = chr(ord($data[8]) & 0x3f | 0x80);

        return vsprintf('%s%s-%s-%s-%s-%s%s%s', str_split(bin2hex($data), 4));
    }
}
