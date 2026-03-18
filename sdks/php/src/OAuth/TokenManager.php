<?php

declare(strict_types=1);

namespace Atap\Sdk\OAuth;

use Atap\Sdk\Crypto\DPoPProof;
use Atap\Sdk\Crypto\PKCE;
use Atap\Sdk\DomainUtils;
use Atap\Sdk\Http\HttpClient;
use Atap\Sdk\Model\OAuthToken;

/**
 * Manages OAuth 2.1 tokens with DPoP binding and auto-refresh.
 *
 * Supports client_credentials (agent/machine) and
 * authorization_code+PKCE (human/org) grant types.
 */
class TokenManager
{
    private ?OAuthToken $token = null;
    private float $tokenObtainedAt = 0;

    /** @var string[] */
    private array $scopes;
    private string $platformDomain;

    /**
     * @param string[] $scopes
     */
    public function __construct(
        private readonly HttpClient $httpClient,
        private readonly string $signingKey,
        private readonly string $did,
        private readonly ?string $clientSecret = null,
        ?array $scopes = null,
        ?string $platformDomain = null,
    ) {
        $this->scopes = $scopes ?? ['atap:inbox', 'atap:send', 'atap:revoke', 'atap:manage'];
        $this->platformDomain = $platformDomain ?? DomainUtils::domainFromDid($did);
    }

    /**
     * Get a valid access token, refreshing if needed.
     */
    public function getAccessToken(): string
    {
        if ($this->token !== null && !$this->isExpired()) {
            return $this->token->accessToken;
        }

        if ($this->token !== null && $this->token->refreshToken !== null) {
            return $this->refresh()->accessToken;
        }

        return $this->obtain()->accessToken;
    }

    /**
     * Clear cached token, forcing re-authentication on next request.
     */
    public function invalidate(): void
    {
        $this->token = null;
        $this->tokenObtainedAt = 0;
    }

    /**
     * Check if the current token is expired or near expiry.
     */
    private function isExpired(): bool
    {
        if ($this->token === null) {
            return true;
        }
        $elapsed = time() - $this->tokenObtainedAt;
        // Refresh 60 seconds before expiry
        return $elapsed >= ($this->token->expiresIn - 60);
    }

    /**
     * Obtain a new token via client_credentials grant.
     */
    private function obtain(): OAuthToken
    {
        if ($this->clientSecret === null) {
            throw new \RuntimeException(
                'client_secret is required for client_credentials grant. '
                . 'For human/org entities, use obtainAuthorizationCode() instead.'
            );
        }

        $tokenUrl = "https://{$this->platformDomain}/v1/oauth/token";
        $dpopProof = DPoPProof::create($this->signingKey, 'POST', $tokenUrl);

        $formData = [
            'grant_type' => 'client_credentials',
            'client_id' => $this->did,
            'client_secret' => $this->clientSecret,
            'scope' => implode(' ', $this->scopes),
        ];

        $data = $this->httpClient->postForm('/v1/oauth/token', $formData, $dpopProof);
        $this->token = OAuthToken::fromArray($data);
        $this->tokenObtainedAt = time();

        return $this->token;
    }

    /**
     * Refresh an expired token using the refresh token.
     */
    private function refresh(): OAuthToken
    {
        if ($this->token === null || $this->token->refreshToken === null) {
            return $this->obtain();
        }

        $tokenUrl = "https://{$this->platformDomain}/v1/oauth/token";
        $dpopProof = DPoPProof::create($this->signingKey, 'POST', $tokenUrl);

        $formData = [
            'grant_type' => 'refresh_token',
            'refresh_token' => $this->token->refreshToken,
        ];

        $data = $this->httpClient->postForm('/v1/oauth/token', $formData, $dpopProof);

        $refreshToken = $data['refresh_token'] ?? $this->token->refreshToken;
        $data['refresh_token'] = $refreshToken;

        $this->token = OAuthToken::fromArray($data);
        $this->tokenObtainedAt = time();

        return $this->token;
    }

    /**
     * Obtain a token via authorization_code + PKCE flow (for human/org).
     */
    public function obtainAuthorizationCode(string $redirectUri = 'atap://callback'): OAuthToken
    {
        $pkce = PKCE::generate();
        $authorizeUrl = "https://{$this->platformDomain}/v1/oauth/authorize";
        $dpopProof = DPoPProof::create($this->signingKey, 'GET', $authorizeUrl);

        $params = [
            'response_type' => 'code',
            'client_id' => $this->did,
            'redirect_uri' => $redirectUri,
            'scope' => implode(' ', $this->scopes),
            'code_challenge' => $pkce['challenge'],
            'code_challenge_method' => 'S256',
        ];

        $redirectLocation = $this->httpClient->getRedirect(
            '/v1/oauth/authorize',
            $params,
            $dpopProof,
        );

        // Extract code from redirect URL
        $parsed = parse_url($redirectLocation);
        parse_str($parsed['query'] ?? '', $qs);
        $code = $qs['code'] ?? null;

        if ($code === null) {
            throw new \RuntimeException("No authorization code in redirect: {$redirectLocation}");
        }

        // Exchange code for token
        $tokenUrl = "https://{$this->platformDomain}/v1/oauth/token";
        $dpopProof2 = DPoPProof::create($this->signingKey, 'POST', $tokenUrl);
        $formData = [
            'grant_type' => 'authorization_code',
            'code' => $code,
            'redirect_uri' => $redirectUri,
            'code_verifier' => $pkce['verifier'],
        ];

        $data = $this->httpClient->postForm('/v1/oauth/token', $formData, $dpopProof2);
        $this->token = OAuthToken::fromArray($data);
        $this->tokenObtainedAt = time();

        return $this->token;
    }
}
