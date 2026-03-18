package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.DIDDocument;
import dev.atap.sdk.model.DiscoveryDocument;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class DiscoveryApiTest {

    private WireMockServer wireMock;
    private ATAPClient client;

    @BeforeEach
    void setUp() {
        wireMock = new WireMockServer(WireMockConfiguration.wireMockConfig().dynamicPort());
        wireMock.start();

        client = ATAPClient.builder()
                .baseUrl("http://localhost:" + wireMock.port())
                .build();
    }

    @AfterEach
    void tearDown() {
        wireMock.stop();
        client.close();
    }

    @Test
    void testDiscover() {
        wireMock.stubFor(get(urlEqualTo("/.well-known/atap.json"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.discoveryDocumentJson())));

        DiscoveryDocument doc = client.discovery.discover();
        assertNotNull(doc);
        assertEquals("localhost:8080", doc.getDomain());
        assertEquals("http://localhost:8080", doc.getApiBase());
        assertEquals(3, doc.getClaimTypes().size());
        assertTrue(doc.getClaimTypes().contains("email"));
        assertEquals(1, doc.getTrustLevel());
    }

    @Test
    void testResolveDid() {
        wireMock.stubFor(get(urlEqualTo("/agent/test123/did.json"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.didDocumentJson())));

        DIDDocument doc = client.discovery.resolveDid("agent", "test123");
        assertNotNull(doc);
        assertEquals(TestFixtures.TEST_DID, doc.getId());
        assertEquals(1, doc.getVerificationMethod().size());
        assertEquals("Ed25519VerificationKey2020", doc.getVerificationMethod().get(0).getType());
        assertEquals("z6Mktest", doc.getVerificationMethod().get(0).getPublicKeyMultibase());
    }

    @Test
    void testServerDid() {
        wireMock.stubFor(get(urlEqualTo("/server/platform/did.json"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.didDocumentJson())));

        DIDDocument doc = client.discovery.serverDid();
        assertNotNull(doc);
        assertNotNull(doc.getId());
        assertEquals("agent", doc.getAtapType());
    }

    @Test
    void testHealth() {
        wireMock.stubFor(get(urlEqualTo("/v1/health"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.healthResponseJson())));

        Map<String, Object> result = client.discovery.health();
        assertNotNull(result);
        assertEquals("ok", result.get("status"));
        assertEquals("0.1.0", result.get("version"));
    }

    @Test
    void testDiscoverWithContext() {
        wireMock.stubFor(get(urlEqualTo("/.well-known/atap.json"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.discoveryDocumentJson())));

        DiscoveryDocument doc = client.discovery.discover();
        assertEquals("http://localhost:8080/v1/didcomm", doc.getDidcommEndpoint());
        assertEquals("24h", doc.getMaxApprovalTtl());
    }
}
