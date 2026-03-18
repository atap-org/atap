/**
 * Example: Email verification and credential management.
 *
 * Run with: npx tsx examples/credential-verification.ts
 */

import { ATAPClient } from "@atap/sdk";

async function main() {
  // Create an authenticated client
  const client = new ATAPClient({
    baseUrl: "http://localhost:8080",
    did: "did:web:localhost%3A8080:human:user",
    privateKey: "<base64 Ed25519 seed>",
    clientSecret: "atap_...",
  });

  // Start email verification
  const msg = await client.credentials.startEmailVerification(
    "user@example.com",
  );
  console.log(`Email verification: ${msg}`);

  // Verify with OTP (user enters code from email)
  const credential = await client.credentials.verifyEmail(
    "user@example.com",
    "123456",
  );
  console.log("Email verified!");
  console.log(`  Credential ID:   ${credential.id}`);
  console.log(`  Type:            ${credential.type}`);
  console.log(`  Issued at:       ${credential.issuedAt}`);

  // List all credentials
  const credentials = await client.credentials.list();
  console.log(`\nTotal credentials: ${credentials.length}`);
  for (const cred of credentials) {
    console.log(`  - ${cred.type} (${cred.id})`);
  }

  // Check credential status list (public endpoint)
  const status = await client.credentials.statusList("1");
  console.log(`\nStatus list: ${JSON.stringify(status)}`);
}

main().catch(console.error);
