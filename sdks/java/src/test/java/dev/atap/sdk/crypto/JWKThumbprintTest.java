package dev.atap.sdk.crypto;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class JWKThumbprintTest {

    @Test
    void testComputeThumbprint() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String thumbprint = JWKThumbprint.compute(kp.getPublicKeyBytes());
        assertNotNull(thumbprint);
        assertFalse(thumbprint.isEmpty());
        assertFalse(thumbprint.contains("="));
        assertFalse(thumbprint.contains("+"));
        assertFalse(thumbprint.contains("/"));
    }

    @Test
    void testSameKeyProducesSameThumbprint() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String t1 = JWKThumbprint.compute(kp.getPublicKeyBytes());
        String t2 = JWKThumbprint.compute(kp.getPublicKeyBytes());
        assertEquals(t1, t2);
    }

    @Test
    void testDifferentKeysProduceDifferentThumbprints() {
        Ed25519KeyPair kp1 = Ed25519KeyPair.generate();
        Ed25519KeyPair kp2 = Ed25519KeyPair.generate();
        String t1 = JWKThumbprint.compute(kp1.getPublicKeyBytes());
        String t2 = JWKThumbprint.compute(kp2.getPublicKeyBytes());
        assertNotEquals(t1, t2);
    }

    @Test
    void testThumbprintLengthIs43Chars() {
        // SHA-256 = 32 bytes -> base64url = 43 characters (no padding)
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String thumbprint = JWKThumbprint.compute(kp.getPublicKeyBytes());
        assertEquals(43, thumbprint.length());
    }
}
