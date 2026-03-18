<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\Revocation;
use Atap\Sdk\Model\RevocationList;
use Atap\Sdk\Tests\TestCase;

class RevocationApiTest extends TestCase
{
    public function testSubmit(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(201, [
                'id' => 'rev_test123',
                'approval_id' => 'apr_test456',
                'approver_did' => 'did:web:approver',
                'revoked_at' => '2024-01-01T00:00:00Z',
                'expires_at' => '2024-01-01T01:00:00Z',
            ]),
        ]);

        $revocation = $client->revocations()->submit('apr_test456', 'jws_sig');
        $this->assertInstanceOf(Revocation::class, $revocation);
        $this->assertSame('rev_test123', $revocation->id);
        $this->assertSame('apr_test456', $revocation->approvalId);
    }

    public function testSubmitWithValidUntil(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(201, [
                'id' => 'rev_test789',
                'approval_id' => 'apr_test',
                'expires_at' => '2025-01-01T00:00:00Z',
            ]),
        ]);

        $revocation = $client->revocations()->submit('apr_test', 'sig', '2025-01-01T00:00:00Z');
        $this->assertSame('2025-01-01T00:00:00Z', $revocation->expiresAt);
    }

    public function testList(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'entity' => 'did:web:test',
                'revocations' => [
                    [
                        'id' => 'rev_1',
                        'approval_id' => 'apr_1',
                        'approver_did' => 'did:web:test',
                        'revoked_at' => '2024-01-01T00:00:00Z',
                        'expires_at' => '2024-01-01T01:00:00Z',
                    ],
                    [
                        'id' => 'rev_2',
                        'approval_id' => 'apr_2',
                        'approver_did' => 'did:web:test',
                        'revoked_at' => '2024-01-02T00:00:00Z',
                        'expires_at' => '2024-01-02T01:00:00Z',
                    ],
                ],
                'checked_at' => '2024-01-02T00:00:00Z',
            ]),
        ]);

        $list = $client->revocations()->list('did:web:test');
        $this->assertInstanceOf(RevocationList::class, $list);
        $this->assertSame('did:web:test', $list->entity);
        $this->assertCount(2, $list->revocations);
        $this->assertSame('rev_1', $list->revocations[0]->id);
    }

    public function testListEmpty(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(200, [
                'entity' => 'did:web:nobody',
                'revocations' => [],
                'checked_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $list = $client->revocations()->list('did:web:nobody');
        $this->assertCount(0, $list->revocations);
    }
}
