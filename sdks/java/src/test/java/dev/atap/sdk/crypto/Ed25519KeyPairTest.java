package dev.atap.sdk.crypto;

import org.junit.jupiter.api.Test;

import java.util.Base64;

import static org.junit.jupiter.api.Assertions.*;

class Ed25519KeyPairTest {

    @Test
    void testGenerateKeypair() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        assertNotNull(kp);
        assertNotNull(kp.getPrivateKey());
        assertNotNull(kp.getPublicKey());
        assertEquals(32, kp.getPublicKeyBytes().length);
    }

    @Test
    void testLoadSigningKey32Bytes() {
        Ed25519KeyPair original = Ed25519KeyPair.generate();
        byte[] seed = original.getPrivateKey().getEncoded();
        assertEquals(32, seed.length);

        String b64 = Base64.getEncoder().encodeToString(seed);
        Ed25519KeyPair loaded = Ed25519KeyPair.loadSigningKey(b64);

        assertArrayEquals(original.getPublicKeyBytes(), loaded.getPublicKeyBytes());
    }

    @Test
    void testLoadSigningKey64Bytes() {
        Ed25519KeyPair original = Ed25519KeyPair.generate();
        byte[] seed = original.getPrivateKey().getEncoded();
        byte[] pub = original.getPublicKeyBytes();
        byte[] full = new byte[64];
        System.arraycopy(seed, 0, full, 0, 32);
        System.arraycopy(pub, 0, full, 32, 32);

        String b64 = Base64.getEncoder().encodeToString(full);
        Ed25519KeyPair loaded = Ed25519KeyPair.loadSigningKey(b64);

        assertArrayEquals(original.getPublicKeyBytes(), loaded.getPublicKeyBytes());
    }

    @Test
    void testLoadSigningKeyInvalidLength() {
        byte[] invalid = new byte[16];
        String b64 = Base64.getEncoder().encodeToString(invalid);

        assertThrows(IllegalArgumentException.class, () -> Ed25519KeyPair.loadSigningKey(b64));
    }

    @Test
    void testSignAndVerify() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        byte[] data = "hello world".getBytes();

        byte[] signature = kp.sign(data);
        assertNotNull(signature);
        assertEquals(64, signature.length);

        assertTrue(kp.verify(data, signature));
    }

    @Test
    void testSignatureVerificationFails() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        byte[] data = "hello world".getBytes();
        byte[] signature = kp.sign(data);

        byte[] tamperedData = "hello WORLD".getBytes();
        assertFalse(kp.verify(tamperedData, signature));
    }

    @Test
    void testPublicKeyBase64() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String b64 = kp.getPublicKeyBase64();
        assertNotNull(b64);
        assertFalse(b64.isEmpty());

        byte[] decoded = Base64.getDecoder().decode(b64);
        assertEquals(32, decoded.length);
    }

    @Test
    void testPublicKeyBase64Url() {
        Ed25519KeyPair kp = Ed25519KeyPair.generate();
        String b64url = kp.getPublicKeyBase64Url();
        assertNotNull(b64url);
        assertFalse(b64url.contains("="));
        assertFalse(b64url.contains("+"));
        assertFalse(b64url.contains("/"));
    }

    @Test
    void testDifferentKeypairsProduceDifferentSignatures() {
        Ed25519KeyPair kp1 = Ed25519KeyPair.generate();
        Ed25519KeyPair kp2 = Ed25519KeyPair.generate();
        byte[] data = "test data".getBytes();

        byte[] sig1 = kp1.sign(data);
        byte[] sig2 = kp2.sign(data);

        assertFalse(java.util.Arrays.equals(sig1, sig2));
    }
}
