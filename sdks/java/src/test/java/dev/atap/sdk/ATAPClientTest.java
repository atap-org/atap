package dev.atap.sdk;

import dev.atap.sdk.crypto.Ed25519KeyPair;
import org.junit.jupiter.api.Test;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.*;

class ATAPClientTest {

    @Test
    void testBuilderDefaults() {
        ATAPClient client = ATAPClient.builder().build();
        assertNotNull(client);
        assertNotNull(client.entities);
        assertNotNull(client.approvals);
        assertNotNull(client.revocations);
        assertNotNull(client.didcomm);
        assertNotNull(client.credentials);
        assertNotNull(client.discovery);
        client.close();
    }

    @Test
    void testBuilderWithAllOptions() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:9090")
                .did("did:web:example%3A9090:agent:abc")
                .keyPair(kp)
                .clientSecret("atap_test_secret")
                .platformDomain("example:9090")
                .timeout(Duration.ofSeconds(10))
                .build();

        assertNotNull(client);
        assertEquals("did:web:example%3A9090:agent:abc", client.getDid());
        assertEquals("example:9090", client.getPlatformDomain());
        assertNotNull(client.getTokenManager());
        client.close();
    }

    @Test
    void testBuilderWithPrivateKeyString() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String privKeyB64 = java.util.Base64.getEncoder().encodeToString(
                kp.getPrivateKey().getEncoded());

        ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .did("did:web:localhost%3A8080:agent:test")
                .privateKey(privKeyB64)
                .clientSecret("atap_secret")
                .build();

        assertNotNull(client);
        assertNotNull(client.getTokenManager());
        client.close();
    }

    @Test
    void testTokenManagerNotAvailableWithoutAuth() {
        ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .build();

        assertThrows(IllegalStateException.class, client::getTokenManager);
        client.close();
    }

    @Test
    void testAuthedRequestNotAvailableWithoutAuth() {
        ATAPClient client = ATAPClient.builder().build();

        assertThrows(IllegalStateException.class,
                () -> client.authedRequest("GET", "/test", null, null, null, null));
        client.close();
    }

    @Test
    void testPlatformDomainExtractedFromDid() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        ATAPClient client = ATAPClient.builder()
                .did("did:web:myserver%3A443:agent:abc")
                .keyPair(kp)
                .clientSecret("atap_secret")
                .build();

        assertEquals("myserver:443", client.getPlatformDomain());
        client.close();
    }

    @Test
    void testAutoCloseable() {
        try (ATAPClient client = ATAPClient.builder().build()) {
            assertNotNull(client);
        }
    }

    @Test
    void testHttpClientAccessible() {
        ATAPClient client = ATAPClient.builder()
                .baseUrl("http://example.com")
                .build();
        assertNotNull(client.getHttpClient());
        assertEquals("http://example.com", client.getHttpClient().getBaseUrl());
        client.close();
    }

    @Test
    void testBuilderWithDefaultDomain() {
        ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .build();
        assertEquals("localhost", client.getPlatformDomain());
        client.close();
    }

    @Test
    void testBuilderWithScopes() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        ATAPClient client = ATAPClient.builder()
                .did("did:web:localhost%3A8080:agent:abc")
                .keyPair(kp)
                .clientSecret("atap_secret")
                .scopes(java.util.Arrays.asList("atap:inbox", "atap:send"))
                .build();
        assertNotNull(client.getTokenManager());
        client.close();
    }
}
