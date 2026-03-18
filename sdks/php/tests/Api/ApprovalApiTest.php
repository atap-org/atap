<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\Approval;
use Atap\Sdk\Model\ApprovalSubject;
use Atap\Sdk\Tests\TestCase;

class ApprovalApiTest extends TestCase
{
    public function testCreate(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(201, [
                'id' => 'apr_test123',
                'state' => 'pending',
                'from' => 'did:web:localhost%3A8080:agent:a',
                'to' => 'did:web:localhost%3A8080:human:b',
                'subject' => [
                    'type' => 'data_access',
                    'label' => 'Access user data',
                    'payload' => ['scope' => 'read'],
                ],
                'created_at' => '2024-01-01T00:00:00Z',
            ]),
        ]);

        $subject = new ApprovalSubject('data_access', 'Access user data', payload: ['scope' => 'read']);
        $approval = $client->approvals()->create(
            'did:web:localhost%3A8080:agent:a',
            'did:web:localhost%3A8080:human:b',
            $subject,
        );

        $this->assertInstanceOf(Approval::class, $approval);
        $this->assertSame('apr_test123', $approval->id);
        $this->assertSame('pending', $approval->state);
        $this->assertNotNull($approval->subject);
        $this->assertSame('data_access', $approval->subject->type);
    }

    public function testRespond(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'apr_test123',
                'state' => 'approved',
                'responded_at' => '2024-01-01T00:01:00Z',
                'signatures' => ['approver' => 'sig_data'],
            ]),
        ]);

        $approval = $client->approvals()->respond('apr_test123', 'jws_signature');
        $this->assertSame('approved', $approval->state);
        $this->assertNotNull($approval->respondedAt);
    }

    public function testList(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'approvals' => [
                    ['id' => 'apr_1', 'state' => 'pending', 'from' => 'a', 'to' => 'b'],
                    ['id' => 'apr_2', 'state' => 'approved', 'from' => 'c', 'to' => 'd'],
                ],
            ]),
        ]);

        $approvals = $client->approvals()->list();
        $this->assertCount(2, $approvals);
        $this->assertSame('apr_1', $approvals[0]->id);
        $this->assertSame('apr_2', $approvals[1]->id);
    }

    public function testListWithDirectArray(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'items' => [
                    ['id' => 'apr_1', 'state' => 'pending', 'from' => 'a', 'to' => 'b'],
                ],
            ]),
        ]);

        $approvals = $client->approvals()->list();
        $this->assertCount(1, $approvals);
    }

    public function testRevoke(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'id' => 'apr_test123',
                'state' => 'revoked',
            ]),
        ]);

        $approval = $client->approvals()->revoke('apr_test123');
        $this->assertSame('revoked', $approval->state);
    }

    public function testCreateWithVia(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(201, [
                'id' => 'apr_via',
                'state' => 'pending',
                'via' => 'did:web:mediator',
                'from' => 'a',
                'to' => 'b',
            ]),
        ]);

        $subject = new ApprovalSubject('action', 'Test');
        $approval = $client->approvals()->create('a', 'b', $subject, 'did:web:mediator');
        $this->assertSame('did:web:mediator', $approval->via);
    }
}
