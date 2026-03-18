<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Crypto;

use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Tests\TestCase;

class Ed25519Test extends TestCase
{
    public function testGenerateKeypair(): void
    {
        $keys = Ed25519::generateKeypair();

        $this->assertArrayHasKey('secretKey', $keys);
        $this->assertArrayHasKey('publicKey', $keys);
        $this->assertSame(SODIUM_CRYPTO_SIGN_SECRETKEYBYTES, strlen($keys['secretKey']));
        $this->assertSame(SODIUM_CRYPTO_SIGN_PUBLICKEYBYTES, strlen($keys['publicKey']));
    }

    public function testLoadSigningKeyFromSeed(): void
    {
        $keys = Ed25519::generateKeypair();
        // Extract 32-byte seed from 64-byte secret key
        $seed = substr($keys['secretKey'], 0, 32);
        $seedB64 = base64_encode($seed);

        $loaded = Ed25519::loadSigningKey($seedB64);
        $this->assertSame(SODIUM_CRYPTO_SIGN_SECRETKEYBYTES, strlen($loaded));

        // Public key should match
        $this->assertSame($keys['publicKey'], Ed25519::getPublicKey($loaded));
    }

    public function testLoadSigningKeyFromFullKey(): void
    {
        $keys = Ed25519::generateKeypair();
        $fullB64 = base64_encode($keys['secretKey']);

        $loaded = Ed25519::loadSigningKey($fullB64);
        $this->assertSame($keys['secretKey'], $loaded);
    }

    public function testLoadSigningKeyInvalidLength(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Invalid private key length');
        Ed25519::loadSigningKey(base64_encode('short'));
    }

    public function testLoadSigningKeyInvalidBase64(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        Ed25519::loadSigningKey('!!!not-base64!!!');
    }

    public function testSignAndVerify(): void
    {
        $keys = Ed25519::generateKeypair();
        $data = 'Hello, ATAP!';

        $signature = Ed25519::sign($data, $keys['secretKey']);
        $this->assertSame(SODIUM_CRYPTO_SIGN_BYTES, strlen($signature));

        $this->assertTrue(Ed25519::verify($signature, $data, $keys['publicKey']));
    }

    public function testVerifyFailsWithWrongData(): void
    {
        $keys = Ed25519::generateKeypair();
        $signature = Ed25519::sign('original', $keys['secretKey']);

        $this->assertFalse(Ed25519::verify($signature, 'tampered', $keys['publicKey']));
    }

    public function testVerifyFailsWithWrongKey(): void
    {
        $keys1 = Ed25519::generateKeypair();
        $keys2 = Ed25519::generateKeypair();
        $signature = Ed25519::sign('data', $keys1['secretKey']);

        $this->assertFalse(Ed25519::verify($signature, 'data', $keys2['publicKey']));
    }

    public function testGetPublicKey(): void
    {
        $keys = Ed25519::generateKeypair();
        $extracted = Ed25519::getPublicKey($keys['secretKey']);

        $this->assertSame($keys['publicKey'], $extracted);
    }

    public function testDeterministicKeypairFromSeed(): void
    {
        $seed = random_bytes(32);
        $key1 = Ed25519::loadSigningKey(base64_encode($seed));
        $key2 = Ed25519::loadSigningKey(base64_encode($seed));

        $this->assertSame($key1, $key2);
    }
}
