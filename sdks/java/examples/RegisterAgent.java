import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.model.Entity;

/**
 * Example: Register an agent entity with the ATAP platform.
 */
public class RegisterAgent {

    public static void main(String[] args) {
        // 1. Generate a new Ed25519 keypair
        Ed25519KeyPair keyPair = Ed25519KeyPair.generate();
        System.out.println("Generated public key: " + keyPair.getPublicKeyBase64());

        // 2. Register the agent (server will assign a DID)
        try (ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .build()) {

            Entity entity = client.entities.register("agent", "My AI Agent",
                    keyPair.getPublicKeyBase64(), null);

            System.out.println("Registered entity:");
            System.out.println("  ID:            " + entity.getId());
            System.out.println("  DID:           " + entity.getDid());
            System.out.println("  Client Secret: " + entity.getClientSecret());

            // 3. Now create an authenticated client using the returned credentials
            ATAPClient authedClient = ATAPClient.builder()
                    .baseUrl("http://localhost:8080")
                    .did(entity.getDid())
                    .keyPair(keyPair)
                    .clientSecret(entity.getClientSecret())
                    .build();

            // 4. Use the authenticated client
            Entity self = authedClient.entities.get(entity.getId());
            System.out.println("  Trust Level:   " + self.getTrustLevel());

            authedClient.close();
        }
    }
}
