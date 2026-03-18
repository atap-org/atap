package dev.atap.sdk.model;

import dev.atap.sdk.exception.*;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests exercising model getters/setters and exception constructors.
 */
class ModelCoverageTest {

    @Test
    void testEntityAllFields() {
        Entity e = new Entity();
        e.setId("id1");
        e.setType("agent");
        e.setDid("did:web:example");
        e.setPrincipalDid("did:web:principal");
        e.setName("My Agent");
        e.setKeyId("key_1");
        e.setPublicKey("pubkey==");
        e.setTrustLevel(2);
        e.setRegistry("registry");
        e.setCreatedAt("2025-01-01T00:00:00Z");
        e.setUpdatedAt("2025-01-02T00:00:00Z");
        e.setClientSecret("atap_secret");
        e.setPrivateKey("privkey==");

        assertEquals("id1", e.getId());
        assertEquals("agent", e.getType());
        assertEquals("did:web:example", e.getDid());
        assertEquals("did:web:principal", e.getPrincipalDid());
        assertEquals("My Agent", e.getName());
        assertEquals("key_1", e.getKeyId());
        assertEquals("pubkey==", e.getPublicKey());
        assertEquals(2, e.getTrustLevel());
        assertEquals("registry", e.getRegistry());
        assertEquals("2025-01-01T00:00:00Z", e.getCreatedAt());
        assertEquals("2025-01-02T00:00:00Z", e.getUpdatedAt());
        assertEquals("atap_secret", e.getClientSecret());
        assertEquals("privkey==", e.getPrivateKey());
    }

    @Test
    void testApprovalAllFields() {
        Approval a = new Approval();
        a.setId("apr_1");
        a.setState("approved");
        a.setCreatedAt("2025-01-01T00:00:00Z");
        a.setValidUntil("2025-02-01T00:00:00Z");
        a.setFromDid("did:web:from");
        a.setToDid("did:web:to");
        a.setVia("did:web:via");
        a.setParent("apr_parent");
        a.setTemplateUrl("https://example.com/template");
        a.setSignatures(Collections.singletonMap("approver", "sig"));
        a.setRespondedAt("2025-01-01T01:00:00Z");
        a.setFanOut(3);

        ApprovalSubject subject = new ApprovalSubject();
        subject.setType("data_access");
        subject.setLabel("Access");
        subject.setReversible(true);
        subject.setPayload(Collections.singletonMap("key", "value"));
        a.setSubject(subject);

        assertEquals("apr_1", a.getId());
        assertEquals("approved", a.getState());
        assertEquals("2025-01-01T00:00:00Z", a.getCreatedAt());
        assertEquals("2025-02-01T00:00:00Z", a.getValidUntil());
        assertEquals("did:web:from", a.getFromDid());
        assertEquals("did:web:to", a.getToDid());
        assertEquals("did:web:via", a.getVia());
        assertEquals("apr_parent", a.getParent());
        assertEquals("https://example.com/template", a.getTemplateUrl());
        assertEquals("sig", a.getSignatures().get("approver"));
        assertEquals("2025-01-01T01:00:00Z", a.getRespondedAt());
        assertEquals(3, a.getFanOut());
        assertEquals("data_access", a.getSubject().getType());
        assertEquals("Access", a.getSubject().getLabel());
        assertTrue(a.getSubject().isReversible());
        assertEquals("value", a.getSubject().getPayload().get("key"));
    }

    @Test
    void testApprovalSubjectConstructor() {
        ApprovalSubject s = new ApprovalSubject("type1", "label1", false, Collections.emptyMap());
        assertEquals("type1", s.getType());
        assertEquals("label1", s.getLabel());
        assertFalse(s.isReversible());
        assertNotNull(s.getPayload());
    }

    @Test
    void testRevocationAllFields() {
        Revocation r = new Revocation();
        r.setId("rev_1");
        r.setApprovalId("apr_1");
        r.setApproverDid("did:web:approver");
        r.setRevokedAt("2025-01-01T00:00:00Z");
        r.setExpiresAt("2025-01-02T00:00:00Z");

        assertEquals("rev_1", r.getId());
        assertEquals("apr_1", r.getApprovalId());
        assertEquals("did:web:approver", r.getApproverDid());
        assertEquals("2025-01-01T00:00:00Z", r.getRevokedAt());
        assertEquals("2025-01-02T00:00:00Z", r.getExpiresAt());
    }

    @Test
    void testRevocationList() {
        RevocationList rl = new RevocationList();
        rl.setEntity("did:web:entity");
        rl.setCheckedAt("2025-01-01T00:00:00Z");
        rl.setRevocations(Collections.emptyList());

        assertEquals("did:web:entity", rl.getEntity());
        assertEquals("2025-01-01T00:00:00Z", rl.getCheckedAt());
        assertTrue(rl.getRevocations().isEmpty());
    }

    @Test
    void testCredentialAllFields() {
        Credential c = new Credential();
        c.setId("cred_1");
        c.setType("ATAPEmailVerification");
        c.setCredential("eyJhbGciOiJFZERTQSJ9...");
        c.setIssuedAt("2025-01-01T00:00:00Z");

        assertEquals("cred_1", c.getId());
        assertEquals("ATAPEmailVerification", c.getType());
        assertEquals("eyJhbGciOiJFZERTQSJ9...", c.getCredential());
        assertEquals("2025-01-01T00:00:00Z", c.getIssuedAt());

        c.setRevokedAt("2025-06-01T00:00:00Z");
        assertEquals("2025-06-01T00:00:00Z", c.getRevokedAt());
    }

    @Test
    void testDIDCommMessage() {
        DIDCommMessage msg = new DIDCommMessage();
        msg.setId("msg_1");
        msg.setSenderDid("did:web:sender");
        msg.setMessageType("notification");
        msg.setPayload("{\"hello\":\"world\"}");
        msg.setCreatedAt("2025-01-01T00:00:00Z");

        assertEquals("msg_1", msg.getId());
        assertEquals("did:web:sender", msg.getSenderDid());
        assertEquals("notification", msg.getMessageType());
        assertEquals("{\"hello\":\"world\"}", msg.getPayload());
        assertEquals("2025-01-01T00:00:00Z", msg.getCreatedAt());
    }

    @Test
    void testDIDCommInbox() {
        DIDCommInbox inbox = new DIDCommInbox();
        inbox.setMessages(Collections.emptyList());
        inbox.setCount(0);

        assertEquals(0, inbox.getCount());
        assertTrue(inbox.getMessages().isEmpty());
    }

    @Test
    void testDIDDocument() {
        DIDDocument doc = new DIDDocument();
        doc.setId("did:web:example");
        doc.setContext(Arrays.asList("https://www.w3.org/ns/did/v1"));
        doc.setAuthentication(Arrays.asList("did:web:example#key-1"));
        doc.setAssertionMethod(Arrays.asList("did:web:example#key-1"));
        doc.setKeyAgreement(Collections.emptyList());
        doc.setService(Collections.emptyList());
        doc.setAtapType("agent");
        doc.setAtapPrincipal("");

        VerificationMethod vm = new VerificationMethod();
        vm.setId("did:web:example#key-1");
        vm.setType("Ed25519VerificationKey2020");
        vm.setController("did:web:example");
        vm.setPublicKeyMultibase("z6Mktest");
        doc.setVerificationMethod(Arrays.asList(vm));

        assertEquals("did:web:example", doc.getId());
        assertEquals(1, doc.getContext().size());
        assertEquals(1, doc.getAuthentication().size());
        assertEquals(1, doc.getAssertionMethod().size());
        assertTrue(doc.getKeyAgreement().isEmpty());
        assertTrue(doc.getService().isEmpty());
        assertEquals("agent", doc.getAtapType());
        assertEquals("", doc.getAtapPrincipal());
        assertEquals(1, doc.getVerificationMethod().size());
        assertEquals("z6Mktest", doc.getVerificationMethod().get(0).getPublicKeyMultibase());
        assertEquals("did:web:example#key-1", doc.getVerificationMethod().get(0).getId());
        assertEquals("Ed25519VerificationKey2020", doc.getVerificationMethod().get(0).getType());
        assertEquals("did:web:example", doc.getVerificationMethod().get(0).getController());
    }

    @Test
    void testDiscoveryDocument() {
        DiscoveryDocument doc = new DiscoveryDocument();
        doc.setDomain("localhost");
        doc.setApiBase("http://localhost:8080");
        doc.setDidcommEndpoint("http://localhost:8080/v1/didcomm");
        doc.setClaimTypes(Arrays.asList("email", "phone"));
        doc.setMaxApprovalTtl("24h");
        doc.setTrustLevel(1);
        doc.setOauth(Collections.singletonMap("token_endpoint", "/v1/oauth/token"));

        assertEquals("localhost", doc.getDomain());
        assertEquals("http://localhost:8080", doc.getApiBase());
        assertEquals("http://localhost:8080/v1/didcomm", doc.getDidcommEndpoint());
        assertEquals(2, doc.getClaimTypes().size());
        assertEquals("24h", doc.getMaxApprovalTtl());
        assertEquals(1, doc.getTrustLevel());
        assertNotNull(doc.getOauth());
    }

    @Test
    void testKeyVersion() {
        KeyVersion kv = new KeyVersion();
        kv.setId("key_v1");
        kv.setEntityId("entity_1");
        kv.setKeyIndex(1);
        kv.setValidFrom("2025-01-01T00:00:00Z");
        kv.setCreatedAt("2025-01-01T00:00:00Z");

        assertEquals("key_v1", kv.getId());
        assertEquals("entity_1", kv.getEntityId());
        assertEquals(1, kv.getKeyIndex());
        assertEquals("2025-01-01T00:00:00Z", kv.getValidFrom());
        assertEquals("2025-01-01T00:00:00Z", kv.getCreatedAt());

        kv.setValidUntil("2025-12-31T23:59:59Z");
        assertEquals("2025-12-31T23:59:59Z", kv.getValidUntil());
    }

    @Test
    void testProblemDetail() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Not Found", 404, "Resource not found", "/v1/entities/abc");
        assertEquals("about:blank", pd.getType());
        assertEquals("Not Found", pd.getTitle());
        assertEquals(404, pd.getStatus());
        assertEquals("Resource not found", pd.getDetail());
        assertEquals("/v1/entities/abc", pd.getInstance());
    }

    @Test
    void testProblemDetailDefaultConstructor() {
        ProblemDetail pd = new ProblemDetail();
        assertNull(pd.getType());
        assertNull(pd.getTitle());
    }

    @Test
    void testOAuthToken() {
        dev.atap.sdk.oauth.OAuthToken token = new dev.atap.sdk.oauth.OAuthToken();
        assertEquals("DPoP", token.getTokenType());
        assertEquals(3600, token.getExpiresIn());

        token.setAccessToken("at");
        token.setTokenType("Bearer");
        token.setExpiresIn(7200);
        token.setScope("openid");
        token.setRefreshToken("rt");

        assertEquals("at", token.getAccessToken());
        assertEquals("Bearer", token.getTokenType());
        assertEquals(7200, token.getExpiresIn());
        assertEquals("openid", token.getScope());
        assertEquals("rt", token.getRefreshToken());
    }

    @Test
    void testATAPExceptionVariants() {
        ATAPException ex1 = new ATAPException("msg");
        assertEquals("msg", ex1.getMessage());
        assertEquals(0, ex1.getStatusCode());

        ATAPException ex2 = new ATAPException("msg2", 500);
        assertEquals(500, ex2.getStatusCode());

        RuntimeException cause = new RuntimeException("cause");
        ATAPException ex3 = new ATAPException("msg3", 502, cause);
        assertEquals(502, ex3.getStatusCode());
        assertEquals(cause, ex3.getCause());
    }

    @Test
    void testATAPProblemExceptionToString() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Server Error", 500, "Something broke", null);
        ATAPProblemException ex = new ATAPProblemException(pd);
        assertEquals(500, ex.getStatusCode());
        assertNotNull(ex.getProblem());
        String str = ex.toString();
        assertTrue(str.contains("500"));
        assertTrue(str.contains("Server Error"));
        assertTrue(str.contains("Something broke"));
    }

    @Test
    void testATAPProblemExceptionNoDetail() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Error", 400, null, null);
        ATAPProblemException ex = new ATAPProblemException(pd);
        assertEquals("Error", ex.getMessage());
        String str = ex.toString();
        assertTrue(str.contains("400"));
    }

    @Test
    void testATAPAuthException() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Unauthorized", 401, "Bad token", null);
        ATAPAuthException ex = new ATAPAuthException("Bad token", 401, pd);
        assertEquals(401, ex.getStatusCode());
        assertEquals(pd, ex.getProblem());
    }

    @Test
    void testATAPNotFoundException() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Not Found", 404, "Missing", null);
        ATAPNotFoundException ex = new ATAPNotFoundException("Missing", pd);
        assertEquals(404, ex.getStatusCode());
        assertEquals(pd, ex.getProblem());
    }

    @Test
    void testATAPConflictException() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Conflict", 409, "Exists", null);
        ATAPConflictException ex = new ATAPConflictException("Exists", pd);
        assertEquals(409, ex.getStatusCode());
    }

    @Test
    void testATAPRateLimitException() {
        ProblemDetail pd = new ProblemDetail("about:blank", "Too Many", 429, "Slow down", null);
        ATAPRateLimitException ex = new ATAPRateLimitException("Slow down", pd);
        assertEquals(429, ex.getStatusCode());
    }
}
