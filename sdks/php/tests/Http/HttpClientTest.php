<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\Http;

use Atap\Sdk\Exception\ATAPAuthException;
use Atap\Sdk\Exception\ATAPConflictException;
use Atap\Sdk\Exception\ATAPException;
use Atap\Sdk\Exception\ATAPNotFoundException;
use Atap\Sdk\Exception\ATAPProblemException;
use Atap\Sdk\Exception\ATAPRateLimitException;
use Atap\Sdk\Http\HttpClient;
use Atap\Sdk\Tests\TestCase;
use GuzzleHttp\Psr7\Response;

class HttpClientTest extends TestCase
{
    private function createClient(array $responses): HttpClient
    {
        return new HttpClient(
            'http://localhost:8080',
            30.0,
            $this->createHandlerStack($responses),
        );
    }

    public function testSuccessfulJsonRequest(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(200, ['id' => 'test123', 'type' => 'agent']),
        ]);

        $data = $client->request('GET', '/v1/entities/test123');
        $this->assertSame('test123', $data['id']);
        $this->assertSame('agent', $data['type']);
    }

    public function testPostJsonBody(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(201, ['id' => 'new_entity', 'type' => 'agent']),
        ]);

        $data = $client->request('POST', '/v1/entities', ['type' => 'agent', 'name' => 'Test']);
        $this->assertSame('new_entity', $data['id']);
    }

    public function test204ReturnsEmptyArray(): void
    {
        $client = $this->createClient([
            $this->noContentResponse(),
        ]);

        $data = $client->request('DELETE', '/v1/entities/test123');
        $this->assertSame([], $data);
    }

    public function test401ThrowsAuthException(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(401, [
                'type' => 'about:blank',
                'title' => 'Unauthorized',
                'status' => 401,
                'detail' => 'Invalid token',
            ]),
        ]);

        $this->expectException(ATAPAuthException::class);
        $this->expectExceptionMessage('Invalid token');
        $client->request('GET', '/v1/entities/test123');
    }

    public function test403ThrowsAuthException(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(403, [
                'type' => 'about:blank',
                'title' => 'Forbidden',
                'status' => 403,
                'detail' => 'Insufficient scope',
            ]),
        ]);

        $this->expectException(ATAPAuthException::class);
        $client->request('GET', '/v1/entities/test123');
    }

    public function test404ThrowsNotFoundException(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(404, [
                'type' => 'about:blank',
                'title' => 'Not Found',
                'status' => 404,
                'detail' => 'Entity not found',
            ]),
        ]);

        $this->expectException(ATAPNotFoundException::class);
        $this->expectExceptionMessage('Entity not found');
        $client->request('GET', '/v1/entities/missing');
    }

    public function test409ThrowsConflictException(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(409, [
                'type' => 'about:blank',
                'title' => 'Conflict',
                'status' => 409,
                'detail' => 'Entity already exists',
            ]),
        ]);

        $this->expectException(ATAPConflictException::class);
        $client->request('POST', '/v1/entities');
    }

    public function test429ThrowsRateLimitException(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(429, [
                'type' => 'about:blank',
                'title' => 'Too Many Requests',
                'status' => 429,
                'detail' => 'Rate limit exceeded',
            ]),
        ]);

        $this->expectException(ATAPRateLimitException::class);
        $client->request('GET', '/v1/entities');
    }

    public function testProblemDetailExceptionForOtherErrors(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(422, [
                'type' => 'about:blank',
                'title' => 'Unprocessable Entity',
                'status' => 422,
                'detail' => 'Validation failed',
            ]),
        ]);

        $this->expectException(ATAPProblemException::class);
        $client->request('POST', '/v1/entities');
    }

    public function testNonJsonErrorResponse(): void
    {
        $client = $this->createClient([
            new Response(500, [], 'Internal Server Error'),
        ]);

        $this->expectException(ATAPException::class);
        $client->request('GET', '/v1/health');
    }

    public function testNonJsonSuccessReturnsEmpty(): void
    {
        $client = $this->createClient([
            new Response(200, [], 'OK'),
        ]);

        $data = $client->request('GET', '/v1/health');
        $this->assertSame([], $data);
    }

    public function testPostForm(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(200, [
                'access_token' => 'token_123',
                'token_type' => 'DPoP',
            ]),
        ]);

        $data = $client->postForm('/v1/oauth/token', [
            'grant_type' => 'client_credentials',
            'client_id' => 'test',
        ], 'dpop_proof_here');

        $this->assertSame('token_123', $data['access_token']);
    }

    public function testGetRedirect(): void
    {
        $client = $this->createClient([
            $this->redirectResponse('https://example.com/callback?code=abc'),
        ]);

        $location = $client->getRedirect('/v1/oauth/authorize', ['response_type' => 'code']);
        $this->assertSame('https://example.com/callback?code=abc', $location);
    }

    public function testGetRedirectNon302Throws(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(200, ['status' => 'ok']),
        ]);

        $this->expectException(ATAPException::class);
        $this->expectExceptionMessage('Expected 302 redirect');
        $client->getRedirect('/v1/oauth/authorize');
    }

    public function testGetRedirectNoLocationThrows(): void
    {
        $client = $this->createClient([
            new Response(302),
        ]);

        $this->expectException(ATAPException::class);
        $this->expectExceptionMessage('no Location header');
        $client->getRedirect('/v1/oauth/authorize');
    }

    public function testRequestWithQueryParams(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(200, ['items' => []]),
        ]);

        $data = $client->request('GET', '/v1/revocations', params: ['entity' => 'did:web:test']);
        $this->assertSame([], $data['items']);
    }

    public function testErrorWithoutProblemDetail(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(500, ['message' => 'Server error']),
        ]);

        $this->expectException(ATAPException::class);
        $client->request('GET', '/v1/health');
    }

    public function test401WithoutProblemDetailFallback(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(401, ['detail' => 'Bad token']),
        ]);

        $this->expectException(ATAPAuthException::class);
        $this->expectExceptionMessage('Bad token');
        $client->request('GET', '/v1/entities/test');
    }

    public function test404WithoutProblemDetailFallback(): void
    {
        $client = $this->createClient([
            $this->jsonResponse(404, ['error' => 'not found']),
        ]);

        $this->expectException(ATAPNotFoundException::class);
        $this->expectExceptionMessage('Not found');
        $client->request('GET', '/v1/entities/missing');
    }
}
