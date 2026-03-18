package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.Entity;
import dev.atap.sdk.model.KeyVersion;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class EntityApiTest {

    private WireMockServer wireMock;
    private ATAPClient client;

    @BeforeEach
    void setUp() {
        wireMock = new WireMockServer(WireMockConfiguration.wireMockConfig().dynamicPort());
        wireMock.start();

        // Stub token endpoint for authenticated requests
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
    void testRegister() {
        wireMock.stubFor(post(urlEqualTo("/v1/entities"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.entityResponseJson())));

        Entity entity = client.entities.register("agent", "Test Agent", null, null);
        assertNotNull(entity);
        assertEquals("agent", entity.getType());
        assertEquals("Test Agent", entity.getName());
        assertEquals(TestFixtures.TEST_ENTITY_ID, entity.getId());
    }

    @Test
    void testRegisterMinimal() {
        wireMock.stubFor(post(urlEqualTo("/v1/entities"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.entityResponseJson())));

        Entity entity = client.entities.register("agent");
        assertNotNull(entity);
        assertEquals("agent", entity.getType());
    }

    @Test
    void testGet() {
        wireMock.stubFor(get(urlEqualTo("/v1/entities/" + TestFixtures.TEST_ENTITY_ID))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.entityResponseJson())));

        Entity entity = client.entities.get(TestFixtures.TEST_ENTITY_ID);
        assertNotNull(entity);
        assertEquals(TestFixtures.TEST_ENTITY_ID, entity.getId());
        assertEquals(TestFixtures.TEST_DID, entity.getDid());
        assertEquals(1, entity.getTrustLevel());
    }

    @Test
    void testDelete() {
        wireMock.stubFor(delete(urlEqualTo("/v1/entities/" + TestFixtures.TEST_ENTITY_ID))
                .willReturn(aResponse().withStatus(204)));

        assertDoesNotThrow(() -> client.entities.delete(TestFixtures.TEST_ENTITY_ID));
    }

    @Test
    void testRotateKey() {
        String keyVersionJson = "{\"id\":\"key_v2\","
                + "\"entity_id\":\"" + TestFixtures.TEST_ENTITY_ID + "\","
                + "\"key_index\":2,"
                + "\"valid_from\":\"2025-01-02T00:00:00Z\","
                + "\"created_at\":\"2025-01-02T00:00:00Z\"}";

        wireMock.stubFor(post(urlEqualTo("/v1/entities/" + TestFixtures.TEST_ENTITY_ID + "/keys/rotate"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(keyVersionJson)));

        KeyVersion kv = client.entities.rotateKey(TestFixtures.TEST_ENTITY_ID, "newpubkey==");
        assertNotNull(kv);
        assertEquals("key_v2", kv.getId());
        assertEquals(2, kv.getKeyIndex());
        assertEquals(TestFixtures.TEST_ENTITY_ID, kv.getEntityId());
    }

    @Test
    void testRegisterWithPublicKey() {
        String responseWithSecret = "{\"id\":\"newid\","
                + "\"type\":\"agent\","
                + "\"did\":\"did:web:example:agent:newid\","
                + "\"name\":\"My Agent\","
                + "\"key_id\":\"key_test\","
                + "\"public_key\":\"testkey==\","
                + "\"client_secret\":\"atap_secretvalue\","
                + "\"trust_level\":0,"
                + "\"created_at\":\"2025-01-01T00:00:00Z\","
                + "\"updated_at\":\"2025-01-01T00:00:00Z\"}";

        wireMock.stubFor(post(urlEqualTo("/v1/entities"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(responseWithSecret)));

        Entity entity = client.entities.register("agent", "My Agent", "testkey==", null);
        assertNotNull(entity);
        assertEquals("atap_secretvalue", entity.getClientSecret());
    }
}
