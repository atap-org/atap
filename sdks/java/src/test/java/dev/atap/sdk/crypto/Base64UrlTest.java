package dev.atap.sdk.crypto;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class Base64UrlTest {

    @Test
    void testEncodeNoPadding() {
        byte[] data = "Hello, World!".getBytes();
        String encoded = Base64Url.encode(data);
        assertFalse(encoded.contains("="));
        assertFalse(encoded.contains("+"));
        assertFalse(encoded.contains("/"));
    }

    @Test
    void testRoundTrip() {
        byte[] original = "test data for base64url".getBytes();
        String encoded = Base64Url.encode(original);
        byte[] decoded = Base64Url.decode(encoded);
        assertArrayEquals(original, decoded);
    }

    @Test
    void testRoundTripBinaryData() {
        byte[] original = new byte[]{0, 1, 2, (byte) 255, (byte) 254, (byte) 253, 127, (byte) 128};
        String encoded = Base64Url.encode(original);
        byte[] decoded = Base64Url.decode(encoded);
        assertArrayEquals(original, decoded);
    }

    @Test
    void testEmptyInput() {
        byte[] empty = new byte[0];
        String encoded = Base64Url.encode(empty);
        assertEquals("", encoded);
        byte[] decoded = Base64Url.decode(encoded);
        assertEquals(0, decoded.length);
    }

    @Test
    void testEncodeUrlSafeCharacters() {
        // Bytes that would produce + and / in standard base64
        byte[] data = new byte[]{(byte) 0xFB, (byte) 0xFF, (byte) 0xFE};
        String encoded = Base64Url.encode(data);
        assertFalse(encoded.contains("+"));
        assertFalse(encoded.contains("/"));
    }

    @Test
    void testDecodeWithPadding() {
        // Should handle input that already has padding
        String withPadding = "SGVsbG8=";
        byte[] decoded = Base64Url.decode(withPadding);
        assertEquals("Hello", new String(decoded));
    }
}
