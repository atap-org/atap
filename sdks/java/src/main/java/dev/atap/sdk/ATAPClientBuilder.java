package dev.atap.sdk;

import dev.atap.sdk.crypto.Ed25519KeyPair;

import java.time.Duration;
import java.util.List;

/**
 * Builder for {@link ATAPClient}.
 */
public class ATAPClientBuilder {

    private String baseUrl = "http://localhost:8080";
    private String did = "";
    private String privateKey;
    private Ed25519KeyPair keyPair;
    private String clientSecret;
    private List<String> scopes;
    private String platformDomain;
    private Duration timeout = Duration.ofSeconds(30);

    ATAPClientBuilder() {
    }

    /**
     * Set the HTTP base URL for the ATAP server.
     */
    public ATAPClientBuilder baseUrl(String baseUrl) {
        this.baseUrl = baseUrl;
        return this;
    }

    /**
     * Set the entity DID (used as client_id for OAuth).
     */
    public ATAPClientBuilder did(String did) {
        this.did = did;
        return this;
    }

    /**
     * Set the base64-encoded Ed25519 private key (seed or full key).
     */
    public ATAPClientBuilder privateKey(String privateKey) {
        this.privateKey = privateKey;
        return this;
    }

    /**
     * Set a pre-loaded Ed25519 keypair (alternative to privateKey).
     */
    public ATAPClientBuilder keyPair(Ed25519KeyPair keyPair) {
        this.keyPair = keyPair;
        return this;
    }

    /**
     * Set the client secret for agent/machine client_credentials grant.
     */
    public ATAPClientBuilder clientSecret(String clientSecret) {
        this.clientSecret = clientSecret;
        return this;
    }

    /**
     * Set OAuth scopes (defaults to all).
     */
    public ATAPClientBuilder scopes(List<String> scopes) {
        this.scopes = scopes;
        return this;
    }

    /**
     * Set the platform domain for DPoP htu construction.
     */
    public ATAPClientBuilder platformDomain(String platformDomain) {
        this.platformDomain = platformDomain;
        return this;
    }

    /**
     * Set the HTTP request timeout.
     */
    public ATAPClientBuilder timeout(Duration timeout) {
        this.timeout = timeout;
        return this;
    }

    /**
     * Build the ATAPClient.
     */
    public ATAPClient build() {
        Ed25519KeyPair resolvedKeyPair = this.keyPair;
        if (resolvedKeyPair == null && privateKey != null && !privateKey.isEmpty()) {
            resolvedKeyPair = Ed25519KeyPair.loadSigningKey(privateKey);
        }

        return new ATAPClient(baseUrl, did, resolvedKeyPair, clientSecret, scopes, platformDomain, timeout);
    }
}
