<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\DIDCommInbox;
use Atap\Sdk\Model\DIDCommMessage;

/**
 * Send and receive DIDComm messages.
 */
class DIDCommApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Send a DIDComm message (JWE envelope). Public endpoint.
     *
     * @param string $jweBytes Raw JWE bytes (application/didcomm-encrypted+json)
     * @return array<string, mixed> Dict with id and status ("queued")
     */
    public function send(string $jweBytes): array
    {
        return $this->client->getHttpClient()->request(
            'POST',
            '/v1/didcomm',
            headers: ['Content-Type' => 'application/didcomm-encrypted+json'],
            jsonBody: null,
        );
    }

    /**
     * Retrieve pending DIDComm messages. Requires atap:inbox scope.
     *
     * @param int $limit Max messages to return (default 50, max 100)
     */
    public function inbox(int $limit = 50): DIDCommInbox
    {
        $params = ['limit' => (string) min($limit, 100)];
        $data = $this->client->authedRequest('GET', '/v1/didcomm/inbox', params: $params);

        $messages = array_map(
            fn(array $m) => DIDCommMessage::fromArray($m),
            $data['messages'] ?? [],
        );

        return new DIDCommInbox(
            messages: $messages,
            count: (int) ($data['count'] ?? count($messages)),
        );
    }
}
