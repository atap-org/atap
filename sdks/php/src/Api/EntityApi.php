<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\Entity;
use Atap\Sdk\Model\KeyVersion;

/**
 * Entity registration, retrieval, deletion, and key rotation.
 */
class EntityApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Register a new entity.
     *
     * @param string $entityType One of "agent", "machine", "human", "org"
     * @param string $name Optional display name
     * @param string|null $publicKey Optional base64-encoded Ed25519 public key
     * @param string|null $principalDid Optional DID for agent-to-principal binding
     */
    public function register(
        string $entityType,
        string $name = '',
        ?string $publicKey = null,
        ?string $principalDid = null,
    ): Entity {
        $body = ['type' => $entityType];
        if ($name !== '') {
            $body['name'] = $name;
        }
        if ($publicKey !== null) {
            $body['public_key'] = $publicKey;
        }
        if ($principalDid !== null) {
            $body['principal_did'] = $principalDid;
        }

        $data = $this->client->getHttpClient()->request('POST', '/v1/entities', $body);

        return Entity::fromArray($data);
    }

    /**
     * Get public entity info by ID.
     */
    public function get(string $entityId): Entity
    {
        $data = $this->client->getHttpClient()->request('GET', "/v1/entities/{$entityId}");

        return Entity::fromArray($data);
    }

    /**
     * Delete an entity (crypto-shred). Requires atap:manage scope.
     */
    public function delete(string $entityId): void
    {
        $this->client->authedRequest('DELETE', "/v1/entities/{$entityId}");
    }

    /**
     * Rotate an entity's Ed25519 public key. Requires atap:manage scope.
     *
     * @param string $entityId The entity ID
     * @param string $publicKey Base64-encoded new Ed25519 public key
     */
    public function rotateKey(string $entityId, string $publicKey): KeyVersion
    {
        $data = $this->client->authedRequest(
            'POST',
            "/v1/entities/{$entityId}/keys/rotate",
            jsonBody: ['public_key' => $publicKey],
        );

        return KeyVersion::fromArray($data);
    }
}
