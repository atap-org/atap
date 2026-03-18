package dev.atap.sdk;

/**
 * Utility methods for DID and domain operations.
 */
public final class DomainUtils {

    private DomainUtils() {
    }

    /**
     * Extract platform domain from a DID.
     * <p>
     * Example: did:web:localhost%3A8080:agent:abc -> localhost:8080
     *
     * @param did the DID string
     * @return the extracted domain
     * @throws IllegalArgumentException if the DID format is invalid
     */
    public static String domainFromDid(String did) {
        if (did == null || did.isEmpty()) {
            throw new IllegalArgumentException("DID cannot be null or empty");
        }
        String[] parts = did.split(":");
        if (parts.length < 3) {
            throw new IllegalArgumentException("Invalid DID format: " + did);
        }
        return parts[2].replace("%3A", ":");
    }
}
