<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\Entity;
use Atap\Sdk\Model\KeyVersion;
use Atap\Sdk\Tests\TestCase;

class EntityApiTest extends TestCase
{
    public function testRegister(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(201, [
                'id' => 'entity123',
                'type' => 'agent',
                'did' => 'did:web:localhost%3A8080:agent:entity123',
                'name' => 'Test Agent',
                'key_id' => 'key_123',
                'public_key' => 'pubkey_b64',
                'client_secret' => 'atap_secret123',
            ]),
        ]);

        $entity = $client->entities()->register('agent', 'Test Agent');

        $this->assertInstanceOf(Entity::class, $entity);
        $this->assertSame('entity123', $entity->id);
        $this->assertSame('agent', $entity->type);
        $this->assertSame('Test Agent', $entity->name);
        $this->assertSame('atap_secret123', $entity->clientSecret);
    }

    public function testRegisterWithPublicKey(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(201, [
                'id' => 'entity456',
                'type' => 'machine',
                'did' => 'did:web:localhost%3A8080:machine:entity456',
                'public_key' => 'my_pubkey',
            ]),
        ]);

        $entity = $client->entities()->register('machine', '', 'my_pubkey');
        $this->assertSame('entity456', $entity->id);
        $this->assertSame('machine', $entity->type);
    }

    public function testGet(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'id' => 'entity789',
                'type' => 'human',
                'did' => 'did:web:localhost%3A8080:human:entity789',
                'trust_level' => 2,
            ]),
        ]);

        $entity = $client->entities()->get('entity789');
        $this->assertSame('entity789', $entity->id);
        $this->assertSame('human', $entity->type);
        $this->assertSame(2, $entity->trustLevel);
    }

    public function testDelete(): void
    {
        $client = $this->createAuthedClient([
            $this->noContentResponse(),
        ]);

        // Should not throw
        $client->entities()->delete('entity123');
        $this->assertTrue(true);
    }

    public function testRotateKey(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'key_new_123',
                'entity_id' => 'entity123',
                'key_index' => 2,
                'valid_from' => '2024-01-01T00:00:00Z',
                'created_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $keyVersion = $client->entities()->rotateKey('entity123', 'new_pubkey_b64');
        $this->assertInstanceOf(KeyVersion::class, $keyVersion);
        $this->assertSame('key_new_123', $keyVersion->id);
        $this->assertSame(2, $keyVersion->keyIndex);
    }

    public function testRegisterWithPrincipalDid(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(201, [
                'id' => 'agent_sub',
                'type' => 'agent',
                'principal_did' => 'did:web:localhost%3A8080:human:parent',
            ]),
        ]);

        $entity = $client->entities()->register(
            'agent',
            '',
            null,
            'did:web:localhost%3A8080:human:parent',
        );
        $this->assertSame('did:web:localhost%3A8080:human:parent', $entity->principalDid);
    }
}
