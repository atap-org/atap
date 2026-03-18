package dev.atap.sdk.http;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;
import dev.atap.sdk.TestFixtures;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.exception.*;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

import static com.github.tomakehurst.wiremock.client.WireMock.*;
import static org.junit.jupiter.api.Assertions.*;

class ATAPHttpClientTest {

    private WireMockServer wireMock;
    private ATAPHttpClient client;

    @BeforeEach
    void setUp() {
        wireMock = new WireMockServer(WireMockConfiguration.wireMockConfig().dynamicPort());
        wireMock.start();
        client = new ATAPHttpClient("http://localhost:" + wireMock.port());
    }

    @AfterEach
    void tearDown() {
        wireMock.stop();
    }

    @Test
    void testGetRequest() {
        wireMock.stubFor(get(urlEqualTo("/v1/health"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"status\":\"ok\"}")));

        Map<String, Object> result = client.request("GET", "/v1/health", null, null, null);
        assertEquals("ok", result.get("status"));
    }

    @Test
    void testPostRequestWithJsonBody() {
        wireMock.stubFor(post(urlEqualTo("/v1/entities"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.entityResponseJson())));

        Map<String, Object> body = Collections.singletonMap("type", "agent");
        Map<String, Object> result = client.request("POST", "/v1/entities", body, null, null);
        assertEquals("agent", result.get("type"));
    }

    @Test
    void testDeleteRequest() {
        wireMock.stubFor(delete(urlEqualTo("/v1/entities/abc"))
                .willReturn(aResponse().withStatus(204)));

        Map<String, Object> result = client.request("DELETE", "/v1/entities/abc", null, null, null);
        assertTrue(result.isEmpty());
    }

    @Test
    void testRequestWithQueryParams() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/revocations"))
                .withQueryParam("entity", equalTo("did:web:example"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"revocations\":[]}")));

        Map<String, String> params = Collections.singletonMap("entity", "did:web:example");
        Map<String, Object> result = client.request("GET", "/v1/revocations", null, null, params);
        assertNotNull(result);
    }

    @Test
    void testAuthenticatedRequest() {
        wireMock.stubFor(get(urlEqualTo("/v1/entities/abc"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.entityResponseJson())));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        Map<String, Object> result = client.authenticatedRequest("GET", "/v1/entities/abc",
                kp, "test_token", "localhost:8080", null, null, null, null);

        assertNotNull(result);
        wireMock.verify(getRequestedFor(urlEqualTo("/v1/entities/abc"))
                .withHeader("Authorization", equalTo("DPoP test_token"))
                .withHeader("DPoP", matching(".*\\..*\\..*")));
    }

    @Test
    void testPostFormData() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        Map<String, String> formData = new LinkedHashMap<>();
        formData.put("grant_type", "client_credentials");
        formData.put("client_id", "test");

        Map<String, Object> result = client.postForm("/v1/oauth/token", formData, "dpop_proof_here");
        assertEquals(TestFixtures.TEST_ACCESS_TOKEN, result.get("access_token"));

        wireMock.verify(postRequestedFor(urlEqualTo("/v1/oauth/token"))
                .withHeader("Content-Type", containing("application/x-www-form-urlencoded"))
                .withHeader("DPoP", equalTo("dpop_proof_here")));
    }

    @Test
    void test401ThrowsATAPAuthException() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(401)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(401, "Unauthorized", "Invalid token"))));

        ATAPAuthException ex = assertThrows(ATAPAuthException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(401, ex.getStatusCode());
        assertNotNull(ex.getProblem());
    }

    @Test
    void test403ThrowsATAPAuthException() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(403)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(403, "Forbidden", "Insufficient scope"))));

        ATAPAuthException ex = assertThrows(ATAPAuthException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(403, ex.getStatusCode());
    }

    @Test
    void test404ThrowsATAPNotFoundException() {
        wireMock.stubFor(get(urlEqualTo("/v1/entities/missing"))
                .willReturn(aResponse()
                        .withStatus(404)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(404, "Not Found", "Entity not found"))));

        ATAPNotFoundException ex = assertThrows(ATAPNotFoundException.class,
                () -> client.request("GET", "/v1/entities/missing", null, null, null));
        assertEquals(404, ex.getStatusCode());
        assertNotNull(ex.getProblem());
    }

    @Test
    void test409ThrowsATAPConflictException() {
        wireMock.stubFor(post(urlEqualTo("/v1/entities"))
                .willReturn(aResponse()
                        .withStatus(409)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(409, "Conflict", "Entity already exists"))));

        Map<String, Object> body = Collections.singletonMap("type", "agent");
        ATAPConflictException ex = assertThrows(ATAPConflictException.class,
                () -> client.request("POST", "/v1/entities", body, null, null));
        assertEquals(409, ex.getStatusCode());
    }

    @Test
    void test429ThrowsATAPRateLimitException() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(429)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(429, "Too Many Requests", "Rate limit exceeded"))));

        ATAPRateLimitException ex = assertThrows(ATAPRateLimitException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(429, ex.getStatusCode());
    }

    @Test
    void test500ThrowsATAPProblemException() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(500)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(500, "Internal Server Error", "Something went wrong"))));

        ATAPProblemException ex = assertThrows(ATAPProblemException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(500, ex.getStatusCode());
    }

    @Test
    void testNonJsonErrorThrowsATAPException() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(502)
                        .withBody("Bad Gateway")));

        ATAPException ex = assertThrows(ATAPException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(502, ex.getStatusCode());
    }

    @Test
    void testGetRedirect() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)
                        .withHeader("Location", "atap://callback?code=abc123")));

        String location = client.getRedirect("/v1/oauth/authorize", null, null);
        assertEquals("atap://callback?code=abc123", location);
    }

    @Test
    void testGetRedirectNon302Throws() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"error\":\"unexpected\"}")));

        assertThrows(ATAPException.class,
                () -> client.getRedirect("/v1/oauth/authorize", null, null));
    }

    @Test
    void testAuthenticatedRequestWithRawBody() {
        wireMock.stubFor(post(urlEqualTo("/v1/didcomm"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"id\":\"msg_1\",\"status\":\"queued\"}")));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        byte[] rawBody = "{\"test\":true}".getBytes();
        Map<String, Object> result = client.authenticatedRequest("POST", "/v1/didcomm",
                kp, "test_token", "localhost:8080", null, rawBody,
                "application/didcomm-encrypted+json", null);

        assertEquals("queued", result.get("status"));
    }

    @Test
    void testPostFormWithoutDPoP() {
        wireMock.stubFor(post(urlEqualTo("/v1/oauth/token"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.tokenResponseJson())));

        Map<String, String> formData = Collections.singletonMap("grant_type", "client_credentials");
        Map<String, Object> result = client.postForm("/v1/oauth/token", formData, null);
        assertNotNull(result);

        wireMock.verify(postRequestedFor(urlEqualTo("/v1/oauth/token"))
                .withoutHeader("DPoP"));
    }

    @Test
    void testGetBaseUrl() {
        assertEquals("http://localhost:" + wireMock.port(), client.getBaseUrl());
    }

    @Test
    void testAuthenticatedRequestWithJsonBody() {
        wireMock.stubFor(post(urlEqualTo("/v1/approvals"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"id\":\"apr_123\",\"state\":\"pending\"}")));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        Map<String, Object> jsonBody = new LinkedHashMap<>();
        jsonBody.put("from", "did:web:a");
        jsonBody.put("to", "did:web:b");

        Map<String, Object> result = client.authenticatedRequest("POST", "/v1/approvals",
                kp, "test_token", "localhost:8080", jsonBody, null, null, null);

        assertNotNull(result);
        assertEquals("apr_123", result.get("id"));

        wireMock.verify(postRequestedFor(urlEqualTo("/v1/approvals"))
                .withHeader("Authorization", equalTo("DPoP test_token"))
                .withHeader("DPoP", matching(".*\\..*\\..*"))
                .withHeader("Content-Type", equalTo("application/json")));
    }

    @Test
    void testAuthenticatedDeleteRequest() {
        wireMock.stubFor(delete(urlEqualTo("/v1/entities/abc"))
                .willReturn(aResponse().withStatus(204)));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        Map<String, Object> result = client.authenticatedRequest("DELETE", "/v1/entities/abc",
                kp, "test_token", "localhost:8080", null, null, null, null);

        assertTrue(result.isEmpty());
    }

    @Test
    void testAuthenticatedRequestWithParams() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/didcomm/inbox"))
                .withQueryParam("limit", equalTo("50"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"messages\":[],\"count\":0}")));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        Map<String, String> params = Collections.singletonMap("limit", "50");
        Map<String, Object> result = client.authenticatedRequest("GET", "/v1/didcomm/inbox",
                kp, "test_token", "localhost:8080", null, null, null, params);

        assertNotNull(result);
        assertEquals(0, result.get("count"));
    }

    @Test
    void testAuthenticatedRawBodyWithoutContentType() {
        wireMock.stubFor(post(urlEqualTo("/v1/upload"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"ok\":true}")));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        byte[] rawBody = "binary data".getBytes();
        Map<String, Object> result = client.authenticatedRequest("POST", "/v1/upload",
                kp, "test_token", "localhost:8080", null, rawBody, null, null);

        assertNotNull(result);
        wireMock.verify(postRequestedFor(urlEqualTo("/v1/upload"))
                .withHeader("Content-Type", equalTo("application/octet-stream")));
    }

    @Test
    void testAuthenticatedPutNoBody() {
        wireMock.stubFor(any(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"ok\":true}")));

        Ed25519KeyPair kp = TestFixtures.getTestKeyPair();
        Map<String, Object> result = client.authenticatedRequest("PUT", "/v1/test",
                kp, "test_token", "localhost:8080", null, null, null, null);

        assertNotNull(result);
    }

    @Test
    void testPutRequestNoBody() {
        wireMock.stubFor(any(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"ok\":true}")));

        Map<String, Object> result = client.request("PUT", "/v1/test", null, null, null);
        assertNotNull(result);
    }

    @Test
    void testRequestWithCustomHeaders() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"ok\":true}")));

        Map<String, String> headers = Collections.singletonMap("X-Custom", "value");
        Map<String, Object> result = client.request("GET", "/v1/test", null, headers, null);
        assertNotNull(result);

        wireMock.verify(getRequestedFor(urlEqualTo("/v1/test"))
                .withHeader("X-Custom", equalTo("value")));
    }

    @Test
    void testGetRedirectWithParamsAndDPoP() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)
                        .withHeader("Location", "atap://callback?code=xyz")));

        Map<String, String> params = new LinkedHashMap<>();
        params.put("response_type", "code");
        params.put("client_id", "did:web:example");

        String location = client.getRedirect("/v1/oauth/authorize", params, "dpop_proof_123");
        assertEquals("atap://callback?code=xyz", location);

        wireMock.verify(getRequestedFor(urlPathEqualTo("/v1/oauth/authorize"))
                .withHeader("DPoP", equalTo("dpop_proof_123"))
                .withQueryParam("response_type", equalTo("code")));
    }

    @Test
    void testGetRedirect302NoLocationThrows() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(302)));

        assertThrows(ATAPException.class,
                () -> client.getRedirect("/v1/oauth/authorize", null, null));
    }

    @Test
    void testGetRedirectErrorResponse() {
        wireMock.stubFor(get(urlPathEqualTo("/v1/oauth/authorize"))
                .willReturn(aResponse()
                        .withStatus(401)
                        .withHeader("Content-Type", "application/json")
                        .withBody(TestFixtures.problemDetailJson(401, "Unauthorized", "Bad token"))));

        assertThrows(ATAPAuthException.class,
                () -> client.getRedirect("/v1/oauth/authorize", null, null));
    }

    @Test
    void testErrorWithNonProblemJson() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(422)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"message\":\"Validation failed\",\"detail\":\"bad input\"}")));

        ATAPException ex = assertThrows(ATAPException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(422, ex.getStatusCode());
        assertTrue(ex.getMessage().contains("bad input"));
    }

    @Test
    void testTrailingSlashStripped() {
        ATAPHttpClient c = new ATAPHttpClient("http://example.com///");
        assertEquals("http://example.com", c.getBaseUrl());
    }

    @Test
    void testPathWithoutLeadingSlash() {
        wireMock.stubFor(get(urlEqualTo("/v1/health"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"status\":\"ok\"}")));

        Map<String, Object> result = client.request("GET", "v1/health", null, null, null);
        assertEquals("ok", result.get("status"));
    }

    @Test
    void testSuccessNonJsonBody() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withBody("plain text response")));

        Map<String, Object> result = client.request("GET", "/v1/test", null, null, null);
        assertTrue(result.isEmpty());
    }

    @Test
    void testObjectMapper() {
        assertNotNull(client.getObjectMapper());
    }

    @Test
    void testCustomTimeout() {
        ATAPHttpClient c = new ATAPHttpClient("http://localhost:1234", java.time.Duration.ofSeconds(5));
        assertNotNull(c);
        assertEquals("http://localhost:1234", c.getBaseUrl());
    }

    @Test
    void test401WithoutProblemDetail() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(401)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"detail\":\"token expired\"}")));

        ATAPAuthException ex = assertThrows(ATAPAuthException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(401, ex.getStatusCode());
        assertTrue(ex.getMessage().contains("token expired"));
    }

    @Test
    void test404WithoutProblemDetail() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(404)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"error\":\"not found\"}")));

        ATAPNotFoundException ex = assertThrows(ATAPNotFoundException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(404, ex.getStatusCode());
    }

    @Test
    void test409WithoutProblemDetail() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(409)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"error\":\"conflict\"}")));

        ATAPConflictException ex = assertThrows(ATAPConflictException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(409, ex.getStatusCode());
    }

    @Test
    void test429WithoutProblemDetail() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(429)
                        .withHeader("Content-Type", "application/json")
                        .withBody("{\"error\":\"rate limited\"}")));

        ATAPRateLimitException ex = assertThrows(ATAPRateLimitException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(429, ex.getStatusCode());
    }

    @Test
    void testEmptyResponseBody() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(200)
                        .withBody("")));

        Map<String, Object> result = client.request("GET", "/v1/test", null, null, null);
        assertTrue(result.isEmpty());
    }

    @Test
    void testErrorWithEmptyBody() {
        wireMock.stubFor(get(urlEqualTo("/v1/test"))
                .willReturn(aResponse()
                        .withStatus(500)
                        .withBody("")));

        ATAPException ex = assertThrows(ATAPException.class,
                () -> client.request("GET", "/v1/test", null, null, null));
        assertEquals(500, ex.getStatusCode());
    }
}
