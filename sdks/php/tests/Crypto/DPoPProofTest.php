<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Crypto;

use Atap\Sdk\Crypto\Base64Url;
use Atap\Sdk\Crypto\DPoPProof;
use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Tests\TestCase;

class DPoPProofTest extends TestCase
{
    public function testCreateProducesThreeParts(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts = explode('.', $proof);
        $this->assertCount(3, $parts);
    }

    public function testHeaderHasCorrectFields(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts = explode('.', $proof);
        $header = json_decode(Base64Url::decode($parts[0]), true);

        $this->assertSame('dpop+jwt', $header['typ']);
        $this->assertSame('EdDSA', $header['alg']);
        $this->assertArrayHasKey('jwk', $header);
        $this->assertSame('OKP', $header['jwk']['kty']);
        $this->assertSame('Ed25519', $header['jwk']['crv']);
        $this->assertArrayHasKey('x', $header['jwk']);
    }

    public function testPayloadHasRequiredClaims(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'GET', 'https://example.com/resource');

        $parts = explode('.', $proof);
        $payload = json_decode(Base64Url::decode($parts[1]), true);

        $this->assertArrayHasKey('jti', $payload);
        $this->assertSame('GET', $payload['htm']);
        $this->assertSame('https://example.com/resource', $payload['htu']);
        $this->assertArrayHasKey('iat', $payload);
        $this->assertIsInt($payload['iat']);
    }

    public function testPayloadIncludesAthWhenAccessTokenProvided(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create(
            $keys['secretKey'],
            'POST',
            'https://example.com/resource',
            'my_access_token',
        );

        $parts = explode('.', $proof);
        $payload = json_decode(Base64Url::decode($parts[1]), true);

        $this->assertArrayHasKey('ath', $payload);

        // Verify ath is SHA-256 of access token
        $expectedAth = Base64Url::encode(hash('sha256', 'my_access_token', true));
        $this->assertSame($expectedAth, $payload['ath']);
    }

    public function testPayloadHasNoAthWithoutAccessToken(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts = explode('.', $proof);
        $payload = json_decode(Base64Url::decode($parts[1]), true);

        $this->assertArrayNotHasKey('ath', $payload);
    }

    public function testSignatureIsValid(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts = explode('.', $proof);
        $signingInput = $parts[0] . '.' . $parts[1];
        $signature = Base64Url::decode($parts[2]);

        $this->assertTrue(Ed25519::verify($signature, $signingInput, $keys['publicKey']));
    }

    public function testJtiIsUnique(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof1 = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');
        $proof2 = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts1 = explode('.', $proof1);
        $parts2 = explode('.', $proof2);
        $payload1 = json_decode(Base64Url::decode($parts1[1]), true);
        $payload2 = json_decode(Base64Url::decode($parts2[1]), true);

        $this->assertNotSame($payload1['jti'], $payload2['jti']);
    }

    public function testPublicKeyInHeaderMatchesSecretKey(): void
    {
        $keys = Ed25519::generateKeypair();
        $proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');

        $parts = explode('.', $proof);
        $header = json_decode(Base64Url::decode($parts[0]), true);
        $xBytes = Base64Url::decode($header['jwk']['x']);

        $this->assertSame($keys['publicKey'], $xBytes);
    }
}
