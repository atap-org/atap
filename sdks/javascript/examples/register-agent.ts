/**
 * Example: Register an agent and make authenticated requests.
 *
 * Run with: npx tsx examples/register-agent.ts
 */

import { ATAPClient } from "@atap/sdk";

async function main() {
  // Create an unauthenticated client for registration
  const client = new ATAPClient({
    baseUrl: "http://localhost:8080",
  });

  // Register a new agent (server generates keypair)
  const entity = await client.entities.register("agent", {
    name: "my-agent",
  });

  console.log("Registered agent:");
  console.log(`  ID:     ${entity.id}`);
  console.log(`  DID:    ${entity.did}`);
  console.log(`  Name:   ${entity.name}`);
  console.log(`  Secret: ${entity.clientSecret}`);

  // Create an authenticated client using the returned credentials
  const authed = new ATAPClient({
    baseUrl: "http://localhost:8080",
    did: entity.did,
    privateKey: entity.privateKey!,
    clientSecret: entity.clientSecret!,
  });

  // Check DIDComm inbox
  const inbox = await authed.didcomm.inbox();
  console.log(`\nInbox has ${inbox.count} messages`);

  // Check server health
  const health = await authed.discovery.health();
  console.log(`\nServer health: ${JSON.stringify(health)}`);
}

main().catch(console.error);
