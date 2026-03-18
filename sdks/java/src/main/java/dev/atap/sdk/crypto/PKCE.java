package dev.atap.sdk.crypto;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;

/**
 * PKCE (Proof Key for Code Exchange) verifier and S256 challenge generation.
 */
public final class PKCE {

    private PKCE() {
    }

    /**
     * Result of PKCE generation containing verifier and challenge.
     */
    public static final class PKCEPair {
        private final String verifier;
        private final String challenge;

        public PKCEPair(String verifier, String challenge) {
            this.verifier = verifier;
            this.challenge = challenge;
        }

        public String getVerifier() {
            return verifier;
        }

        public String getChallenge() {
            return challenge;
        }
    }

    /**
     * Generate PKCE code verifier and S256 challenge.
     *
     * @return PKCEPair with verifier and challenge
     */
    public static PKCEPair generate() {
        byte[] randomBytes = new byte[32];
        new SecureRandom().nextBytes(randomBytes);
        String verifier = Base64Url.encode(randomBytes);

        try {
            MessageDigest sha256 = MessageDigest.getInstance("SHA-256");
            byte[] challengeBytes = sha256.digest(verifier.getBytes(StandardCharsets.UTF_8));
            String challenge = Base64Url.encode(challengeBytes);
            return new PKCEPair(verifier, challenge);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 not available", e);
        }
    }
}
