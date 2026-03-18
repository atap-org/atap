<?php

declare(strict_types=1);

namespace Atap\Sdk\Tests\OAuth;

use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Http\HttpClient;
use Atap\Sdk\OAuth\TokenManager;
use Atap\Sdk\Tests\TestCase;

class TokenManagerTest extends TestCase
{
    private function createTokenManager(array $responses, ?string $clientSecret = 'atap_secret'): TokenManager
    {
        $keys = Ed25519::generateKeypair();
        $handler = $this->createHandlerStack($responses);
        $httpClient = new HttpClient('http://localhost:8080', 30.0, $handler);

        return new TokenManager(
            httpClient: $httpClient,
            signingKey: $keys['secretKey'],
            did: 'did:web:localhost%3A8080:agent:test123',
            clientSecret: $clientSecret,
            platformDomain: 'localhost:8080',
        );
    }

    public function testGetAccessTokenObtainsNewToken(): void
    {
        $tm = $this->createTokenManager([
            $this->jsonResponse(200, [
                'access_token' => 'new_token_123',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
                'scope' => 'atap:inbox',
            ]),
        ]);

        $token = $tm->getAccessToken();
        $this->assertSame('new_token_123', $token);
    }

    public function testGetAccessTokenReturnsCachedToken(): void
    {
        $tm = $this->createTokenManager([
            $this->jsonResponse(200, [
                'access_token' => 'cached_token',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
            ]),
        ]);

        $token1 = $tm->getAccessToken();
        $token2 = $tm->getAccessToken();
        $this->assertSame($token1, $token2);
    }

    public function testInvalidateClearsToken(): void
    {
        $tm = $this->createTokenManager([
            $this->jsonResponse(200, [
                'access_token' => 'token_1',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
            ]),
            $this->jsonResponse(200, [
                'access_token' => 'token_2',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
            ]),
        ]);

        $token1 = $tm->getAccessToken();
        $this->assertSame('token_1', $token1);

        $tm->invalidate();

        $token2 = $tm->getAccessToken();
        $this->assertSame('token_2', $token2);
    }

    public function testThrowsWithoutClientSecret(): void
    {
        $tm = $this->createTokenManager([], null);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('client_secret is required');
        $tm->getAccessToken();
    }

    public function testRefreshTokenUsed(): void
    {
        $tm = $this->createTokenManager([
            $this->jsonResponse(200, [
                'access_token' => 'first_token',
                'token_type' => 'DPoP',
                'expires_in' => 1, // Expires almost immediately
                'refresh_token' => 'refresh_123',
            ]),
            $this->jsonResponse(200, [
                'access_token' => 'refreshed_token',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
            ]),
        ]);

        // Get first token (will appear expired because expires_in=1 and we subtract 60)
        $token = $tm->getAccessToken();
        // The token expires_in=1 which is < 60 buffer, so next call should try refresh
        // But since we just got it, the elapsed time is 0 which is >= (1 - 60) = -59 ... so it IS expired
        // Actually 0 >= -59 is true, so it tries refresh on second call
        $token2 = $tm->getAccessToken();
        $this->assertSame('refreshed_token', $token2);
    }

    public function testObtainAuthorizationCode(): void
    {
        $tm = $this->createTokenManager([
            $this->redirectResponse('atap://callback?code=auth_code_123'),
            $this->jsonResponse(200, [
                'access_token' => 'auth_code_token',
                'token_type' => 'DPoP',
                'expires_in' => 3600,
            ]),
        ]);

        $token = $tm->obtainAuthorizationCode();
        $this->assertSame('auth_code_token', $token->accessToken);
    }

    public function testObtainAuthorizationCodeNoCodeInRedirect(): void
    {
        $tm = $this->createTokenManager([
            $this->redirectResponse('atap://callback?error=access_denied'),
        ]);

        $this->expectException(\RuntimeException::class);
        $this->expectExceptionMessage('No authorization code');
        $tm->obtainAuthorizationCode();
    }
}
