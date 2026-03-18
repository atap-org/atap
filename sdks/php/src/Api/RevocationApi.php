<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\Revocation;
use Atap\Sdk\Model\RevocationList;

/**
 * Submit and query revocations.
 */
class RevocationApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Submit a revocation. Requires atap:revoke scope.
     *
     * @param string $approvalId The approval ID to revoke (apr_...)
     * @param string $signature JWS signature
     * @param string|null $validUntil Optional RFC3339 expiry
     */
    public function submit(
        string $approvalId,
        string $signature,
        ?string $validUntil = null,
    ): Revocation {
        $body = [
            'approval_id' => $approvalId,
            'signature' => $signature,
        ];
        if ($validUntil !== null) {
            $body['valid_until'] = $validUntil;
        }

        $data = $this->client->authedRequest('POST', '/v1/revocations', jsonBody: $body);

        return Revocation::fromArray($data);
    }

    /**
     * Query active revocations for an entity (public endpoint).
     *
     * @param string $entityDid The approver DID to query
     */
    public function list(string $entityDid): RevocationList
    {
        $data = $this->client->getHttpClient()->request(
            'GET',
            '/v1/revocations',
            params: ['entity' => $entityDid],
        );

        $revocations = array_map(
            fn(array $r) => Revocation::fromArray($r),
            $data['revocations'] ?? [],
        );

        return new RevocationList(
            entity: (string) ($data['entity'] ?? $entityDid),
            revocations: $revocations,
            checkedAt: (string) ($data['checked_at'] ?? ''),
        );
    }
}
