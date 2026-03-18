import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Approval;
import dev.atap.sdk.model.ApprovalSubject;

import java.util.Collections;
import java.util.List;

/**
 * Example: Create and respond to an approval request.
 */
public class ApprovalFlow {

    public static void main(String[] args) {
        // Create an authenticated client
        try (ATAPClient client = ATAPClient.builder()
                .baseUrl("http://localhost:8080")
                .did("did:web:localhost%3A8080:agent:requester")
                .privateKey("<base64-encoded-private-key>")
                .clientSecret("atap_<your-secret>")
                .build()) {

            // 1. Create an approval request
            ApprovalSubject subject = new ApprovalSubject(
                    "data_access",
                    "Access user profile data",
                    true, // reversible
                    Collections.singletonMap("resource", "user_profiles")
            );

            Approval approval = client.approvals.create(
                    "did:web:localhost%3A8080:agent:requester",
                    "did:web:localhost%3A8080:human:approver",
                    subject
            );

            System.out.println("Created approval: " + approval.getId());
            System.out.println("State: " + approval.getState());

            // 2. List pending approvals
            List<Approval> pending = client.approvals.list();
            System.out.println("Pending approvals: " + pending.size());

            // 3. Respond to an approval (as the approver)
            // Approval responded = client.approvals.respond(approval.getId(), "jws_signature");

            // 4. Revoke an approval
            // Approval revoked = client.approvals.revoke(approval.getId());
        }
    }
}
