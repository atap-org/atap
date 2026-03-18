<?php

declare(strict_types=1);

namespace Atap\Sdk\Crypto;

/**
 * Ed25519 cryptographic operations using ext-sodium.
 *
 * Note: sodium secret keys are 64 bytes (32-byte seed + 32-byte public key).
 * Seeds are 32 bytes and can be expanded to full secret keys.
 */
final class Ed25519
{
    /**
     * Generate a new Ed25519 keypair.
     *
     * @return array{secretKey: string, publicKey: string} Raw binary keys
     */
    public static function generateKeypair(): array
    {
        $keypair = sodium_crypto_sign_keypair();

        return [
            'secretKey' => sodium_crypto_sign_secretkey($keypair),
            'publicKey' => sodium_crypto_sign_publickey($keypair),
        ];
    }

    /**
     * Load a signing key from base64. Accepts 32-byte seed or 64-byte full key.
     *
     * @param string $base64Key Base64-encoded private key
     * @return string 64-byte secret key
     */
    public static function loadSigningKey(string $base64Key): string
    {
        $raw = base64_decode($base64Key, true);
        if ($raw === false) {
            throw new \InvalidArgumentException('Invalid base64 key data');
        }

        $len = strlen($raw);
        if ($len === SODIUM_CRYPTO_SIGN_SECRETKEYBYTES) {
            // Full 64-byte secret key
            return $raw;
        }

        if ($len === SODIUM_CRYPTO_SIGN_SEEDBYTES) {
            // 32-byte seed — expand to full keypair and extract secret key
            $keypair = sodium_crypto_sign_seed_keypair($raw);
            return sodium_crypto_sign_secretkey($keypair);
        }

        throw new \InvalidArgumentException(
            sprintf('Invalid private key length: %d bytes (expected 32 or 64)', $len)
        );
    }

    /**
     * Sign data with an Ed25519 secret key (detached signature).
     *
     * @param string $data Data to sign
     * @param string $secretKey 64-byte secret key
     * @return string 64-byte signature
     */
    public static function sign(string $data, string $secretKey): string
    {
        return sodium_crypto_sign_detached($data, $secretKey);
    }

    /**
     * Extract the 32-byte public key from a 64-byte secret key.
     */
    public static function getPublicKey(string $secretKey): string
    {
        return sodium_crypto_sign_publickey_from_secretkey($secretKey);
    }

    /**
     * Verify a detached signature.
     *
     * @param string $signature 64-byte signature
     * @param string $data Original data
     * @param string $publicKey 32-byte public key
     */
    public static function verify(string $signature, string $data, string $publicKey): bool
    {
        try {
            return sodium_crypto_sign_verify_detached($signature, $data, $publicKey);
        } catch (\SodiumException) {
            return false;
        }
    }
}
