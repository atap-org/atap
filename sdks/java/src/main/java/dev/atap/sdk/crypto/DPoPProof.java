package dev.atap.sdk.crypto;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.UUID;

/**
 * DPoP proof JWT generation (RFC 9449) for Ed25519.
 */
public final class DPoPProof {

    private DPoPProof() {
    }

    /**
     * Create a DPoP proof JWT.
     *
     * @param keyPair     Ed25519 keypair for signing
     * @param method      HTTP method (GET, POST, etc.)
     * @param url         Full URL for the htu claim (must use https://{platformDomain}/path)
     * @param accessToken if provided, includes ath (access token hash) claim
     * @return compact JWS string (header.payload.signature)
     */
    public static String create(Ed25519KeyPair keyPair, String method, String url, String accessToken) {
        String x = keyPair.getPublicKeyBase64Url();

        // Build header JSON
        String header = "{\"alg\":\"EdDSA\",\"jwk\":{\"crv\":\"Ed25519\",\"kty\":\"OKP\",\"x\":\""
                + x + "\"},\"typ\":\"dpop+jwt\"}";

        // Build payload JSON
        StringBuilder payload = new StringBuilder();
        payload.append("{");

        if (accessToken != null && !accessToken.isEmpty()) {
            try {
                MessageDigest sha256 = MessageDigest.getInstance("SHA-256");
                byte[] athBytes = sha256.digest(accessToken.getBytes(StandardCharsets.US_ASCII));
                payload.append("\"ath\":\"").append(Base64Url.encode(athBytes)).append("\",");
            } catch (NoSuchAlgorithmException e) {
                throw new RuntimeException("SHA-256 not available", e);
            }
        }

        long iat = System.currentTimeMillis() / 1000;
        String jti = UUID.randomUUID().toString();

        payload.append("\"htm\":\"").append(method).append("\",");
        payload.append("\"htu\":\"").append(url).append("\",");
        payload.append("\"iat\":").append(iat).append(",");
        payload.append("\"jti\":\"").append(jti).append("\"");
        payload.append("}");

        String headerB64 = Base64Url.encode(header.getBytes(StandardCharsets.UTF_8));
        String payloadB64 = Base64Url.encode(payload.toString().getBytes(StandardCharsets.UTF_8));

        byte[] signingInput = (headerB64 + "." + payloadB64).getBytes(StandardCharsets.US_ASCII);
        byte[] signature = keyPair.sign(signingInput);

        return headerB64 + "." + payloadB64 + "." + Base64Url.encode(signature);
    }

    /**
     * Create a DPoP proof JWT without access token hash.
     */
    public static String create(Ed25519KeyPair keyPair, String method, String url) {
        return create(keyPair, method, url, null);
    }
}
