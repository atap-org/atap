package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.Approval;
import dev.atap.sdk.model.ApprovalSubject;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Collections;
import java.util.List;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class ApprovalApiTest {

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
    void testCreate() {
        wireMock.stubFor(post(urlEqualTo("/v1/approvals"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.approvalResponseJson())));

        ApprovalSubject subject = new ApprovalSubject("data_access", "Access user data", true, null);
        Approval approval = client.approvals.create("did:web:example:agent:a",
                "did:web:example:agent:b", subject);

        assertNotNull(approval);
        assertEquals("apr_abc123", approval.getId());
        assertEquals("pending", approval.getState());
        assertEquals("did:web:example:agent:a", approval.getFromDid());
        assertEquals("did:web:example:agent:b", approval.getToDid());
        assertNotNull(approval.getSubject());
        assertEquals("data_access", approval.getSubject().getType());
    }

    @Test
    void testCreateWithVia() {
        wireMock.stubFor(post(urlEqualTo("/v1/approvals"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.approvalResponseJson())));

        ApprovalSubject subject = new ApprovalSubject("data_access", "Access user data", false,
                Collections.singletonMap("resource", "users"));
        Approval approval = client.approvals.create("did:web:example:agent:a",
                "did:web:example:agent:b", subject, "did:web:example:machine:proxy");

        assertNotNull(approval);
    }

    @Test
    void testRespond() {
        String respondedJson = "{\"id\":\"apr_abc123\","
                + "\"state\":\"approved\","
                + "\"from\":\"did:web:example:agent:a\","
                + "\"to\":\"did:web:example:agent:b\","
                + "\"subject\":{\"type\":\"data_access\",\"label\":\"Access\"},"
                + "\"responded_at\":\"2025-01-01T01:00:00Z\","
                + "\"signatures\":{\"approver\":\"sig_data\"}}";

        wireMock.stubFor(post(urlEqualTo("/v1/approvals/apr_abc123/respond"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(respondedJson)));

        Approval approval = client.approvals.respond("apr_abc123", "sig_data");
        assertNotNull(approval);
        assertEquals("approved", approval.getState());
        assertNotNull(approval.getRespondedAt());
    }

    @Test
    void testList() {
        String listJson = "{\"approvals\":[" + TestFixtures.approvalResponseJson() + "]}";

        wireMock.stubFor(get(urlEqualTo("/v1/approvals"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(listJson)));

        List<Approval> approvals = client.approvals.list();
        assertNotNull(approvals);
        assertEquals(1, approvals.size());
        assertEquals("apr_abc123", approvals.get(0).getId());
    }

    @Test
    void testRevoke() {
        wireMock.stubFor(delete(urlEqualTo("/v1/approvals/apr_abc123"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"id\":\"apr_abc123\",\"state\":\"revoked\"}")));

        Approval approval = client.approvals.revoke("apr_abc123");
        assertNotNull(approval);
        assertEquals("revoked", approval.getState());
    }
}
