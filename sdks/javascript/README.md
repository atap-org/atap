# ATAP JavaScript/TypeScript SDK

TypeScript client for the [ATAP](https://atap.dev) (Agent Trust and Authority Protocol) platform.

## Installation

```bash
npm install @atap/sdk
```

## Quick Start

```typescript
import { ATAPClient } from "@atap/sdk";

// Register a new agent
const client = new ATAPClient({ baseUrl: "http://localhost:8080" });
const entity = await client.entities.register("agent", { name: "my-agent" });

// Create an authenticated client
const authed = new ATAPClient({
  baseUrl: "http://localhost:8080",
  did: entity.did,
  privateKey: entity.privateKey!,
  clientSecret: entity.clientSecret!,
});

// List approvals
const approvals = await authed.approvals.list();

// Check DIDComm inbox
const inbox = await authed.didcomm.inbox();
```

## Authentication

The SDK handles OAuth 2.1 + DPoP authentication automatically.

**Agent/Machine** (client_credentials grant):
```typescript
const client = new ATAPClient({
  baseUrl: "http://localhost:8080",
  did: "did:web:localhost%3A8080:agent:abc",
  privateKey: "<base64 Ed25519 seed>",
  clientSecret: "atap_...",
});
// Tokens are obtained and refreshed automatically
```

**Human/Org** (authorization_code + PKCE):
```typescript
const client = new ATAPClient({
  baseUrl: "http://localhost:8080",
  did: "did:web:localhost%3A8080:human:abc",
  privateKey: "<base64 Ed25519 seed>",
});
await client.tokenManager.obtainAuthorizationCode();
```

## API Reference

### Entities

```typescript
// Register
const entity = await client.entities.register("agent", { name: "my-agent" });
const entity = await client.entities.register("human", { publicKey: "<base64 pubkey>" });

// Get
const entity = await client.entities.get("entity_id");

// Delete (requires atap:manage)
await client.entities.delete("entity_id");

// Rotate key (requires atap:manage)
const kv = await client.entities.rotateKey("entity_id", "<base64 new pubkey>");
```

### Approvals

```typescript
import type { ApprovalSubject } from "@atap/sdk";

// Create (requires atap:send)
const approval = await client.approvals.create(
  "did:web:...:agent:requester",
  "did:web:...:human:approver",
  {
    type: "com.example.payment",
    label: "Payment of $100",
    payload: { amount: 100 },
  } satisfies ApprovalSubject,
);

// Respond (requires atap:send)
const result = await client.approvals.respond("apr_...", "<JWS>");

// List (requires atap:inbox)
const approvals = await client.approvals.list();

// Revoke (requires atap:revoke)
await client.approvals.revoke("apr_...");
```

### Revocations

```typescript
// Submit (requires atap:revoke)
const rev = await client.revocations.submit("apr_...", "<JWS>");

// List (public)
const result = await client.revocations.list("did:web:...:agent:abc");
```

### DIDComm

```typescript
// Send message (public)
await client.didcomm.send(jweBytes);

// Check inbox (requires atap:inbox)
const inbox = await client.didcomm.inbox({ limit: 50 });
for (const msg of inbox.messages) {
  console.log(`From: ${msg.senderDid}, Type: ${msg.messageType}`);
}
```

### Credentials

```typescript
// Email verification (requires atap:manage)
await client.credentials.startEmailVerification("user@example.com");
const cred = await client.credentials.verifyEmail("user@example.com", "123456");

// Phone verification (requires atap:manage)
await client.credentials.startPhoneVerification("+1234567890");
const cred = await client.credentials.verifyPhone("+1234567890", "654321");

// Personhood (requires atap:manage)
const cred = await client.credentials.submitPersonhood();

// List credentials (requires atap:manage)
const creds = await client.credentials.list();

// Status list (public)
const status = await client.credentials.statusList("1");
```

### Discovery

```typescript
// Server discovery
const doc = await client.discovery.discover();

// DID resolution
const didDoc = await client.discovery.resolveDid("agent", "entity_id");

// Server DID
const serverDid = await client.discovery.serverDid();

// Health check
const health = await client.discovery.health();
```

## Error Handling

```typescript
import {
  ATAPError,
  ATAPProblemError,
  ATAPAuthError,
  ATAPNotFoundError,
  ATAPRateLimitError,
} from "@atap/sdk";

try {
  const entity = await client.entities.get("missing");
} catch (e) {
  if (e instanceof ATAPNotFoundError) {
    console.log(`Not found: ${e.message}`);
  } else if (e instanceof ATAPAuthError) {
    console.log(`Auth error (${e.statusCode}): ${e.message}`);
  } else if (e instanceof ATAPRateLimitError) {
    console.log(`Rate limited: ${e.message}`);
  } else if (e instanceof ATAPProblemError) {
    console.log(`API error: ${e.problem.title} - ${e.problem.detail}`);
  } else if (e instanceof ATAPError) {
    console.log(`Error: ${e.message}`);
  }
}
```

## Configuration

```typescript
const client = new ATAPClient({
  baseUrl: "http://localhost:8080",       // HTTP target
  did: "did:web:...",                     // Entity DID
  privateKey: "<base64>",                 // Ed25519 private key
  clientSecret: "atap_...",              // For agent/machine auth
  scopes: ["atap:inbox", "atap:send"],   // OAuth scopes
  platformDomain: "api.atap.app",        // Domain for DPoP htu
  timeout: 30000,                        // Request timeout (ms)
});
```

## Crypto Utilities

The SDK exports low-level cryptographic functions:

```typescript
import {
  generateKeypair,
  loadSigningKey,
  makeDPoPProof,
  jwkThumbprint,
  generatePKCE,
  b64urlEncode,
  b64urlDecode,
  domainFromDID,
} from "@atap/sdk";

// Generate Ed25519 keypair
const { privateKey, publicKey } = await generateKeypair();

// Create DPoP proof
const proof = await makeDPoPProof(privateKey, "POST", "https://api.atap.app/v1/oauth/token");

// Generate PKCE challenge
const { verifier, challenge } = generatePKCE();
```

## Examples

See the [examples/](examples/) directory:
- [register-agent.ts](examples/register-agent.ts) -- Register and authenticate
- [approval-flow.ts](examples/approval-flow.ts) -- Create and respond to approvals
- [credential-verification.ts](examples/credential-verification.ts) -- Email verification

## Development

```bash
npm install
npm test
npm run test:coverage
npm run build
npm run lint
```

## License

Apache-2.0
