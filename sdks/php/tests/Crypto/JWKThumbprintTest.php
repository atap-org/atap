<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Crypto;

use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Crypto\JWKThumbprint;
use Atap\Sdk\Tests\TestCase;

class JWKThumbprintTest extends TestCase
{
    public function testComputeReturnsString(): void
    {
        $keys = Ed25519::generateKeypair();
        $thumbprint = JWKThumbprint::compute($keys['publicKey']);

        $this->assertIsString($thumbprint);
        $this->assertNotEmpty($thumbprint);
    }

    public function testComputeIsDeterministic(): void
    {
        $keys = Ed25519::generateKeypair();

        $t1 = JWKThumbprint::compute($keys['publicKey']);
        $t2 = JWKThumbprint::compute($keys['publicKey']);

        $this->assertSame($t1, $t2);
    }

    public function testDifferentKeysProduceDifferentThumbprints(): void
    {
        $keys1 = Ed25519::generateKeypair();
        $keys2 = Ed25519::generateKeypair();

        $t1 = JWKThumbprint::compute($keys1['publicKey']);
        $t2 = JWKThumbprint::compute($keys2['publicKey']);

        $this->assertNotSame($t1, $t2);
    }

    public function testThumbprintIsBase64Url(): void
    {
        $keys = Ed25519::generateKeypair();
        $thumbprint = JWKThumbprint::compute($keys['publicKey']);

        // Should not contain +, /, or = (base64url)
        $this->assertDoesNotMatchRegularExpression('/[+\/=]/', $thumbprint);
    }

    public function testThumbprintLength(): void
    {
        $keys = Ed25519::generateKeypair();
        $thumbprint = JWKThumbprint::compute($keys['publicKey']);

        // SHA-256 = 32 bytes, base64url encoded = 43 chars
        $this->assertSame(43, strlen($thumbprint));
    }
}
