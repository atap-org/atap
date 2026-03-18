/**
 * Example: Create and respond to approvals.
 *
 * Run with: npx tsx examples/approval-flow.ts
 */

import { ATAPClient } from "@atap/sdk";
import type { ApprovalSubject } from "@atap/sdk";

async function main() {
  // Create an authenticated client
  const client = new ATAPClient({
    baseUrl: "http://localhost:8080",
    did: "did:web:localhost%3A8080:agent:requester",
    privateKey: "<base64 Ed25519 seed>",
    clientSecret: "atap_...",
  });

  // Create an approval request
  const subject: ApprovalSubject = {
    type: "com.example.payment",
    label: "Payment of $100 to vendor",
    payload: {
      amount: 100,
      currency: "USD",
      recipient: "vendor@example.com",
    },
  };

  const approval = await client.approvals.create(
    "did:web:localhost%3A8080:agent:requester",
    "did:web:localhost%3A8080:human:approver",
    subject,
  );

  console.log("Created approval:");
  console.log(`  ID:    ${approval.id}`);
  console.log(`  State: ${approval.state}`);
  console.log(`  From:  ${approval.fromDid}`);
  console.log(`  To:    ${approval.toDid}`);

  // List pending approvals
  const approvals = await client.approvals.list();
  console.log(`\nPending approvals: ${approvals.length}`);

  // Respond to an approval (as the approver)
  const responded = await client.approvals.respond(approval.id, "<JWS signature>");
  console.log(`\nApproval ${responded.id} is now: ${responded.state}`);
}

main().catch(console.error);
