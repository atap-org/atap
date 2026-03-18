<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\Credential;
use Atap\Sdk\Tests\TestCase;

class CredentialApiTest extends TestCase
{
    public function testStartEmail(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, ['message' => 'OTP sent to user@example.com']),
        ]);

        $message = $client->credentials()->startEmail('user@example.com');
        $this->assertSame('OTP sent to user@example.com', $message);
    }

    public function testVerifyEmail(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'cred_123',
                'type' => 'ATAPEmailVerification',
                'credential' => 'eyJhbGciOi...',
                'issued_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $cred = $client->credentials()->verifyEmail('user@example.com', '123456');
        $this->assertInstanceOf(Credential::class, $cred);
        $this->assertSame('cred_123', $cred->id);
        $this->assertSame('ATAPEmailVerification', $cred->type);
    }

    public function testStartPhone(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, ['message' => 'OTP sent']),
        ]);

        $message = $client->credentials()->startPhone('+1234567890');
        $this->assertSame('OTP sent', $message);
    }

    public function testVerifyPhone(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'cred_456',
                'type' => 'ATAPPhoneVerification',
                'credential' => 'eyJhbGciOi...',
                'issued_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $cred = $client->credentials()->verifyPhone('+1234567890', '654321');
        $this->assertInstanceOf(Credential::class, $cred);
        $this->assertSame('ATAPPhoneVerification', $cred->type);
    }

    public function testSubmitPersonhood(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'cred_789',
                'type' => 'ATAPPersonhood',
                'credential' => 'eyJhbGciOi...',
                'issued_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $cred = $client->credentials()->submitPersonhood('provider_token_abc');
        $this->assertSame('ATAPPersonhood', $cred->type);
    }

    public function testSubmitPersonhoodNoToken(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'cred_no_token',
                'type' => 'ATAPPersonhood',
                'credential' => 'eyJhbGciOi...',
                'issued_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $cred = $client->credentials()->submitPersonhood();
        $this->assertSame('cred_no_token', $cred->id);
    }

    public function testListCredentials(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'credentials' => [
                    ['id' => 'cred_1', 'type' => 'ATAPEmailVerification', 'credential' => 'jwt1', 'issued_at' => '2024-01-01T00:00:00Z'],
                    ['id' => 'cred_2', 'type' => 'ATAPPhoneVerification', 'credential' => 'jwt2', 'issued_at' => '2024-01-02T00:00:00Z'],
                ],
            ]),
        ]);

        $creds = $client->credentials()->list();
        $this->assertCount(2, $creds);
        $this->assertSame('cred_1', $creds[0]->id);
    }

    public function testStatusList(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'id' => 'https://example.com/credentials/status/1',
                'type' => ['VerifiableCredential', 'BitstringStatusListCredential'],
            ]),
        ]);

        $data = $client->credentials()->statusList('1');
        $this->assertArrayHasKey('id', $data);
    }

    public function testStartEmailDefaultMessage(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, []),
        ]);

        $message = $client->credentials()->startEmail('test@test.com');
        $this->assertSame('OTP sent', $message);
    }
}
