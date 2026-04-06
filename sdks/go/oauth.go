package atap

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenManager manages OAuth 2.1 tokens with DPoP binding and auto-refresh.
type TokenManager struct {
	http           *HTTPClient
	signingKey     ed25519.PrivateKey
	did            string
	clientSecret   string
	scopes         []string
	platformDomain string

	mu              sync.Mutex
	token           *OAuthToken
	tokenObtainedAt time.Time
}

// TokenManagerConfig holds configuration for creating a TokenManager.
type TokenManagerConfig struct {
	HTTPClient     *HTTPClient
	SigningKey     ed25519.PrivateKey
	DID            string
	ClientSecret   string
	Scopes         []string
	PlatformDomain string
}

// NewTokenManager creates a new TokenManager.
func NewTokenManager(cfg TokenManagerConfig) *TokenManager {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"atap:inbox", "atap:send", "atap:revoke", "atap:manage"}
	}
	domain := cfg.PlatformDomain
	if domain == "" {
		d, err := DomainFromDID(cfg.DID)
		if err == nil {
			domain = d
		}
	}
	return &TokenManager{
		http:           cfg.HTTPClient,
		signingKey:     cfg.SigningKey,
		did:            cfg.DID,
		clientSecret:   cfg.ClientSecret,
		scopes:         scopes,
		platformDomain: domain,
	}
}

func (tm *TokenManager) tokenURL() string {
	return fmt.Sprintf("https://%s/v1/oauth/token", tm.platformDomain)
}

// GetAccessToken returns a valid access token, refreshing if needed.
func (tm *TokenManager) GetAccessToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.token != nil && !tm.isExpired() {
		return tm.token.AccessToken, nil
	}
	if tm.token != nil && tm.token.RefreshToken != "" {
		tok, err := tm.refresh(ctx)
		if err != nil {
			return "", err
		}
		return tok.AccessToken, nil
	}
	tok, err := tm.obtain(ctx)
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

func (tm *TokenManager) isExpired() bool {
	if tm.token == nil {
		return true
	}
	elapsed := time.Since(tm.tokenObtainedAt).Seconds()
	return elapsed >= float64(tm.token.ExpiresIn-60)
}

func (tm *TokenManager) obtain(ctx context.Context) (*OAuthToken, error) {
	if tm.clientSecret == "" {
		return nil, fmt.Errorf("client_secret is required for client_credentials grant; for human/org entities, use ObtainAuthorizationCode() instead")
	}

	dpopProof := MakeDPoPProof(tm.signingKey, "POST", tm.tokenURL(), "")

	formData := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     tm.did,
		"client_secret": tm.clientSecret,
		"scope":         strings.Join(tm.scopes, " "),
	}

	data, err := tm.http.PostForm(ctx, "/v1/oauth/token", formData, dpopProof)
	if err != nil {
		return nil, fmt.Errorf("obtain token: %w", err)
	}

	tm.token = parseOAuthToken(data)
	tm.tokenObtainedAt = time.Now()
	return tm.token, nil
}

func (tm *TokenManager) refresh(ctx context.Context) (*OAuthToken, error) {
	if tm.token == nil || tm.token.RefreshToken == "" {
		return tm.obtain(ctx)
	}

	dpopProof := MakeDPoPProof(tm.signingKey, "POST", tm.tokenURL(), "")

	formData := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": tm.token.RefreshToken,
	}

	data, err := tm.http.PostForm(ctx, "/v1/oauth/token", formData, dpopProof)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	oldRefresh := tm.token.RefreshToken
	tm.token = parseOAuthToken(data)
	if tm.token.RefreshToken == "" {
		tm.token.RefreshToken = oldRefresh
	}
	tm.tokenObtainedAt = time.Now()
	return tm.token, nil
}

// ObtainAuthorizationCode performs the authorization_code + PKCE flow for human/org entities.
func (tm *TokenManager) ObtainAuthorizationCode(ctx context.Context, redirectURI string) (*OAuthToken, error) {
	if redirectURI == "" {
		redirectURI = "atap://callback"
	}

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}

	authorizeURL := fmt.Sprintf("https://%s/v1/oauth/authorize", tm.platformDomain)
	dpopProof := MakeDPoPProof(tm.signingKey, "GET", authorizeURL, "")

	params := map[string]string{
		"response_type":         "code",
		"client_id":             tm.did,
		"redirect_uri":          redirectURI,
		"scope":                 strings.Join(tm.scopes, " "),
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}

	redirectLocation, err := tm.http.GetRedirect(ctx, "/v1/oauth/authorize", params, dpopProof)
	if err != nil {
		return nil, fmt.Errorf("authorization redirect: %w", err)
	}

	parsed, err := url.Parse(redirectLocation)
	if err != nil {
		return nil, fmt.Errorf("parse redirect URL: %w", err)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		return nil, fmt.Errorf("no authorization code in redirect: %s", redirectLocation)
	}

	dpopProof2 := MakeDPoPProof(tm.signingKey, "POST", tm.tokenURL(), "")
	formData := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := tm.http.PostForm(ctx, "/v1/oauth/token", formData, dpopProof2)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}

	tm.token = parseOAuthToken(data)
	tm.tokenObtainedAt = time.Now()
	return tm.token, nil
}

// Invalidate clears the cached token, forcing re-authentication on next request.
func (tm *TokenManager) Invalidate() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.token = nil
	tm.tokenObtainedAt = time.Time{}
}

func parseOAuthToken(data map[string]interface{}) *OAuthToken {
	tok := &OAuthToken{
		AccessToken:  getString(data, "access_token"),
		TokenType:    getString(data, "token_type"),
		ExpiresIn:    getInt(data, "expires_in", 3600),
		Scope:        getString(data, "scope"),
		RefreshToken: getString(data, "refresh_token"),
	}
	if tok.TokenType == "" {
		tok.TokenType = "DPoP"
	}
	return tok
}
