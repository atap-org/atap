package dev.atap.sdk.oauth;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.http.ATAPHttpClient;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Arrays;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class TokenManagerTest {

    private WireMockServer wireMock;
    private ATAPHttpClient httpClient;
    private Ed25519KeyPair keyPair;

    @BeforeEach
    void setUp() {
        wireMock = new WireMockServer(WireMockConfiguration.wireMockConfig().dynamicPort());
        wireMock.start();
        httpClient = new ATAPHttpClient("http://localhost:" + wireMock.port());
        keyPair = TestFixtures.getTestKeyPair();
    }

    @AfterEach
    void tearDown() {
        wireMock.stop();
    }

    @Test
    void testGetAccessTokenClientCredentials() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        String token = tm.getAccessToken();
        assertEquals(TestFixtures.TEST_ACCESS_TOKEN, token);
    }

    @Test
    void testGetAccessTokenCachedOnSecondCall() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        String token1 = tm.getAccessToken();
        String token2 = tm.getAccessToken();
        assertEquals(token1, token2);

        // Should only have made one request
        wireMock.verify(1, postRequestedFor(urlEqualTo("/v1/oauth/token")));
    }

    @Test
    void testInvalidate() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        tm.getAccessToken();
        tm.invalidate();
        tm.getAccessToken();

        // After invalidation, should make another request
        wireMock.verify(2, postRequestedFor(urlEqualTo("/v1/oauth/token")));
    }

    @Test
    void testThrowsWithoutClientSecret() {
        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, null,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        assertThrows(IllegalStateException.class, tm::getAccessToken);
    }

    @Test
    void testCustomScopes() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                Arrays.asList("atap:inbox", "atap:send"), TestFixtures.TEST_PLATFORM_DOMAIN);

        String token = tm.getAccessToken();
        assertNotNull(token);

        wireMock.verify(postRequestedFor(urlEqualTo("/v1/oauth/token"))
                .withRequestBody(containing("scope=atap%3Ainbox+atap%3Asend")));
    }

    @Test
    void testRefreshToken() {
        String responseWithRefresh = "{\"access_token\":\"token_v1\","
                + "\"token_type\":\"DPoP\","
                + "\"expires_in\":1," // expires in 1 second
                + "\"scope\":\"atap:inbox\","
                + "\"refresh_token\":\"refresh_abc\"}";

        String refreshedResponse = "{\"access_token\":\"token_v2\","
                + "\"token_type\":\"DPoP\","
                + "\"expires_in\":3600,"
                + "\"scope\":\"atap:inbox\"}";

        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(responseWithRefresh)));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        // Get first token
        String token1 = tm.getAccessToken();
        assertEquals("token_v1", token1);

        // Invalidate so next call triggers refresh
        tm.invalidate();

        // Set up refresh response
        wireMock.resetAll();
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(refreshedResponse)));

        String token2 = tm.getAccessToken();
        assertNotNull(token2);
    }

    @Test
    void testDPoPProofSentInRequest() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        tm.getAccessToken();

        wireMock.verify(postRequestedFor(urlEqualTo("/v1/oauth/token"))
                .withHeader("DPoP", matching(".*\\..*\\..*")));
    }

    @Test
    void testObtainAuthorizationCode() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)
                        .withHeader("Location", "atap://callback?code=auth_code_123")));

        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"access_token\":\"token_authcode\","
                                + "\"token_type\":\"DPoP\","
                                + "\"expires_in\":3600,"
                                + "\"scope\":\"atap:inbox\","
                                + "\"refresh_token\":\"refresh_authcode\"}")));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        OAuthToken token = tm.obtainAuthorizationCode("atap://callback");
        assertNotNull(token);
        assertEquals("token_authcode", token.getAccessToken());
        assertEquals("DPoP", token.getTokenType());
        assertEquals(3600, token.getExpiresIn());
        assertEquals("atap:inbox", token.getScope());
        assertEquals("refresh_authcode", token.getRefreshToken());
    }

    @Test
    void testObtainAuthorizationCodeDefaultRedirect() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)
                        .withHeader("Location", "atap://callback?code=code_default")));

        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        OAuthToken token = tm.obtainAuthorizationCode();
        assertNotNull(token);
    }

    @Test
    void testObtainAuthorizationCodeNoCode() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)
                        .withHeader("Location", "atap://callback?error=access_denied")));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        assertThrows(dev.atap.sdk.exception.ATAPException.class,
                () -> tm.obtainAuthorizationCode("atap://callback"));
    }

    @Test
    void testRefreshTokenPreservedWhenNotReturnedOnRefresh() {
        String responseWithRefresh = "{\"access_token\":\"token_v1\","
                + "\"token_type\":\"DPoP\","
                + "\"expires_in\":1,"
                + "\"scope\":\"atap:inbox\","
                + "\"refresh_token\":\"refresh_original\"}";

        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(responseWithRefresh)));

        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, TestFixtures.TEST_CLIENT_SECRET,
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        String token1 = tm.getAccessToken();
        assertEquals("token_v1", token1);

        tm.invalidate();

        String refreshedNoRefresh = "{\"access_token\":\"token_v2\","
                + "\"token_type\":\"DPoP\","
                + "\"expires_in\":3600,"
                + "\"scope\":\"atap:inbox\"}";

        wireMock.resetAll();
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(refreshedNoRefresh)));

        String token2 = tm.getAccessToken();
        assertEquals("token_v2", token2);
    }

    @Test
    void testEmptyClientSecretThrows() {
        TokenManager tm = new TokenManager(httpClient, keyPair,
                TestFixtures.TEST_DID, "",
                null, TestFixtures.TEST_PLATFORM_DOMAIN);

        assertThrows(IllegalStateException.class, tm::getAccessToken);
    }
}
