<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\DIDDocument;
use Atap\Sdk\Model\DiscoveryDocument;
use Atap\Sdk\Tests\TestCase;

class DiscoveryApiTest extends TestCase
{
    public function testDiscover(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'domain' => 'localhost:8080',
                'api_base' => 'http://localhost:8080/v1',
                'didcomm_endpoint' => 'http://localhost:8080/v1/didcomm',
                'claim_types' => ['email', 'phone', 'personhood'],
                'max_approval_ttl' => '24h',
                'trust_level' => 1,
                'oauth' => [
                    'token_endpoint' => '/v1/oauth/token',
                    'authorization_endpoint' => '/v1/oauth/authorize',
                ],
            ]),
        ]);

        $doc = $client->discovery()->discover();
        $this->assertInstanceOf(DiscoveryDocument::class, $doc);
        $this->assertSame('localhost:8080', $doc->domain);
        $this->assertSame('24h', $doc->maxApprovalTtl);
        $this->assertCount(3, $doc->claimTypes);
        $this->assertNotNull($doc->oauth);
    }

    public function testResolveDid(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'id' => 'did:web:localhost%3A8080:agent:test123',
                '@context' => ['https://www.w3.org/ns/did/v1'],
                'verificationMethod' => [
                    [
                        'id' => 'did:web:localhost%3A8080:agent:test123#key-0',
                        'type' => 'Ed25519VerificationKey2020',
                        'controller' => 'did:web:localhost%3A8080:agent:test123',
                        'publicKeyMultibase' => 'z6Mk...',
                    ],
                ],
                'authentication' => ['did:web:localhost%3A8080:agent:test123#key-0'],
                'atap:type' => 'agent',
            ]),
        ]);

        $doc = $client->discovery()->resolveDid('agent', 'test123');
        $this->assertInstanceOf(DIDDocument::class, $doc);
        $this->assertSame('did:web:localhost%3A8080:agent:test123', $doc->id);
        $this->assertCount(1, $doc->verificationMethod);
        $this->assertSame('Ed25519VerificationKey2020', $doc->verificationMethod[0]->type);
        $this->assertSame('agent', $doc->atapType);
    }

    public function testServerDid(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'id' => 'did:web:localhost%3A8080:server:platform',
                '@context' => ['https://www.w3.org/ns/did/v1'],
                'verificationMethod' => [],
                'service' => [
                    [
                        'id' => '#atap',
                        'type' => 'ATAPService',
                        'serviceEndpoint' => 'https://localhost:8080/v1',
                    ],
                ],
            ]),
        ]);

        $doc = $client->discovery()->serverDid();
        $this->assertSame('did:web:localhost%3A8080:server:platform', $doc->id);
        $this->assertCount(1, $doc->service);
    }

    public function testHealth(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'status' => 'ok',
                'version' => '1.0.0',
            ]),
        ]);

        $health = $client->discovery()->health();
        $this->assertSame('ok', $health['status']);
    }

    public function testDiscoverMinimal(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'domain' => 'example.com',
            ]),
        ]);

        $doc = $client->discovery()->discover();
        $this->assertSame('example.com', $doc->domain);
        $this->assertSame('', $doc->apiBase);
        $this->assertNull($doc->oauth);
    }
}
