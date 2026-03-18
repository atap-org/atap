package dev.atap.sdk.oauth;

import dev.atap.sdk.DomainUtils;
import dev.atap.sdk.crypto.DPoPProof;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.crypto.PKCE;
import dev.atap.sdk.exception.ATAPException;
import dev.atap.sdk.http.ATAPHttpClient;

import java.net.URI;
import java.util.*;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Thread-safe OAuth 2.1 token management with DPoP binding and auto-refresh.
 * <p>
 * Supports client_credentials (agent/machine) and authorization_code+PKCE (human/org) grant types.
 */
public class TokenManager {

    private final ATAPHttpClient httpClient;
    private final Ed25519KeyPair keyPair;
    private final String did;
    private final String clientSecret;
    private final List<String> scopes;
    private final String platformDomain;
    private final ReentrantLock lock = new ReentrantLock();

    private OAuthToken token;
    private long tokenObtainedAt;

    private static final List<String> DEFAULT_SCOPES = Arrays.asList(
            "atap:inbox", "atap:send", "atap:revoke", "atap:manage"
    );

    public TokenManager(ATAPHttpClient httpClient, Ed25519KeyPair keyPair, String did,
                         String clientSecret, List<String> scopes, String platformDomain) {
        this.httpClient = httpClient;
        this.keyPair = keyPair;
        this.did = did;
        this.clientSecret = clientSecret;
        this.scopes = scopes != null ? scopes : DEFAULT_SCOPES;
        this.platformDomain = platformDomain != null ? platformDomain : DomainUtils.domainFromDid(did);
    }

    private String tokenUrl() {
        return "https://" + platformDomain + "/v1/oauth/token";
    }

    /**
     * Get a valid access token, refreshing if needed. Thread-safe.
     */
    public String getAccessToken() {
        lock.lock();
        try {
            if (token != null && !isExpired()) {
                return token.getAccessToken();
            }
            if (token != null && token.getRefreshToken() != null) {
                return refresh().getAccessToken();
            }
            return obtain().getAccessToken();
        } finally {
            lock.unlock();
        }
    }

    /**
     * Clear cached token, forcing re-authentication on next request.
     */
    public void invalidate() {
        lock.lock();
        try {
            token = null;
            tokenObtainedAt = 0;
        } finally {
            lock.unlock();
        }
    }

    private boolean isExpired() {
        if (token == null) return true;
        long elapsed = (System.currentTimeMillis() / 1000) - tokenObtainedAt;
        return elapsed >= (token.getExpiresIn() - 60);
    }

    private OAuthToken obtain() {
        if (clientSecret == null || clientSecret.isEmpty()) {
            throw new IllegalStateException(
                    "client_secret is required for client_credentials grant. " +
                    "For human/org entities, use obtainAuthorizationCode() instead."
            );
        }

        String dpopProof = DPoPProof.create(keyPair, "POST", tokenUrl());

        Map<String, String> formData = new LinkedHashMap<>();
        formData.put("grant_type", "client_credentials");
        formData.put("client_id", did);
        formData.put("client_secret", clientSecret);
        formData.put("scope", String.join(" ", scopes));

        Map<String, Object> data = httpClient.postForm("/v1/oauth/token", formData, dpopProof);
        token = parseToken(data);
        tokenObtainedAt = System.currentTimeMillis() / 1000;
        return token;
    }

    private OAuthToken refresh() {
        if (token == null || token.getRefreshToken() == null) {
            return obtain();
        }

        String dpopProof = DPoPProof.create(keyPair, "POST", tokenUrl());

        Map<String, String> formData = new LinkedHashMap<>();
        formData.put("grant_type", "refresh_token");
        formData.put("refresh_token", token.getRefreshToken());

        Map<String, Object> data = httpClient.postForm("/v1/oauth/token", formData, dpopProof);
        String prevRefresh = token.getRefreshToken();
        token = parseToken(data);
        if (token.getRefreshToken() == null) {
            token.setRefreshToken(prevRefresh);
        }
        tokenObtainedAt = System.currentTimeMillis() / 1000;
        return token;
    }

    /**
     * Obtain a token via authorization_code + PKCE flow (for human/org).
     */
    public OAuthToken obtainAuthorizationCode(String redirectUri) {
        PKCE.PKCEPair pkce = PKCE.generate();
        String authorizeUrl = "https://" + platformDomain + "/v1/oauth/authorize";

        String dpopProof = DPoPProof.create(keyPair, "GET", authorizeUrl);

        Map<String, String> params = new LinkedHashMap<>();
        params.put("response_type", "code");
        params.put("client_id", did);
        params.put("redirect_uri", redirectUri);
        params.put("scope", String.join(" ", scopes));
        params.put("code_challenge", pkce.getChallenge());
        params.put("code_challenge_method", "S256");

        String redirectLocation = httpClient.getRedirect("/v1/oauth/authorize", params, dpopProof);

        // Extract code from redirect URL
        URI redirected = URI.create(redirectLocation);
        String query = redirected.getQuery();
        String code = null;
        if (query != null) {
            for (String param : query.split("&")) {
                String[] kv = param.split("=", 2);
                if ("code".equals(kv[0]) && kv.length > 1) {
                    code = kv[1];
                    break;
                }
            }
        }
        if (code == null || code.isEmpty()) {
            throw new ATAPException("No authorization code in redirect: " + redirectLocation);
        }

        String dpopProof2 = DPoPProof.create(keyPair, "POST", tokenUrl());
        Map<String, String> formData = new LinkedHashMap<>();
        formData.put("grant_type", "authorization_code");
        formData.put("code", code);
        formData.put("redirect_uri", redirectUri);
        formData.put("code_verifier", pkce.getVerifier());

        lock.lock();
        try {
            Map<String, Object> data = httpClient.postForm("/v1/oauth/token", formData, dpopProof2);
            token = parseToken(data);
            tokenObtainedAt = System.currentTimeMillis() / 1000;
            return token;
        } finally {
            lock.unlock();
        }
    }

    public OAuthToken obtainAuthorizationCode() {
        return obtainAuthorizationCode("atap://callback");
    }

    private OAuthToken parseToken(Map<String, Object> data) {
        return new OAuthToken(
                (String) data.get("access_token"),
                data.containsKey("token_type") ? (String) data.get("token_type") : "DPoP",
                data.containsKey("expires_in") ? ((Number) data.get("expires_in")).intValue() : 3600,
                data.containsKey("scope") ? (String) data.get("scope") : "",
                (String) data.get("refresh_token")
        );
    }
}
