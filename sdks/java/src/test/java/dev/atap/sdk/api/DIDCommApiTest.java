package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.DIDCommInbox;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class DIDCommApiTest {

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
    void testSend() {
        wireMock.stubFor(post(urlEqualTo("/v1/didcomm"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"id\":\"msg_1\",\"status\":\"queued\"}")));

        Map<String, Object> result = client.didcomm.send("jwe_bytes".getBytes());
        assertNotNull(result);
        assertEquals("queued", result.get("status"));
    }

    @Test
    void testInbox() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/didcomm/inbox"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.didcommInboxJson())));

        DIDCommInbox inbox = client.didcomm.inbox();
        assertNotNull(inbox);
        assertEquals(1, inbox.getCount());
        assertEquals(1, inbox.getMessages().size());
        assertEquals("msg_1", inbox.getMessages().get(0).getId());
        assertEquals("did:web:example:agent:sender", inbox.getMessages().get(0).getSenderDid());
        assertEquals("notification", inbox.getMessages().get(0).getMessageType());
    }

    @Test
    void testInboxWithLimit() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/didcomm/inbox"))
                .withQueryParam("limit", equalTo("10"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"messages\":[],\"count\":0}")));

        DIDCommInbox inbox = client.didcomm.inbox(10);
        assertNotNull(inbox);
        assertEquals(0, inbox.getCount());
        assertTrue(inbox.getMessages().isEmpty());
    }

    @Test
    void testInboxLimitCappedAt100() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/didcomm/inbox"))
                .withQueryParam("limit", equalTo("100"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"messages\":[],\"count\":0}")));

        DIDCommInbox inbox = client.didcomm.inbox(200);
        assertNotNull(inbox);
    }
}
