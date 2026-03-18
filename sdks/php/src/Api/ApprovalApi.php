<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\Approval;
use Atap\Sdk\Model\ApprovalSubject;

/**
 * Create, respond to, list, and revoke approvals.
 */
class ApprovalApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Create an approval request. Requires atap:send scope.
     *
     * @param string $fromDid Requester DID
     * @param string $toDid Approver DID (or org DID for fan-out)
     * @param ApprovalSubject $subject The approval subject
     * @param string|null $via Optional mediating system DID
     */
    public function create(
        string $fromDid,
        string $toDid,
        ApprovalSubject $subject,
        ?string $via = null,
    ): Approval {
        $body = [
            'from' => $fromDid,
            'to' => $toDid,
            'subject' => $subject->toArray(),
        ];
        if ($via !== null) {
            $body['via'] = $via;
        }

        $data = $this->client->authedRequest('POST', '/v1/approvals', jsonBody: $body);

        return Approval::fromArray($data);
    }

    /**
     * Respond to an approval (approve). Requires atap:send scope.
     *
     * @param string $approvalId The approval ID (apr_...)
     * @param string $signature JWS signature from the approver
     */
    public function respond(string $approvalId, string $signature): Approval
    {
        $data = $this->client->authedRequest(
            'POST',
            "/v1/approvals/{$approvalId}/respond",
            jsonBody: ['signature' => $signature],
        );

        return Approval::fromArray($data);
    }

    /**
     * List approvals addressed to the authenticated entity. Requires atap:inbox scope.
     *
     * @return Approval[]
     */
    public function list(): array
    {
        $data = $this->client->authedRequest('GET', '/v1/approvals');

        $items = $data;
        if (!isset($data[0])) {
            $items = $data['approvals'] ?? $data['items'] ?? [];
        }

        return array_map(fn(array $a) => Approval::fromArray($a), $items);
    }

    /**
     * Revoke an approval. Requires atap:revoke scope.
     */
    public function revoke(string $approvalId): Approval
    {
        $data = $this->client->authedRequest('DELETE', "/v1/approvals/{$approvalId}");

        return Approval::fromArray($data);
    }
}
