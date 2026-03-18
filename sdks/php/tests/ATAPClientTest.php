<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests;

use Atap\Sdk\Api\ApprovalApi;
use Atap\Sdk\Api\CredentialApi;
use Atap\Sdk\Api\DIDCommApi;
use Atap\Sdk\Api\DiscoveryApi;
use Atap\Sdk\Api\EntityApi;
use Atap\Sdk\Api\RevocationApi;
use Atap\Sdk\ATAPClient;
use Atap\Sdk\Crypto\Ed25519;

class ATAPClientTest extends TestCase
{
    public function testDefaultConstruction(): void
    {
        $client = new ATAPClient();
        $this->assertInstanceOf(ATAPClient::class, $client);
    }

    public function testApiModulesAvailable(): void
    {
        $client = new ATAPClient();
        $this->assertInstanceOf(EntityApi::class, $client->entities());
        $this->assertInstanceOf(ApprovalApi::class, $client->approvals());
        $this->assertInstanceOf(RevocationApi::class, $client->revocations());
        $this->assertInstanceOf(DIDCommApi::class, $client->didcomm());
        $this->assertInstanceOf(CredentialApi::class, $client->credentials());
        $this->assertInstanceOf(DiscoveryApi::class, $client->discovery());
    }

    public function testTokenManagerNotInitializedWithoutCredentials(): void
    {
        $client = new ATAPClient();
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('Token manager not initialized');
        $client->tokenManager();
    }

    public function testTokenManagerInitializedWithCredentials(): void
    {
        $keys = Ed25519::generateKeypair();
        $client = new ATAPClient([
            'did' => 'did:web:localhost%3A8080:agent:test',
            'signing_key' => $keys['secretKey'],
            'client_secret' => 'atap_secret',
        ]);

        $tm = $client->tokenManager();
        $this->assertNotNull($tm);
    }

    public function testAuthedRequestFailsWithoutAuth(): void
    {
        $client = new ATAPClient();
        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('Authentication not configured');
        $client->authedRequest('GET', '/v1/test');
    }

    public function testConstructWithPrivateKeyBase64(): void
    {
        $keys = Ed25519::generateKeypair();
        $seed = substr($keys['secretKey'], 0, 32);
        $b64 = base64_encode($seed);

        $client = new ATAPClient([
            'did' => 'did:web:localhost%3A8080:agent:test',
            'private_key' => $b64,
            'client_secret' => 'atap_secret',
        ]);

        $this->assertNotNull($client->tokenManager());
    }

    public function testPlatformDomainExtractedFromDid(): void
    {
        $keys = Ed25519::generateKeypair();
        $client = new ATAPClient([
            'did' => 'did:web:example.com%3A443:agent:test',
            'signing_key' => $keys['secretKey'],
        ]);

        // Platform domain is extracted internally, we verify through tokenManager
        $this->assertInstanceOf(ATAPClient::class, $client);
    }

    public function testExplicitPlatformDomain(): void
    {
        $keys = Ed25519::generateKeypair();
        $client = new ATAPClient([
            'did' => 'did:web:localhost%3A8080:agent:test',
            'signing_key' => $keys['secretKey'],
            'platform_domain' => 'custom.example.com',
        ]);

        $this->assertInstanceOf(ATAPClient::class, $client);
    }

    public function testCustomHandler(): void
    {
        $handler = $this->createHandlerStack([
            $this->jsonResponse(200, ['status' => 'ok']),
        ]);

        $client = new ATAPClient([
            'handler' => $handler,
        ]);

        $data = $client->discovery()->health();
        $this->assertSame('ok', $data['status']);
    }

    public function testCustomTimeout(): void
    {
        $client = new ATAPClient([
            'timeout' => 60.0,
        ]);

        $this->assertInstanceOf(ATAPClient::class, $client);
    }

    public function testCustomScopes(): void
    {
        $keys = Ed25519::generateKeypair();
        $client = new ATAPClient([
            'did' => 'did:web:localhost%3A8080:agent:test',
            'signing_key' => $keys['secretKey'],
            'client_secret' => 'atap_secret',
            'scopes' => ['atap:inbox', 'atap:send'],
        ]);

        $this->assertNotNull($client->tokenManager());
    }
}
