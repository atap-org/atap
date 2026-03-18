package dev.atap.sdk.crypto;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/**
 * JWK Thumbprint computation (RFC 7638) for Ed25519 public keys.
 */
public final class JWKThumbprint {

    private JWKThumbprint() {
    }

    /**
     * Compute the JWK thumbprint for an Ed25519 public key.
     *
     * @param publicKeyBytes the 32-byte Ed25519 public key
     * @return base64url-encoded SHA-256 thumbprint
     */
    public static String compute(byte[] publicKeyBytes) {
        String x = Base64Url.encode(publicKeyBytes);
        // Canonical JSON: sorted keys, no whitespace
        String canonical = "{\"crv\":\"Ed25519\",\"kty\":\"OKP\",\"x\":\"" + x + "\"}";
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(canonical.getBytes(StandardCharsets.UTF_8));
            return Base64Url.encode(hash);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 not available", e);
        }
    }
}
