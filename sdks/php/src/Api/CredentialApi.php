<?php

declare(strict_types=1);

namespace Atap\Sdk\Api;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\Credential;

/**
 * Email/phone/personhood verification and credential management.
 */
class CredentialApi
{
    public function __construct(
        private readonly ATAPClient $client,
    ) {
    }

    /**
     * Initiate email verification (OTP). Requires atap:manage scope.
     */
    public function startEmail(string $email): string
    {
        $data = $this->client->authedRequest(
            'POST',
            '/v1/credentials/email/start',
            jsonBody: ['email' => $email],
        );

        return (string) ($data['message'] ?? 'OTP sent');
    }

    /**
     * Verify email with OTP, issuing ATAPEmailVerification VC. Requires atap:manage scope.
     */
    public function verifyEmail(string $email, string $otp): Credential
    {
        $data = $this->client->authedRequest(
            'POST',
            '/v1/credentials/email/verify',
            jsonBody: ['email' => $email, 'otp' => $otp],
        );

        return Credential::fromArray($data);
    }

    /**
     * Initiate phone verification (OTP). Requires atap:manage scope.
     */
    public function startPhone(string $phone): string
    {
        $data = $this->client->authedRequest(
            'POST',
            '/v1/credentials/phone/start',
            jsonBody: ['phone' => $phone],
        );

        return (string) ($data['message'] ?? 'OTP sent');
    }

    /**
     * Verify phone with OTP, issuing ATAPPhoneVerification VC. Requires atap:manage scope.
     */
    public function verifyPhone(string $phone, string $otp): Credential
    {
        $data = $this->client->authedRequest(
            'POST',
            '/v1/credentials/phone/verify',
            jsonBody: ['phone' => $phone, 'otp' => $otp],
        );

        return Credential::fromArray($data);
    }

    /**
     * Submit personhood attestation, issuing ATAPPersonhood VC. Requires atap:manage scope.
     */
    public function submitPersonhood(?string $providerToken = null): Credential
    {
        $body = [];
        if ($providerToken !== null) {
            $body['provider_token'] = $providerToken;
        }

        $data = $this->client->authedRequest(
            'POST',
            '/v1/credentials/personhood',
            jsonBody: $body,
        );

        return Credential::fromArray($data);
    }

    /**
     * List credentials for the authenticated entity. Requires atap:manage scope.
     *
     * @return Credential[]
     */
    public function list(): array
    {
        $data = $this->client->authedRequest('GET', '/v1/credentials');

        $items = $data;
        if (!isset($data[0])) {
            $items = $data['credentials'] ?? [];
        }

        return array_map(fn(array $c) => Credential::fromArray($c), $items);
    }

    /**
     * Get W3C Bitstring Status List VC (public endpoint).
     *
     * @return array<string, mixed>
     */
    public function statusList(string $listId = '1'): array
    {
        return $this->client->getHttpClient()->request('GET', "/v1/credentials/status/{$listId}");
    }
}
