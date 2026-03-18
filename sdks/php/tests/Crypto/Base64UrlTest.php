<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Crypto;

use Atap\Sdk\Crypto\Base64Url;
use Atap\Sdk\Tests\TestCase;

class Base64UrlTest extends TestCase
{
    public function testEncodeEmpty(): void
    {
        $this->assertSame('', Base64Url::encode(''));
    }

    public function testDecodeEmpty(): void
    {
        $this->assertSame('', Base64Url::decode(''));
    }

    public function testRoundTrip(): void
    {
        $data = random_bytes(32);
        $encoded = Base64Url::encode($data);
        $decoded = Base64Url::decode($encoded);
        $this->assertSame($data, $decoded);
    }

    public function testNoPadding(): void
    {
        $encoded = Base64Url::encode('test');
        $this->assertStringNotContainsString('=', $encoded);
    }

    public function testUrlSafe(): void
    {
        // Use data that produces + and / in standard base64
        $data = "\xff\xff\xff";
        $encoded = Base64Url::encode($data);
        $this->assertStringNotContainsString('+', $encoded);
        $this->assertStringNotContainsString('/', $encoded);
    }

    public function testDecodeWithPadding(): void
    {
        // Standard base64 with padding
        $data = 'test';
        $standard = base64_encode($data);
        $urlSafe = rtrim(strtr($standard, '+/', '-_'), '=');
        $this->assertSame($data, Base64Url::decode($urlSafe));
    }

    public function testInvalidBase64Throws(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        Base64Url::decode('!!!invalid!!!');
    }

    public function testKnownValues(): void
    {
        // "Hello" in base64url should be "SGVsbG8"
        $this->assertSame('SGVsbG8', Base64Url::encode('Hello'));
        $this->assertSame('Hello', Base64Url::decode('SGVsbG8'));
    }

    public function testBinaryData(): void
    {
        $data = "\x00\x01\x02\x03\xfe\xff";
        $encoded = Base64Url::encode($data);
        $this->assertSame($data, Base64Url::decode($encoded));
    }
}
