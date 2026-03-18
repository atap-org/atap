import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Credential;

import java.util.List;

/**
 * Example: Email verification and credential management.
 */
public class CredentialVerification {

    public static void main(String[] args) {
        try (ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .did("did:web:localhost%3A8080:human:user123")
                .privateKey("<base64-encoded-private-key>")
                .clientSecret("atap_<your-secret>")
                .build()) {

            // 1. Start email verification
            String message = client.credentials.startEmail("user@example.com");
            System.out.println(message); // "OTP sent to user@example.com"

            // 2. Verify with OTP (received via email)
            Credential emailCred = client.credentials.verifyEmail("user@example.com", "123456");
            System.out.println("Email credential: " + emailCred.getId());
            System.out.println("Type: " + emailCred.getType());
            System.out.println("VC JWT: " + emailCred.getCredential());

            // 3. Start phone verification
            client.credentials.startPhone("+15551234567");

            // 4. Verify with OTP
            Credential phoneCred = client.credentials.verifyPhone("+15551234567", "654321");
            System.out.println("Phone credential: " + phoneCred.getId());

            // 5. Submit personhood attestation
            Credential personhood = client.credentials.submitPersonhood();
            System.out.println("Personhood credential: " + personhood.getId());

            // 6. List all credentials
            List<Credential> allCreds = client.credentials.list();
            System.out.println("Total credentials: " + allCreds.size());
        }
    }
}
