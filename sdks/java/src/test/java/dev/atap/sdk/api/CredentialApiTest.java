package dev.atap.sdk.api;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.model.Credential;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class CredentialApiTest {

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
    void testStartEmail() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/email/start"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"message\":\"OTP sent to test@example.com\"}")));

        String result = client.credentials.startEmail("test@example.com");
        assertEquals("OTP sent to test@example.com", result);
    }

    @Test
    void testVerifyEmail() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/email/verify"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.credentialResponseJson())));

        Credential cred = client.credentials.verifyEmail("test@example.com", "123456");
        assertNotNull(cred);
        assertEquals("cred_abc123", cred.getId());
        assertEquals("ATAPEmailVerification", cred.getType());
    }

    @Test
    void testStartPhone() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/phone/start"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"message\":\"OTP sent\"}")));

        String result = client.credentials.startPhone("+15551234567");
        assertEquals("OTP sent", result);
    }

    @Test
    void testVerifyPhone() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/phone/verify"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.credentialResponseJson())));

        Credential cred = client.credentials.verifyPhone("+15551234567", "654321");
        assertNotNull(cred);
    }

    @Test
    void testSubmitPersonhood() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/personhood"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.credentialResponseJson())));

        Credential cred = client.credentials.submitPersonhood("provider_token_123");
        assertNotNull(cred);
        assertEquals("cred_abc123", cred.getId());
    }

    @Test
    void testSubmitPersonhoodWithoutToken() {
        wireMock.stubFor(post(urlEqualTo("/v1/credentials/personhood"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.credentialResponseJson())));

        Credential cred = client.credentials.submitPersonhood();
        assertNotNull(cred);
    }

    @Test
    void testList() {
        String listJson = "{\"credentials\":[" + TestFixtures.credentialResponseJson() + "]}";

        wireMock.stubFor(get(urlEqualTo("/v1/credentials"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(listJson)));

        List<Credential> creds = client.credentials.list();
        assertNotNull(creds);
        assertEquals(1, creds.size());
        assertEquals("cred_abc123", creds.get(0).getId());
    }

    @Test
    void testStatusList() {
        wireMock.stubFor(get(urlEqualTo("/v1/credentials/status/1"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"type\":\"BitstringStatusListCredential\"}")));

        Map<String, Object> result = client.credentials.statusList();
        assertEquals("BitstringStatusListCredential", result.get("type"));
    }

    @Test
    void testStatusListCustomId() {
        wireMock.stubFor(get(urlEqualTo("/v1/credentials/status/42"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"type\":\"BitstringStatusListCredential\"}")));

        Map<String, Object> result = client.credentials.statusList("42");
        assertNotNull(result);
    }
}
