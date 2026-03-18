<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\DIDDocument;
use Atap\Sdk\Model\DiscoveryDocument;

/**
 * Server discovery and DID document resolution.
 */
class DiscoveryApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Fetch the server discovery document from /.well-known/atap.json.
     */
    public function discover(): DiscoveryDocument
    {
        $data = $this->client->getHttpClient()->request('GET', '/.well-known/atap.json');

        return DiscoveryDocument::fromArray($data);
    }

    /**
     * Resolve an entity's DID Document.
     *
     * @param string $entityType Entity type (agent, machine, human, org)
     * @param string $entityId Entity ID
     */
    public function resolveDid(string $entityType, string $entityId): DIDDocument
    {
        $data = $this->client->getHttpClient()->request('GET', "/{$entityType}/{$entityId}/did.json");

        return DIDDocument::fromArray($data);
    }

    /**
     * Fetch the server's DID Document.
     */
    public function serverDid(): DIDDocument
    {
        $data = $this->client->getHttpClient()->request('GET', '/server/platform/did.json');

        return DIDDocument::fromArray($data);
    }

    /**
     * Check server health.
     *
     * @return array<string, mixed>
     */
    public function health(): array
    {
        return $this->client->getHttpClient()->request('GET', '/v1/health');
    }
}
