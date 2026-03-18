<?php

declare(strict_types=1);

namespace Atap\Sdk\Crypto;

/**
 * Base64url encoding/decoding (RFC 4648 Section 5) without padding.
 */
final class Base64Url
{
    /**
     * Encode data using base64url without padding.
     */
    public static function encode(string $data): string
    {
        return rtrim(strtr(base64_encode($data), '+/', '-_'), '=');
    }

    /**
     * Decode base64url-encoded data, restoring padding.
     */
    public static function decode(string $data): string
    {
        $remainder = strlen($data) % 4;
        if ($remainder !== 0) {
            $data .= str_repeat('=', 4 - $remainder);
        }

        $decoded = base64_decode(strtr($data, '-_', '+/'), true);
        if ($decoded === false) {
            throw new \InvalidArgumentException('Invalid base64url data');
        }

        return $decoded;
    }
}
