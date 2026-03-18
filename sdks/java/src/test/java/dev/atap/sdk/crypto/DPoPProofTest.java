package dev.atap.sdk.crypto;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class DPoPProofTest {

    @Test
    void testCreateWithoutAccessToken() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "POST", "https://localhost:8080/v1/oauth/token");

        assertNotNull(proof);
        String[] parts = proof.split("\\.");
        assertEquals(3, parts.length);
    }

    @Test
    void testCreateWithAccessToken() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "GET", "https://localhost:8080/v1/entities/abc",
                "test_access_token");

        assertNotNull(proof);
        String[] parts = proof.split("\\.");
        assertEquals(3, parts.length);
    }

    @Test
    @SuppressWarnings("unchecked")
    void testHeaderContainsCorrectFields() throws Exception {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "POST", "https://example.com/token");

        String[] parts = proof.split("\\.");
        byte[] headerBytes = Base64Url.decode(parts[0]);
        ObjectMapper mapper = new ObjectMapper();
        Map<String, Object> header = mapper.readValue(headerBytes, Map.class);

        assertEquals("dpop+jwt", header.get("typ"));
        assertEquals("EdDSA", header.get("alg"));
        assertNotNull(header.get("jwk"));

        Map<String, Object> jwk = (Map<String, Object>) header.get("jwk");
        assertEquals("OKP", jwk.get("kty"));
        assertEquals("Ed25519", jwk.get("crv"));
        assertNotNull(jwk.get("x"));
    }

    @Test
    @SuppressWarnings("unchecked")
    void testPayloadContainsCorrectClaims() throws Exception {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "GET", "https://localhost:8080/v1/test");

        String[] parts = proof.split("\\.");
        byte[] payloadBytes = Base64Url.decode(parts[1]);
        ObjectMapper mapper = new ObjectMapper();
        Map<String, Object> payload = mapper.readValue(payloadBytes, Map.class);

        assertEquals("GET", payload.get("htm"));
        assertEquals("https://localhost:8080/v1/test", payload.get("htu"));
        assertNotNull(payload.get("iat"));
        assertNotNull(payload.get("jti"));
        assertNull(payload.get("ath")); // no access token
    }

    @Test
    @SuppressWarnings("unchecked")
    void testPayloadContainsAthWhenAccessTokenProvided() throws Exception {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "POST", "https://localhost:8080/v1/test",
                "my_access_token");

        String[] parts = proof.split("\\.");
        byte[] payloadBytes = Base64Url.decode(parts[1]);
        ObjectMapper mapper = new ObjectMapper();
        Map<String, Object> payload = mapper.readValue(payloadBytes, Map.class);

        assertNotNull(payload.get("ath"));
    }

    @Test
    void testSignatureIsVerifiable() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof = DPoPProof.create(kp, "POST", "https://example.com/token");

        String[] parts = proof.split("\\.");
        byte[] signingInput = (parts[0] + "." + parts[1]).getBytes();
        byte[] signature = Base64Url.decode(parts[2]);

        assertTrue(kp.verify(signingInput, signature));
    }

    @Test
    void testEachProofHasUniqueJti() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String proof1 = DPoPProof.create(kp, "POST", "https://example.com/token");
        String proof2 = DPoPProof.create(kp, "POST", "https://example.com/token");

        assertNotEquals(proof1, proof2);
    }
}
