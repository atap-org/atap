package dev.atap.sdk;

import dev.atap.sdk.api.*;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.http.ATAPHttpClient;
import dev.atap.sdk.oauth.TokenManager;

import java.time.Duration;
import java.util.List;
import java.util.Map;

/**
 * High-level client for the ATAP platform.
 *
 * <p>Usage for agent/machine (client_credentials):
 * <pre>{@code
 * ATAPClient client = ATAPClient.builder()
 *     .baseUrl("http://localhost:8080")
 *     .did("did:web:localhost%3A8080:agent:abc")
 *     .privateKey("<base64 Ed25519 seed>")
 *     .clientSecret("atap_...")
 *     .build();
 * }</pre>
 */
public class ATAPClient implements AutoCloseable {

    private final ATAPHttpClient httpClient;
    private final String did;
    private final String platformDomain;
    private final Ed25519KeyPair keyPair;
    private final TokenManager tokenManager;

    /**
     * API modules.
     */
    public final EntityApi entities;
    public final ApprovalApi approvals;
    public final RevocationApi revocations;
    public final DIDCommApi didcomm;
    public final CredentialApi credentials;
    public final DiscoveryApi discovery;

    ATAPClient(String baseUrl, String did, Ed25519KeyPair keyPair,
               String clientSecret, List<String> scopes,
               String platformDomain, Duration timeout) {
        this.httpClient = new ATAPHttpClient(baseUrl, timeout);
        this.did = did;
        this.keyPair = keyPair;
        this.platformDomain = platformDomain != null ? platformDomain
                : (did != null && !did.isEmpty() ? DomainUtils.domainFromDid(did) : "localhost");

        if (keyPair != null && did != null && !did.isEmpty()) {
            this.tokenManager = new TokenManager(
                    httpClient, keyPair, did, clientSecret, scopes, this.platformDomain);
        } else {
            this.tokenManager = null;
        }

        this.entities = new EntityApi(this);
        this.approvals = new ApprovalApi(this);
        this.revocations = new RevocationApi(this);
        this.didcomm = new DIDCommApi(this);
        this.credentials = new CredentialApi(this);
        this.discovery = new DiscoveryApi(this);
    }

    /**
     * Create a new builder for ATAPClient.
     */
    public static ATAPClientBuilder builder() {
        return new ATAPClientBuilder();
    }

    /**
     * Get the token manager for manual token operations.
     *
     * @throws IllegalStateException if authentication is not configured
     */
    public TokenManager getTokenManager() {
        if (tokenManager == null) {
            throw new IllegalStateException("Token manager not initialized. Provide did and privateKey.");
        }
        return tokenManager;
    }

    /**
     * Make an authenticated request using the token manager.
     */
    public Map<String, Object> authedRequest(String method, String path,
                                              Map<String, Object> jsonBody,
                                              byte[] rawBody,
                                              String contentType,
                                              Map<String, String> params) {
        if (tokenManager == null || keyPair == null) {
            throw new IllegalStateException(
                    "Authentication not configured. Provide did, privateKey, and optionally clientSecret.");
        }

        String accessToken = tokenManager.getAccessToken();
        return httpClient.authenticatedRequest(method, path, keyPair, accessToken,
                platformDomain, jsonBody, rawBody, contentType, params);
    }

    /**
     * Get the underlying HTTP client.
     */
    public ATAPHttpClient getHttpClient() {
        return httpClient;
    }

    /**
     * Get the DID for this client.
     */
    public String getDid() {
        return did;
    }

    /**
     * Get the platform domain.
     */
    public String getPlatformDomain() {
        return platformDomain;
    }

    @Override
    public void close() {
        // HttpClient in JDK 11 doesn't need explicit close
    }
}
