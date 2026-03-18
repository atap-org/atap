package dev.atap.sdk;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class DomainUtilsTest {

    @Test
    void testDomainFromDid() {
        assertEquals("localhost:8080",
                DomainUtils.domainFromDid("did:web:localhost%3A8080:agent:abc"));
    }

    @Test
    void testDomainFromDidWithoutPort() {
        assertEquals("example.com",
                DomainUtils.domainFromDid("did:web:example.com:agent:abc"));
    }

    @Test
    void testDomainFromDidMinimal() {
        assertEquals("example.com",
                DomainUtils.domainFromDid("did:web:example.com"));
    }

    @Test
    void testDomainFromDidNull() {
        assertThrows(IllegalArgumentException.class, () -> DomainUtils.domainFromDid(null));
    }

    @Test
    void testDomainFromDidEmpty() {
        assertThrows(IllegalArgumentException.class, () -> DomainUtils.domainFromDid(""));
    }

    @Test
    void testDomainFromDidInvalid() {
        assertThrows(IllegalArgumentException.class, () -> DomainUtils.domainFromDid("invalid"));
    }

    @Test
    void testDomainFromDidTwoParts() {
        assertThrows(IllegalArgumentException.class, () -> DomainUtils.domainFromDid("did:web"));
    }
}
