package dev.atap.sdk;

import dev.atap.sdk.crypto.Ed25519KeyPair;

import java.util.Base64;

/**
 * Shared test helpers and fixtures.
 */
public final class TestFixtures {

    public static final String TEST_DID = "did:web:localhost%3A8080:agent:test123";
    public static final String TEST_PLATFORM_DOMAIN = "localhost:8080";
    public static final String TEST_CLIENT_SECRET = "atap_dGVzdF9jbGllbnRfc2VjcmV0X3ZhbHVl";
    public static final String TEST_ACCESS_TOKEN = "test_access_token_value";
    public static final String TEST_ENTITY_ID = "01ARZ3NDEKTSV4RRFFQ69G5FAV";

    private static Ed25519KeyPair testKeyPair;

    private TestFixtures() {
    }

    /**
     * Get a stable test keypair (generated once per test run).
     */
    public static synchronized Ed25519KeyPair getTestKeyPair() {
        if (testKeyPair == null) {
            testKeyPair = Ed25519KeyPair.generate();
        }
        return testKeyPair;
    }

    /**
     * Get the base64-encoded private key seed for the test keypair.
     */
    public static String getTestPrivateKeyB64() {
        Ed25519KeyPair kp = getTestKeyPair();
        // BouncyCastle Ed25519PrivateKeyParameters.getEncoded() returns the 32-byte seed
        return Base64.getEncoder().encodeToString(kp.getPrivateKey().getEncoded());
    }

    /**
     * Standard OAuth token response JSON.
     */
    public static String tokenResponseJson() {
        return "{\"access_token\":\"" + TEST_ACCESS_TOKEN + "\","
                + "\"token_type\":\"DPoP\","
                + "\"expires_in\":3600,"
                + "\"scope\":\"atap:inbox atap:send atap:revoke atap:manage\"}";
    }

    /**
     * Standard entity response JSON.
     */
    public static String entityResponseJson() {
        return "{\"id\":\"" + TEST_ENTITY_ID + "\","
                + "\"type\":\"agent\","
                + "\"did\":\"" + TEST_DID + "\","
                + "\"name\":\"Test Agent\","
                + "\"key_id\":\"key_test_abc\","
                + "\"public_key\":\"dGVzdHB1YmtleQ==\","
                + "\"trust_level\":1,"
                + "\"created_at\":\"2025-01-01T00:00:00Z\","
                + "\"updated_at\":\"2025-01-01T00:00:00Z\"}";
    }

    /**
     * Standard approval response JSON.
     */
    public static String approvalResponseJson() {
        return "{\"id\":\"apr_abc123\","
                + "\"state\":\"pending\","
                + "\"from\":\"did:web:example:agent:a\","
                + "\"to\":\"did:web:example:agent:b\","
                + "\"subject\":{\"type\":\"data_access\",\"label\":\"Access user data\",\"reversible\":true},"
                + "\"created_at\":\"2025-01-01T00:00:00Z\","
                + "\"signatures\":{}}";
    }

    /**
     * Standard revocation response JSON.
     */
    public static String revocationResponseJson() {
        return "{\"id\":\"rev_abc123\","
                + "\"approval_id\":\"apr_abc123\","
                + "\"approver_did\":\"did:web:example:agent:a\","
                + "\"revoked_at\":\"2025-01-01T00:00:00Z\","
                + "\"expires_at\":\"2025-01-01T01:00:00Z\"}";
    }

    /**
     * Standard credential response JSON.
     */
    public static String credentialResponseJson() {
        return "{\"id\":\"cred_abc123\","
                + "\"type\":\"ATAPEmailVerification\","
                + "\"credential\":\"eyJhbGciOiJFZERTQSJ9...\","
                + "\"issued_at\":\"2025-01-01T00:00:00Z\"}";
    }

    /**
     * Standard discovery document JSON.
     */
    public static String discoveryDocumentJson() {
        return "{\"domain\":\"localhost:8080\","
                + "\"api_base\":\"http://localhost:8080\","
                + "\"didcomm_endpoint\":\"http://localhost:8080/v1/didcomm\","
                + "\"claim_types\":[\"email\",\"phone\",\"personhood\"],"
                + "\"max_approval_ttl\":\"24h\","
                + "\"trust_level\":1}";
    }

    /**
     * Standard DID document JSON.
     */
    public static String didDocumentJson() {
        return "{\"id\":\"" + TEST_DID + "\","
                + "\"@context\":[\"https://www.w3.org/ns/did/v1\"],"
                + "\"verificationMethod\":[{\"id\":\"" + TEST_DID + "#key-1\","
                + "\"type\":\"Ed25519VerificationKey2020\","
                + "\"controller\":\"" + TEST_DID + "\","
                + "\"publicKeyMultibase\":\"z6Mktest\"}],"
                + "\"authentication\":[\"" + TEST_DID + "#key-1\"],"
                + "\"assertionMethod\":[\"" + TEST_DID + "#key-1\"],"
                + "\"keyAgreement\":[],"
                + "\"service\":[],"
                + "\"atap:type\":\"agent\","
                + "\"atap:principal\":\"\"}";
    }

    /**
     * Problem detail JSON for a 404 error.
     */
    public static String problemDetailJson(int status, String title, String detail) {
        return "{\"type\":\"about:blank\","
                + "\"title\":\"" + title + "\","
                + "\"status\":" + status + ","
                + "\"detail\":\"" + detail + "\"}";
    }

    /**
     * Health response JSON.
     */
    public static String healthResponseJson() {
        return "{\"status\":\"ok\",\"version\":\"0.1.0\"}";
    }

    /**
     * DIDComm inbox response JSON.
     */
    public static String didcommInboxJson() {
        return "{\"messages\":[{\"id\":\"msg_1\","
                + "\"sender_did\":\"did:web:example:agent:sender\","
                + "\"message_type\":\"notification\","
                + "\"payload\":\"{\\\"hello\\\":\\\"world\\\"}\","
                + "\"created_at\":\"2025-01-01T00:00:00Z\"}],"
                + "\"count\":1}";
    }
}
