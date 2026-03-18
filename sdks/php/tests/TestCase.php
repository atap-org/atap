<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests;

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Crypto\Ed25519;
use GuzzleHttp\Handler\MockHandler;
use GuzzleHttp\HandlerStack;
use GuzzleHttp\Psr7\Response;
use PHPUnit\Framework\TestCase as PHPUnitTestCase;

/**
 * Base test case with mock helpers for the ATAP SDK.
 */
abstract class TestCase extends PHPUnitTestCase
{
    /**
     * Create a MockHandler with the given responses.
     *
     * @param Response[] $responses
     */
    protected function createMockHandler(array $responses): MockHandler
    {
        return new MockHandler($responses);
    }

    /**
     * Create a HandlerStack with a MockHandler.
     *
     * @param Response[] $responses
     */
    protected function createHandlerStack(array $responses): HandlerStack
    {
        return HandlerStack::create($this->createMockHandler($responses));
    }

    /**
     * Create a JSON response.
     *
     * @param array<string, mixed> $body
     */
    protected function jsonResponse(int $status, array $body): Response
    {
        return new Response(
            $status,
            ['Content-Type' => 'application/json'],
            json_encode($body),
        );
    }

    /**
     * Create a 204 No Content response.
     */
    protected function noContentResponse(): Response
    {
        return new Response(204);
    }

    /**
     * Create a 302 redirect response.
     */
    protected function redirectResponse(string $location): Response
    {
        return new Response(302, ['Location' => $location]);
    }

    /**
     * Create a test Ed25519 keypair.
     *
     * @return array{secretKey: string, publicKey: string}
     */
    protected function generateTestKeypair(): array
    {
        return Ed25519::generateKeypair();
    }

    /**
     * Create an ATAPClient with mock responses for authenticated requests.
     * Includes a token response as the first mock, followed by the actual responses.
     *
     * @param Response[] $responses API responses (token response is prepended automatically)
     */
    protected function createAuthedClient(array $responses): ATAPClient
    {
        $keys = $this->generateTestKeypair();

        // Prepend token response
        $tokenResponse = $this->jsonResponse(200, [
            'access_token' => 'test_access_token',
            'token_type' => 'DPoP',
            'expires_in' => 3600,
            'scope' => 'atap:inbox atap:send atap:revoke atap:manage',
        ]);

        array_unshift($responses, $tokenResponse);

        return new ATAPClient([
            'base_url' => 'http://localhost:8080',
            'did' => 'did:web:localhost%3A8080:agent:test123',
            'signing_key' => $keys['secretKey'],
            'client_secret' => 'atap_test_secret',
            'handler' => $this->createHandlerStack($responses),
        ]);
    }

    /**
     * Create an ATAPClient with mock responses (no auth).
     *
     * @param Response[] $responses
     */
    protected function createUnauthClient(array $responses): ATAPClient
    {
        return new ATAPClient([
            'base_url' => 'http://localhost:8080',
            'handler' => $this->createHandlerStack($responses),
        ]);
    }
}
