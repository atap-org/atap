package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.Revocation;
import dev.atap.sdk.model.RevocationList;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class RevocationApiTest {

    private WireMockServer wireMock;
    private ATAPClient client;

    @BeforeEach
    void setUp() {
        wireMock = new WireMockServer(WireMockConfiguration.wireMockConfig().dynamicPort());
        wireMock.start();

        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        client = ATAPClient.builder()
                .baseUrl("http://localhost:" + wireMock.port())
                .did(TestFixtures.TEST_DID)
                .keyPair(TestFixtures.getTestKeyPair())
                .clientSecret(TestFixtures.TEST_CLIENT_SECRET)
                .platformDomain(TestFixtures.TEST_PLATFORM_DOMAIN)
                .build();
    }

    @AfterEach
    void tearDown() {
        wireMock.stop();
        client.close();
    }

    @Test
    void testSubmit() {
        wireMock.stubFor(post(urlEqualTo("/v1/revocations"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.revocationResponseJson())));

        Revocation rev = client.revocations.submit("apr_abc123", "sig_data");
        assertNotNull(rev);
        assertEquals("rev_abc123", rev.getId());
        assertEquals("apr_abc123", rev.getApprovalId());
    }

    @Test
    void testSubmitWithValidUntil() {
        wireMock.stubFor(post(urlEqualTo("/v1/revocations"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.revocationResponseJson())));

        Revocation rev = client.revocations.submit("apr_abc123", "sig_data", "2025-12-31T23:59:59Z");
        assertNotNull(rev);
    }

    @Test
    void testList() {
        String listJson = "{\"entity\":\"did:web:example:agent:a\","
                + "\"revocations\":[" + TestFixtures.revocationResponseJson() + "],"
                + "\"checked_at\":\"2025-01-01T00:00:00Z\"}";

        wireMock.stubFor(get(urlPathEqualTo("/v1/revocations"))
                .withQueryParam("entity", equalTo("did:web:example:agent:a"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(listJson)));

        RevocationList rl = client.revocations.list("did:web:example:agent:a");
        assertNotNull(rl);
        assertEquals("did:web:example:agent:a", rl.getEntity());
        assertEquals(1, rl.getRevocations().size());
        assertEquals("rev_abc123", rl.getRevocations().get(0).getId());
        assertNotNull(rl.getCheckedAt());
    }

    @Test
    void testListEmpty() {
        String listJson = "{\"entity\":\"did:web:example:agent:b\","
                + "\"revocations\":[],"
                + "\"checked_at\":\"2025-01-01T00:00:00Z\"}";

        wireMock.stubFor(get(urlPathEqualTo("/v1/revocations"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(listJson)));

        RevocationList rl = client.revocations.list("did:web:example:agent:b");
        assertNotNull(rl);
        assertTrue(rl.getRevocations().isEmpty());
    }
}
