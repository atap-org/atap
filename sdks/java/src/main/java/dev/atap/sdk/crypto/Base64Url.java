package dev.atap.sdk.crypto;

import java.util.Base64;

/**
 * Base64url encoding/decoding without padding (RFC 4648 Section 5).
 */
public final class Base64Url {

    private Base64Url() {
    }

    /**
     * Base64url encode without padding.
     */
    public static String encode(byte[] data) {
        return Base64.getUrlEncoder().withoutPadding().encodeToString(data);
    }

    /**
     * Base64url decode with padding restoration.
     */
    public static byte[] decode(String s) {
        // Add padding if needed
        int padding = 4 - s.length() % 4;
        if (padding != 4) {
            StringBuilder sb = new StringBuilder(s);
            for (int i = 0; i < padding; i++) {
                sb.append('=');
            }
            s = sb.toString();
        }
        return Base64.getUrlDecoder().decode(s);
    }
}
