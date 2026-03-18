package atap

import (
	"context"
	"crypto/ed25519"
	"time"
)

// Client is the high-level client for the ATAP platform.
type Client struct {
	Entities    *EntityAPI
	Approvals   *ApprovalAPI
	Revocations *RevocationAPI
	DIDComm     *DIDCommAPI
	Credentials *CredentialAPI
	Discovery   *DiscoveryAPI

	http           *HTTPClient
	did            string
	signingKey     ed25519.PrivateKey
	platformDomain string
	tokenManager   *TokenManager
}

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	baseURL        string
	did            string
	privateKey     string
	signingKey     ed25519.PrivateKey
	clientSecret   string
	scopes         []string
	platformDomain string
	timeout        time.Duration
}

// WithBaseURL sets the HTTP base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) { c.baseURL = url }
}

// WithDID sets the entity DID (used as client_id for OAuth).
func WithDID(did string) Option {
	return func(c *clientConfig) { c.did = did }
}

// WithPrivateKey sets the base64-encoded Ed25519 private key (seed or full key).
func WithPrivateKey(key string) Option {
	return func(c *clientConfig) { c.privateKey = key }
}

// WithSigningKey sets a pre-loaded Ed25519 private key.
func WithSigningKey(key ed25519.PrivateKey) Option {
	return func(c *clientConfig) { c.signingKey = key }
}

// WithClientSecret sets the client secret for agent/machine client_credentials grant.
func WithClientSecret(secret string) Option {
	return func(c *clientConfig) { c.clientSecret = secret }
}

// WithScopes sets the OAuth scopes.
func WithScopes(scopes []string) Option {
	return func(c *clientConfig) { c.scopes = scopes }
}

// WithPlatformDomain sets the domain for DPoP htu construction.
func WithPlatformDomain(domain string) Option {
	return func(c *clientConfig) { c.platformDomain = domain }
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) { c.timeout = timeout }
}

// NewClient creates a new ATAP Client.
func NewClient(opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		baseURL: "http://localhost:8080",
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var signingKey ed25519.PrivateKey
	if cfg.signingKey != nil {
		signingKey = cfg.signingKey
	} else if cfg.privateKey != "" {
		var err error
		signingKey, err = LoadSigningKey(cfg.privateKey)
		if err != nil {
			return nil, err
		}
	}

	domain := cfg.platformDomain
	if domain == "" && cfg.did != "" {
		d, err := DomainFromDID(cfg.did)
		if err == nil {
			domain = d
		}
	}
	if domain == "" {
		domain = "localhost"
	}

	httpClient := NewHTTPClient(cfg.baseURL, cfg.timeout)

	c := &Client{
		http:           httpClient,
		did:            cfg.did,
		signingKey:     signingKey,
		platformDomain: domain,
	}

	if signingKey != nil && cfg.did != "" {
		c.tokenManager = NewTokenManager(TokenManagerConfig{
			HTTPClient:     httpClient,
			SigningKey:      signingKey,
			DID:            cfg.did,
			ClientSecret:   cfg.clientSecret,
			Scopes:         cfg.scopes,
			PlatformDomain: domain,
		})
	}

	c.Entities = &EntityAPI{client: c}
	c.Approvals = &ApprovalAPI{client: c}
	c.Revocations = &RevocationAPI{client: c}
	c.DIDComm = &DIDCommAPI{client: c}
	c.Credentials = &CredentialAPI{client: c}
	c.Discovery = &DiscoveryAPI{client: c}

	return c, nil
}

// TokenManager returns the token manager for manual token operations.
func (c *Client) TokenManager() (*TokenManager, error) {
	if c.tokenManager == nil {
		return nil, NewATAPError("token manager not initialized; provide DID and private key", 0)
	}
	return c.tokenManager, nil
}

func (c *Client) authedRequest(ctx context.Context, method, path string, opts *RequestOptions) (map[string]interface{}, error) {
	if c.tokenManager == nil || c.signingKey == nil {
		return nil, NewATAPError("authentication not configured; provide DID, private key, and optionally client secret", 0)
	}

	accessToken, err := c.tokenManager.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &RequestOptions{}
	}
	if opts.Headers == nil {
		opts.Headers = make(map[string]string)
	}

	return c.http.AuthenticatedRequest(ctx, method, path, c.signingKey, accessToken, c.platformDomain, opts)
}

// Close closes the HTTP client.
func (c *Client) Close() {
	// net/http.Client doesn't need explicit closing, but this provides API parity.
}
