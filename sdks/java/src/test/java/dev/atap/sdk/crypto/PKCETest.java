package dev.atap.sdk.crypto;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class PKCETest {

    @Test
    void testGenerate() {
        PKCE.PKCEPair pair = PKCE.generate();
        assertNotNull(pair);
        assertNotNull(pair.getVerifier());
        assertNotNull(pair.getChallenge());
        assertFalse(pair.getVerifier().isEmpty());
        assertFalse(pair.getChallenge().isEmpty());
    }

    @Test
    void testVerifierAndChallengeAreDifferent() {
        PKCE.PKCEPair pair = PKCE.generate();
        assertNotEquals(pair.getVerifier(), pair.getChallenge());
    }

    @Test
    void testEachGenerationIsUnique() {
        PKCE.PKCEPair pair1 = PKCE.generate();
        PKCE.PKCEPair pair2 = PKCE.generate();
        assertNotEquals(pair1.getVerifier(), pair2.getVerifier());
        assertNotEquals(pair1.getChallenge(), pair2.getChallenge());
    }

    @Test
    void testChallengeIsSha256OfVerifier() throws Exception {
        PKCE.PKCEPair pair = PKCE.generate();
        java.security.MessageDigest sha256 = java.security.MessageDigest.getInstance("SHA-256");
        byte[] hash = sha256.digest(pair.getVerifier().getBytes(java.nio.charset.StandardCharsets.UTF_8));
        String expected = Base64Url.encode(hash);
        assertEquals(expected, pair.getChallenge());
    }
}
