<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Api;

use Atap\Sdk\Model\DIDCommInbox;
use Atap\Sdk\Tests\TestCase;

class DIDCommApiTest extends TestCase
{
    public function testSend(): void
    {
        $client = $this->createUnauthClient([
            $this->jsonResponse(202, [
                'id' => 'msg_123',
                'status' => 'queued',
            ]),
        ]);

        $result = $client->didcomm()->send('{"protected":"...","ciphertext":"..."}');
        $this->assertSame('msg_123', $result['id']);
        $this->assertSame('queued', $result['status']);
    }

    public function testInbox(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'messages' => [
                    [
                        'id' => 'msg_1',
                        'sender_did' => 'did:web:sender',
                        'message_type' => 'text',
                        'payload' => 'Hello',
                        'created_at' => '2024-01-01T00:00:00Z',
                    ],
                    [
                        'id' => 'msg_2',
                        'sender_did' => 'did:web:sender2',
                        'message_type' => 'json',
                        'payload' => '{"key":"val"}',
                        'created_at' => '2024-01-01T00:01:00Z',
                    ],
                ],
                'count' => 2,
            ]),
        ]);

        $inbox = $client->didcomm()->inbox();
        $this->assertInstanceOf(DIDCommInbox::class, $inbox);
        $this->assertCount(2, $inbox->messages);
        $this->assertSame(2, $inbox->count);
        $this->assertSame('msg_1', $inbox->messages[0]->id);
        $this->assertSame('did:web:sender', $inbox->messages[0]->senderDid);
    }

    public function testInboxEmpty(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'messages' => [],
                'count' => 0,
            ]),
        ]);

        $inbox = $client->didcomm()->inbox();
        $this->assertCount(0, $inbox->messages);
        $this->assertSame(0, $inbox->count);
    }

    public function testInboxWithLimit(): void
    {
        $client = $this->createAuthedClient([
            $this->jsonResponse(200, [
                'messages' => [
                    ['id' => 'msg_1', 'sender_did' => 'a', 'message_type' => 'x', 'payload' => '', 'created_at' => ''],
                ],
                'count' => 1,
            ]),
        ]);

        $inbox = $client->didcomm()->inbox(10);
        $this->assertSame(1, $inbox->count);
    }
}
