<?php

declare(strict_types=1);

namespace Atap\Sdk;

use Atap\Sdk\Api\ApprovalApi;
use Atap\Sdk\Api\CredentialApi;
use Atap\Sdk\Api\DIDCommApi;
use Atap\Sdk\Api\DiscoveryApi;
use Atap\Sdk\Api\EntityApi;
use Atap\Sdk\Api\RevocationApi;
use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Http\HttpClient;
use Atap\Sdk\OAuth\TokenManager;
use GuzzleHttp\HandlerStack;

/**
 * High-level client for the ATAP platform.
 *
 * Usage for agent/machine (client_credentials):
 *     $client = new ATAPClient([
 *         'base_url' => 'http://localhost:8080',
 *         'did' => 'did:web:localhost%3A8080:agent:abc',
 *         'private_key' => '<base64 Ed25519 seed>',
 *         'client_secret' => 'atap_...',
 *     ]);
 *
 * Usage for human/org (authorization_code + PKCE):
 *     $client = new ATAPClient([
 *         'base_url' => 'http://localhost:8080',
 *         'did' => 'did:web:localhost%3A8080:human:abc',
 *         'private_key' => '<base64 Ed25519 seed>',
 *     ]);
 *     $client->tokenManager()->obtainAuthorizationCode();
 */
class ATAPClient
{
    private HttpClient $http;
    private string $did;
    private string $platformDomain;
    private ?string $signingKey;
    private ?TokenManager $tokenManager;

    private EntityApi $entities;
    private ApprovalApi $approvals;
    private RevocationApi $revocations;
    private DIDCommApi $didcomm;
    private CredentialApi $credentials;
    private DiscoveryApi $discovery;

    /**
     * @param array<string, mixed> $config {
     *     base_url?: string,
     *     did?: string,
     *     private_key?: string,
     *     signing_key?: string (raw 64-byte secret key),
     *     client_secret?: string,
     *     scopes?: string[],
     *     platform_domain?: string,
     *     timeout?: float,
     *     handler?: HandlerStack (for testing),
     * }
     */
    public function __construct(array $config = [])
    {
        $baseUrl = (string) ($config['base_url'] ?? 'http://localhost:8080');
        $timeout = (float) ($config['timeout'] ?? 30.0);
        $handler = $config['handler'] ?? null;

        $this->http = new HttpClient($baseUrl, $timeout, $handler);
        $this->did = (string) ($config['did'] ?? '');
        $this->platformDomain = (string) ($config['platform_domain'] ?? '');

        if ($this->platformDomain === '' && $this->did !== '') {
            $this->platformDomain = DomainUtils::domainFromDid($this->did);
        }
        if ($this->platformDomain === '') {
            $this->platformDomain = 'localhost';
        }

        // Load signing key
        $this->signingKey = null;
        if (isset($config['signing_key'])) {
            $this->signingKey = (string) $config['signing_key'];
        } elseif (isset($config['private_key'])) {
            $this->signingKey = Ed25519::loadSigningKey((string) $config['private_key']);
        }

        // Initialize token manager
        $this->tokenManager = null;
        if ($this->signingKey !== null && $this->did !== '') {
            $this->tokenManager = new TokenManager(
                httpClient: $this->http,
                signingKey: $this->signingKey,
                did: $this->did,
                clientSecret: isset($config['client_secret']) ? (string) $config['client_secret'] : null,
                scopes: $config['scopes'] ?? null,
                platformDomain: $this->platformDomain,
            );
        }

        // API modules
        $this->entities = new EntityApi($this);
        $this->approvals = new ApprovalApi($this);
        $this->revocations = new RevocationApi($this);
        $this->didcomm = new DIDCommApi($this);
        $this->credentials = new CredentialApi($this);
        $this->discovery = new DiscoveryApi($this);
    }

    public function entities(): EntityApi
    {
        return $this->entities;
    }

    public function approvals(): ApprovalApi
    {
        return $this->approvals;
    }

    public function revocations(): RevocationApi
    {
        return $this->revocations;
    }

    public function didcomm(): DIDCommApi
    {
        return $this->didcomm;
    }

    public function credentials(): CredentialApi
    {
        return $this->credentials;
    }

    public function discovery(): DiscoveryApi
    {
        return $this->discovery;
    }

    /**
     * Access the token manager for manual token operations.
     */
    public function tokenManager(): TokenManager
    {
        if ($this->tokenManager === null) {
            throw new \RuntimeException('Token manager not initialized. Provide did and private_key.');
        }

        return $this->tokenManager;
    }

    /**
     * Get the HTTP client (used by API modules).
     */
    public function getHttpClient(): HttpClient
    {
        return $this->http;
    }

    /**
     * Make an authenticated request using the token manager.
     *
     * @param array<string, mixed>|null $jsonBody
     * @param array<string, string>|null $params
     * @return array<string, mixed>
     */
    public function authedRequest(
        string $method,
        string $path,
        ?array $jsonBody = null,
        ?string $rawBody = null,
        ?string $contentType = null,
        ?array $params = null,
    ): array {
        if ($this->tokenManager === null || $this->signingKey === null) {
            throw new \RuntimeException(
                'Authentication not configured. Provide did, private_key, and optionally client_secret.'
            );
        }

        $accessToken = $this->tokenManager->getAccessToken();

        return $this->http->authenticatedRequest(
            $method,
            $path,
            signingKey: $this->signingKey,
            accessToken: $accessToken,
            platformDomain: $this->platformDomain,
            jsonBody: $jsonBody,
            rawBody: $rawBody,
            contentType: $contentType,
            params: $params,
        );
    }
}
