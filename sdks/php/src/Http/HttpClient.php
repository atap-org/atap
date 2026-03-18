<?php

declare(strict_types=1);

namespace Atap\Sdk\Http;

use Atap\Sdk\Crypto\DPoPProof;
use Atap\Sdk\Exception\ATAPAuthException;
use Atap\Sdk\Exception\ATAPConflictException;
use Atap\Sdk\Exception\ATAPException;
use Atap\Sdk\Exception\ATAPNotFoundException;
use Atap\Sdk\Exception\ATAPProblemException;
use Atap\Sdk\Exception\ATAPRateLimitException;
use Atap\Sdk\Model\ProblemDetail;
use GuzzleHttp\Client;
use GuzzleHttp\Exception\GuzzleException;
use GuzzleHttp\HandlerStack;
use GuzzleHttp\Psr7\Response;
use Psr\Http\Message\ResponseInterface;

/**
 * Low-level HTTP client with DPoP proof injection and error handling.
 */
class HttpClient
{
    private Client $client;
    private string $baseUrl;

    public function __construct(
        string $baseUrl = 'http://localhost:8080',
        float $timeout = 30.0,
        ?HandlerStack $handler = null,
    ) {
        $this->baseUrl = rtrim($baseUrl, '/');

        $config = [
            'base_uri' => $this->baseUrl,
            'timeout' => $timeout,
            'http_errors' => false,
        ];

        if ($handler !== null) {
            $config['handler'] = $handler;
        }

        $this->client = new Client($config);
    }

    /**
     * Get the underlying Guzzle client (for testing).
     */
    public function getClient(): Client
    {
        return $this->client;
    }

    /**
     * Make an HTTP request and return parsed JSON response.
     *
     * @param array<string, mixed>|null $jsonBody
     * @param array<string, string>|null $headers
     * @param array<string, string>|null $params
     * @return array<string, mixed>
     */
    public function request(
        string $method,
        string $path,
        ?array $jsonBody = null,
        ?array $headers = null,
        ?array $params = null,
    ): array {
        $options = [];
        $h = $headers ?? [];

        if ($jsonBody !== null) {
            $options['json'] = $jsonBody;
            $h['Content-Type'] = $h['Content-Type'] ?? 'application/json';
        }

        if (!empty($h)) {
            $options['headers'] = $h;
        }

        if ($params !== null) {
            $options['query'] = $params;
        }

        try {
            $response = $this->client->request($method, $path, $options);
        } catch (GuzzleException $e) {
            throw new ATAPException('HTTP request failed: ' . $e->getMessage(), 0, $e);
        }

        return $this->handleResponse($response);
    }

    /**
     * Make a DPoP-authenticated HTTP request.
     *
     * @param array<string, mixed>|null $jsonBody
     * @param array<string, string>|null $params
     * @return array<string, mixed>
     */
    public function authenticatedRequest(
        string $method,
        string $path,
        string $signingKey,
        string $accessToken,
        string $platformDomain,
        ?array $jsonBody = null,
        ?string $rawBody = null,
        ?string $contentType = null,
        ?array $params = null,
    ): array {
        $url = "https://{$platformDomain}{$path}";
        $dpopProof = DPoPProof::create($signingKey, $method, $url, $accessToken);

        $headers = [
            'Authorization' => "DPoP {$accessToken}",
            'DPoP' => $dpopProof,
        ];

        if ($contentType !== null) {
            $headers['Content-Type'] = $contentType;
        }

        $options = [
            'headers' => $headers,
        ];

        if ($rawBody !== null) {
            $options['body'] = $rawBody;
        } elseif ($jsonBody !== null) {
            $options['json'] = $jsonBody;
            // Remove Content-Type if using json option (Guzzle sets it)
            if (!isset($headers['Content-Type'])) {
                unset($options['headers']['Content-Type']);
            }
        }

        if ($params !== null) {
            $options['query'] = $params;
        }

        try {
            $response = $this->client->request($method, $path, $options);
        } catch (GuzzleException $e) {
            throw new ATAPException('HTTP request failed: ' . $e->getMessage(), 0, $e);
        }

        return $this->handleResponse($response);
    }

    /**
     * POST form-encoded data (for OAuth token endpoint).
     *
     * @param array<string, string> $formData
     * @return array<string, mixed>
     */
    public function postForm(
        string $path,
        array $formData,
        ?string $dpopProof = null,
    ): array {
        $headers = [
            'Content-Type' => 'application/x-www-form-urlencoded',
        ];

        if ($dpopProof !== null) {
            $headers['DPoP'] = $dpopProof;
        }

        $options = [
            'headers' => $headers,
            'body' => http_build_query($formData),
        ];

        try {
            $response = $this->client->request('POST', $path, $options);
        } catch (GuzzleException $e) {
            throw new ATAPException('HTTP request failed: ' . $e->getMessage(), 0, $e);
        }

        return $this->handleResponse($response);
    }

    /**
     * GET request expecting a 302 redirect, returns the Location URL.
     *
     * @param array<string, string>|null $params
     */
    public function getRedirect(
        string $path,
        ?array $params = null,
        ?string $dpopProof = null,
    ): string {
        $options = [
            'allow_redirects' => false,
        ];

        $headers = [];
        if ($dpopProof !== null) {
            $headers['DPoP'] = $dpopProof;
        }
        if (!empty($headers)) {
            $options['headers'] = $headers;
        }

        if ($params !== null) {
            $options['query'] = $params;
        }

        try {
            $response = $this->client->request('GET', $path, $options);
        } catch (GuzzleException $e) {
            throw new ATAPException('HTTP request failed: ' . $e->getMessage(), 0, $e);
        }

        if ($response->getStatusCode() !== 302) {
            $this->handleResponse($response);
            throw new ATAPException(
                sprintf('Expected 302 redirect, got %d', $response->getStatusCode()),
                $response->getStatusCode(),
            );
        }

        $location = $response->getHeaderLine('Location');
        if ($location === '') {
            throw new ATAPException('302 redirect with no Location header');
        }

        return $location;
    }

    /**
     * Parse response, raising typed errors for non-2xx status codes.
     *
     * @return array<string, mixed>
     */
    public function handleResponse(ResponseInterface $response): array
    {
        $status = $response->getStatusCode();

        if ($status === 204) {
            return [];
        }

        $body = (string) $response->getBody();

        try {
            $data = json_decode($body, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException) {
            if ($status >= 200 && $status < 300) {
                return [];
            }
            throw new ATAPException("HTTP {$status}: {$body}", $status);
        }

        if (!is_array($data)) {
            $data = [];
        }

        if ($status >= 200 && $status < 300) {
            return $data;
        }

        // Parse RFC 7807 Problem Detail
        $problem = null;
        if (isset($data['type'], $data['status'])) {
            $problem = ProblemDetail::fromArray($data);
        }

        if ($status === 401 || $status === 403) {
            $msg = $problem?->detail ?? ($data['detail'] ?? 'Authentication failed');
            throw new ATAPAuthException((string) $msg, $status, $problem);
        }

        if ($status === 404) {
            $msg = $problem?->detail ?? 'Not found';
            throw new ATAPNotFoundException((string) $msg, $problem);
        }

        if ($status === 409) {
            $msg = $problem?->detail ?? 'Conflict';
            throw new ATAPConflictException((string) $msg, $problem);
        }

        if ($status === 429) {
            $msg = $problem?->detail ?? 'Rate limit exceeded';
            throw new ATAPRateLimitException((string) $msg, $problem);
        }

        if ($problem !== null) {
            throw new ATAPProblemException($problem);
        }

        throw new ATAPException(
            sprintf('HTTP %d: %s', $status, $data['detail'] ?? $data['message'] ?? json_encode($data)),
            $status,
        );
    }
}
