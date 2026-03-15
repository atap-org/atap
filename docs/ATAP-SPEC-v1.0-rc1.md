# ATAP: Agent Trust and Authority Protocol

## Specification v1.0-rc1

**Status:** Release Candidate  
**Date:** March 2026  
**Authors:** ATAP Contributors  
**License:** Apache 2.0

---

## 1. Abstract

The Agent Trust and Authority Protocol (ATAP) defines a multi-signature approval model for verifiable multi-party authorization in AI agent ecosystems. When approval is needed, the requesting entity and — if a mediating system is involved — the system each sign an approval document, which is then sent to the approving entity for a final signature. The result is a self-contained, non-repudiable proof that all parties consented.

ATAP is a focused extension layer built on established standards:

| Layer | Standard | Role |
|-------|----------|------|
| Identity | W3C DIDs (`did:web`) | Entity addressing and key discovery |
| Claims | W3C Verifiable Credentials 2.0 (VC-JOSE-COSE) | Verified properties and relationships |
| Messaging | DIDComm v2.1 | Signal transport and encryption |
| Authorization | OAuth 2.1 + DPoP (RFC 9449) | API authentication and token management |
| Server Trust | WebPKI + DNSSEC + VC attestations | Server authentication and trust assessment |
| Signatures | JWS (RFC 7515) with JCS (RFC 8785) | Approval signatures |
| Templates | Microsoft Adaptive Cards | Approval rendering on devices |

ATAP adds one thing these standards do not provide: a **multi-signature approval** where each party signs independently, producing a portable proof of consent that can be verified by anyone, offline, without callback to an authorization server.

---

## 2. Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

---

## 3. Design Principles

### 3.1 Build on Standards

ATAP does not reinvent identity, messaging, claims, authorization, or trust infrastructure. Each layer uses an established, formally analyzed, widely implemented standard. ATAP defines only the approval model and the conventions for applying these standards in agent ecosystems.

### 3.2 One Novel Contribution

The multi-signature approval — independently verifiable consent from all parties in a single document — is ATAP's sole protocol contribution. Everything else is composition.

### 3.3 System Authority

ATAP does not determine which actions require approval. The mediating system decides. ATAP provides the approval mechanism, not the approval policy.

### 3.4 Opaque Payloads

Each system defines its own payload structure within the approval's `subject.payload` field. ATAP signs and transports it. The system enforces it. The template renders it.

### 3.5 Everything Is a Credential

All verifiable properties — email verification, personhood proof, agent-to-principal binding, organizational membership — are expressed as W3C Verifiable Credentials.

### 3.6 Server Trust

ATAP servers are trusted participants. They co-sign approvals, host DID Documents, and mediate messaging. This trust is explicit and consistent throughout the protocol. Key management, identity lifecycle, and message routing are server-mediated operations. Deployments requiring server-independent key authority SHOULD use `did:webs` with KERI witnesses as an alternative DID method.

---

## 4. Terminology

| Term | Definition |
|------|-----------|
| **Entity** | A participant: agent, machine, human, or organization. Identified by a DID. |
| **Agent** | An ephemeral software actor performing tasks on behalf of other entities. |
| **Machine** | A persistent application or service. |
| **Human** | A natural person. |
| **Organization** | A legal or organizational entity. |
| **Approval** | ATAP's core document: a multi-signature authorization proof. |
| **Subject** | The content of an approval: type, label, reversibility, system-specific payload. |
| **Credential** | A W3C Verifiable Credential expressing a verified property or relationship. |
| **Template** | A signed Adaptive Card defining how an approval is rendered. |
| **ATAP Server** | A service hosting entity identities and mediating communication. Serves two logically distinct roles: DIDComm mediator (untrusted relay) and ATAP system participant (trusted co-signer). Does not store approvals — stores only entity records, credentials, and revocation lists. |

---

## 5. Identity: DIDs

### 5.1 Entity Addressing

Every ATAP entity is identified by a `did:web` DID (W3C DID Core). The ATAP entity type is encoded as a DID Document property, not in the DID itself.

Format:

```
did:web:{server}:{type}:{id}
```

Examples:

```
did:web:provider.example:human:x7k9m2w4p3n8j5q2
did:web:provider.example:agent:01jd7xk3m9
did:web:merchant.example:machine:checkout
did:web:corp.example:org:engineering
```

The `{type}` path component (`human`, `agent`, `machine`, `org`) enables human-readable DIDs while the canonical type declaration lives in the DID Document.

### 5.2 DID Resolution

Resolution follows the `did:web` specification:

```
did:web:provider.example:agent:01jd7xk3m9
  → GET https://provider.example/agent/01jd7xk3m9/did.json
```

Resolution MUST use HTTPS. The server MUST present a valid TLS certificate. DNSSEC requirements are defined in Section 9.

Implementations SHOULD cache resolved DID Documents with a TTL of no more than 1 hour. Implementations MUST re-resolve before relying on cached key material for high-value approvals (system-defined threshold).

Note: DID resolution uses the standard `did:web` HTTPS path. The ATAP API base URL declared in `/.well-known/atap.json` is a separate endpoint used for ATAP-specific operations (approvals, credentials, DIDComm). These are two distinct URL spaces:

- `https://provider.example/{type}/{id}/did.json` — DID resolution (W3C standard)
- `https://api.provider.example/v1/...` — ATAP API (protocol-specific)

### 5.3 DID Document

```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1",
    "https://atap.dev/ns/v1"
  ],
  "id": "did:web:provider.example:agent:01jd7xk3m9",
  "authentication": [{
    "id": "did:web:provider.example:agent:01jd7xk3m9#key-1",
    "type": "Ed25519VerificationKey2020",
    "controller": "did:web:provider.example:agent:01jd7xk3m9",
    "publicKeyMultibase": "z6Mk..."
  }],
  "assertionMethod": [{
    "id": "did:web:provider.example:agent:01jd7xk3m9#key-1"
  }],
  "service": [{
    "id": "did:web:provider.example:agent:01jd7xk3m9#didcomm",
    "type": "DIDCommMessaging",
    "serviceEndpoint": "https://api.provider.example/v1/didcomm"
  }],
  "atap:type": "agent",
  "atap:principal": "did:web:provider.example:human:x7k9m2w4p3n8j5q2"
}
```

### 5.4 ATAP DID Document Properties

ATAP defines the following properties in the `https://atap.dev/ns/v1` JSON-LD context:

| Property | Values | Required | Description |
|----------|--------|----------|-------------|
| `atap:type` | `human`, `agent`, `machine`, `org` | REQUIRED | Entity type. |
| `atap:principal` | DID | OPTIONAL | The entity this entity acts on behalf of. REQUIRED for agents. |

These properties are the sole ATAP-specific additions to the DID Document. All other fields follow W3C DID Core.

### 5.5 Entity Types

| Type | Lifecycle | Description |
|------|-----------|-------------|
| `human` | Persistent | Natural person. ID derived from public key: `lowercase(base32(sha256(public_key))[0:16])`. |
| `agent` | Ephemeral | Software actor. DID Document MUST include `atap:principal`. |
| `machine` | Persistent | Long-running service. |
| `org` | Persistent | Legal entity. Signals routed to delegates (see Section 7.5). |

### 5.6 Key Management

Key material is managed within the DID Document per W3C DID Core. Key rotation is performed by updating the DID Document on the hosting server.

ATAP servers are the root of trust for key management. This is consistent with the server's role as co-signer (`via`) on approvals, host of DID Documents, and DIDComm mediator. The server MUST retain previous key versions with their validity periods for historical signature verification.

For human entities, the private key MUST be stored in the device's secure enclave. Key recovery uses passphrase-encrypted backup with Argon2id (RFC 9106, ≥256 MiB memory, ≥3 iterations) and minimum passphrase entropy of 77 bits (≥6 Diceware words).

Deployments requiring key management independent of the hosting server SHOULD use `did:webs`, which provides KERI-based key event integrity with witness verification.

### 5.7 Crypto-Shredding

To comply with data erasure requirements (e.g., GDPR Art. 17), all Verifiable Credential content containing or referencing personal information MUST be encrypted at rest with a per-entity encryption key managed by the hosting server.

Erasure is performed by deleting the per-entity encryption key. All credential data encrypted with that key becomes cryptographically unrecoverable, regardless of whether copies exist on federated servers.

Upon crypto-shredding, the server MUST:

1. Delete the per-entity encryption key.
2. Deactivate the entity's DID Document (per `did:web` deactivation).
3. Send a DIDComm notification (`https://atap.dev/protocols/entity/1.0/shredded`) to known federation partners.

Federation partners that receive this notification SHOULD delete cached credential data for that entity.

---

## 6. Claims: Verifiable Credentials

### 6.1 Overview

All verified properties of an entity are expressed as W3C Verifiable Credentials 2.0. ATAP uses the VC-JOSE-COSE proof format (JWT and SD-JWT) to avoid JSON-LD processing overhead while maintaining full VC interoperability.

### 6.2 ATAP Credential Types

ATAP defines a JSON-LD vocabulary at `https://atap.dev/ns/v1`:

| Credential Type | Proves | Issued By |
|----------------|--------|-----------|
| `ATAPEmailVerification` | Entity controls an email address | ATAP server |
| `ATAPPhoneVerification` | Entity controls a phone number | ATAP server |
| `ATAPPersonhood` | Entity is a unique human | Personhood provider |
| `ATAPIdentity` | Entity is a specific identified person | Government / eIDAS provider |
| `ATAPPrincipal` | Entity acts on behalf of another entity | ATAP server (co-signed) |
| `ATAPOrgMembership` | Entity belongs to an organization | Organization's ATAP server |

`ATAPPersonhood` credentials MUST use privacy-preserving verification. On-device biometric matching with zero-knowledge proof of match is REQUIRED where biometrics are involved. Raw biometric data MUST NOT be transmitted, stored off-device, or included in the credential. This ensures compliance with GDPR Art. 9 (special category data) and the EU AI Act Art. 50 transparency obligations.

### 6.3 Credential Format

Example: an agent's principal credential.

```json
{
  "@context": [
    "https://www.w3.org/ns/credentials/v2",
    "https://atap.dev/ns/v1"
  ],
  "type": ["VerifiableCredential", "ATAPPrincipal"],
  "issuer": "did:web:provider.example:machine:platform",
  "validFrom": "2026-03-12T14:30:00Z",
  "credentialSubject": {
    "id": "did:web:provider.example:agent:01jd7xk3m9",
    "principal": "did:web:provider.example:human:x7k9m2w4p3n8j5q2"
  },
  "credentialStatus": {
    "id": "https://provider.example/credentials/status/1#42",
    "type": "BitstringStatusListEntry",
    "statusPurpose": "revocation",
    "statusListIndex": "42",
    "statusListCredential": "https://provider.example/credentials/status/1"
  },
  "proof": {
    "type": "JsonWebSignature2020",
    "created": "2026-03-12T14:30:00Z",
    "verificationMethod": "did:web:provider.example:machine:platform#key-1",
    "proofPurpose": "assertionMethod",
    "jws": "<JWS>"
  }
}
```

### 6.4 Selective Disclosure

Credentials containing personal information SHOULD use SD-JWT (RFC 9901) for selective disclosure, in compliance with GDPR Art. 5(1)(c) data minimization.

### 6.5 Credential Revocation

Credential revocation uses W3C Bitstring Status List v1.0. Each ATAP server MUST publish a status list endpoint. Verifiers MUST check the `credentialStatus` field before accepting a credential.

### 6.6 Trust Level Derivation

An entity's trust level is derived from its credentials:

| Level | Required Credential |
|-------|-------------------|
| 0 | No credentials |
| 1 | `ATAPEmailVerification` or `ATAPPhoneVerification` |
| 2 | `ATAPPersonhood` |
| 3 | `ATAPIdentity` |

Effective trust: `min(trust_level, server_trust)`. Server trust is defined in Section 9.

---

## 7. Messaging: DIDComm

### 7.1 Overview

All entity-to-entity communication uses DIDComm v2.1. ATAP does not define a custom message format or transport layer.

### 7.2 Server Roles

An ATAP server serves two logically distinct roles. Implementations MUST treat them as separate concerns, even when deployed on the same infrastructure.

**DIDComm Mediator:** Untrusted relay. Routes messages based on envelope metadata. Cannot read message content due to DIDComm authenticated encryption (ECDH-1PU + A256CBC-HS512). Follows the standard DIDComm mediator specification.

**ATAP System Participant (`via`):** Trusted co-signer. Reads approval content, validates business rules, produces a signature. This role exists only in the approval flow — not all DIDComm messages involve the system as a participant.

DIDComm encryption ensures that approval content is protected from the mediator layer. Only the approval participants can decrypt the payload. The mediator sees routing metadata only.

### 7.3 ATAP Message Types

ATAP defines DIDComm message types under `https://atap.dev/protocols/`:

| Message Type | Purpose |
|-------------|---------|
| `approval/1.0/request` | Approval request (from → via or from → to) |
| `approval/1.0/cosigned` | Co-signed approval (via → to) |
| `approval/1.0/response` | Approval response (to → from [+ via]) |
| `approval/1.0/rejected` | System rejection (via → from, when co-signature is refused) |
| `approval/1.0/revoke` | Revocation of Standing Approval (to → own ATAP server) |
| `entity/1.0/shredded` | Crypto-shredding notification |

### 7.4 Delivery

DIDComm supports HTTPS, WebSocket, Bluetooth, and NFC. Service endpoints are declared in DID Documents. The ATAP server acts as DIDComm mediator for its hosted entities.

### 7.5 Organization Delegate Routing

Signals addressed to an `org:` entity are routed to entities whose DID Documents include `atap:principal` referencing that organization. The following constraints apply:

- Fan-out MUST be capped at 50 delegates.
- The server MUST apply per-source rate limiting.
- If multiple delegates respond to an approval request, the first valid response wins.
- If no delegate responds within the system-defined timeout, the request is considered declined.

---

## 8. Approvals

### 8.1 Overview

The approval is ATAP's core and sole protocol contribution. In its full form, it carries three independent cryptographic signatures:

1. **Requester (`from`)** — the entity initiating the action.
2. **System (`via`)** — the mediating service confirming the action details.
3. **Approver (`to`)** — the entity authorizing the action.

When no mediating system is involved, `via` is omitted and the approval carries two signatures (`from` and `to`). This two-party flow is used for direct entity-to-entity approvals such as a human authorizing an agent without a third-party system.

Approvals are **portable, self-contained documents**. They are transported via DIDComm and stored by the participating parties — not by the ATAP server. The server's role is transport (DIDComm mediator) and identity (DID Documents), not approval storage. This keeps the server stateless with respect to approvals, avoids unbounded storage growth, and reinforces the principle that approvals are verifiable by anyone holding the document — no server callback required.

No existing standard provides independently verifiable multi-party consent in a single portable document. OAuth and GNAP mediate consent through the authorization server — the resulting token does not carry proof of all parties' consent.

### 8.2 Two-Party and Three-Party Approvals

| Flow | Parties | Signatures | Use Case |
|------|---------|------------|----------|
| Two-party | `from`, `to` | 2 | Direct: human grants agent permission, peer-to-peer delegation. Approval or Standing Approval. |
| Three-party | `from`, `via`, `to` | 3 | Mediated: purchase, booking, contract via a system. Approval or Standing Approval. |

The `via` field is OPTIONAL. If present, the three-party signing sequence applies (Section 8.4). If absent, `from` signs and sends directly to `to`.

### 8.3 Approval Lifecycle

```
                    ┌──────────┐
                    │ requested│
                    └────┬─────┘
                         │
           ┌─────────┬───┼───┬──────────┐
           ▼         ▼   ▼   ▼          ▼
      ┌────────┐┌────────┐┌───────┐┌────────┐
      │approved││declined││expired││rejected│
      └───┬────┘└────────┘└───────┘└────────┘
          │
     ┌────┴─────┐
     ▼          ▼
┌────────┐┌────────┐
│consumed││revoked │
└────────┘└────────┘
```

| Transition | Trigger | Final | Applies To |
|-----------|---------|-------|------------|
| requested → approved | Approver signs `"approved"` | No | All |
| requested → declined | Approver signs `"declined"` | Yes | All |
| requested → expired | System-defined timeout | Yes | All |
| requested → rejected | System (`via`) refuses co-signature | Yes | Three-party only |
| approved → consumed | Approval used | Yes | Approval only |
| approved → revoked | Approver sends revocation | Yes | Standing Approval only |

A declined, expired, or rejected approval is final. A new approval with a new ID must be created to retry.

### 8.4 Signing Sequence

#### Three-Party (with `via`)

```
Step 1: from signs
  Requester creates approval, signs it.
  Sends approval/1.0/request via DIDComm to via.

Step 2: via validates and co-signs (or rejects)
  System validates subject and payload.
  If accepted: adds signature, sends approval/1.0/cosigned to to.
  If refused: sends approval/1.0/rejected to from with reason.

Step 3: to signs (approves or declines)
  Approver reviews rendered template or fallback.
  Signs approval response.
  Sends approval/1.0/response via DIDComm to from and via.
```

#### Two-Party (without `via`)

```
Step 1: from signs
  Requester creates approval (no via field), signs it.
  Sends approval/1.0/request via DIDComm to to.

Step 2: to signs (approves or declines)
  Approver reviews and signs.
  Sends approval/1.0/response via DIDComm to from.
```

Both sequences apply regardless of whether entities are on the same or different servers. DIDComm handles cross-server message routing.

### 8.5 Format

```json
{
  "atap_approval": "1",
  "id": "apr_8f3a9b2c",
  "created_at": "2026-03-12T14:30:00Z",

  "from": "did:web:provider.example:agent:01jd7xk3m9",
  "to": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "via": "did:web:merchant.example:machine:checkout",
  "parent": "<parent-approval-id>",

  "subject": {
    "type": "com.merchant.purchase",
    "label": "Order #ORD-7X9K — €129.90",
    "reversible": false,
    "payload": {
      "product": "Running Shoes Model X",
      "sku": "RS-X-BLK-44",
      "quantity": 1,
      "price": { "amount": 129.90, "currency": "EUR" },
      "delivery_estimate": "2-3 business days"
    }
  },

  "template_url": "https://merchant.example/atap/purchase-template.json",

  "signatures": {
    "from": "<JWS>",
    "via": "<JWS>"
  }
}
```

### 8.6 Fields

| Field | Required | Description |
|-------|----------|-------------|
| `atap_approval` | REQUIRED | MUST be `"1"`. |
| `id` | REQUIRED | Globally unique. `apr_` prefix. Implementations SHOULD generate IDs as `apr_` followed by a ULID (Universally Unique Lexicographically Sortable Identifier). |
| `created_at` | REQUIRED | ISO 8601 UTC. |
| `valid_until` | OPTIONAL | Absent/null = Approval (default TTL 60 minutes, RECOMMENDED, system-configurable). ISO 8601 datetime = Standing Approval. Subject to receiver `max_approval_ttl`. |
| `from` | REQUIRED | Requester DID. |
| `to` | REQUIRED | Approver DID. |
| `via` | OPTIONAL | Mediating system DID. If present, three-party flow. If absent, two-party flow. |
| `parent` | OPTIONAL | Parent approval ID for chaining. |
| `subject` | REQUIRED | What is being approved (see Section 8.7). |
| `template_url` | OPTIONAL | HTTPS URL to signed template (see Section 11). Only applicable when `via` is present. |
| `signatures` | REQUIRED | JWS signatures keyed by role (`from`, and `via` if applicable). |

### 8.7 Subject

| Field | Required | Description |
|-------|----------|-------------|
| `type` | REQUIRED | Reverse-domain-notation identifier. System owns its namespace. |
| `label` | REQUIRED | Single-line summary. Fallback display text. |
| `reversible` | REQUIRED | Boolean. |
| `payload` | REQUIRED | System-specific JSON. |

### 8.8 Signatures

Each signature is a JWS Compact Serialization (RFC 7515) with Detached Payload (RFC 7797). The signed payload is the UTF-8 encoding of the JCS-serialized (RFC 8785) approval document excluding the `signatures` field.

The JWS Protected Header MUST contain:

| Header | Requirement | Description |
|--------|-------------|-------------|
| `alg` | REQUIRED | JWA algorithm identifier (e.g., `EdDSA`). |
| `kid` | REQUIRED | MUST be a fully qualified DID URL (e.g., `did:web:provider.example:agent:01jd7xk3m9#key-1`). |

The `kid` is both the key identifier and the binding to the signer's DID. No additional identity or algorithm fields exist in the approval document — the JWS header is the single source of truth.

### 8.9 Verification

A verifier holding a complete approval MUST perform the following steps for each signature:

1. Parse the JWS to extract the Protected Header.
2. Extract `kid` from the header. It MUST be a fully qualified DID URL.
3. Extract the DID from `kid` (everything before the `#` fragment).
4. Verify the extracted DID matches the expected party:
   - `from` signature: DID MUST match the `from` field.
   - `via` signature (if present): DID MUST match the `via` field.
   - Approver signature: DID MUST match the `to` field.
5. Resolve the DID to obtain the DID Document.
6. Locate the verification method matching `kid`.
7. Verify the JWS against the located public key.

For three-party approvals: all three signatures MUST verify. For two-party approvals: both signatures MUST verify. Verification can be performed independently, offline, by any party with access to DID resolution.

### 8.10 System Rejection

When the `via` system refuses to co-sign an approval (e.g., policy violation, invalid payload, trust level insufficient), it sends an `approval/1.0/rejected` DIDComm message to `from`:

```json
{
  "type": "https://atap.dev/protocols/approval/1.0/rejected",
  "body": {
    "approval_id": "apr_8f3a9b2c",
    "reason": "insufficient_trust_level",
    "detail": "Effective trust level 1, required 2."
  }
}
```

The approval transitions to the `rejected` state. The requester MAY create a new approval with a new ID after resolving the rejection reason.

The following `reason` values are RECOMMENDED for interoperability. Implementations MAY define additional values.

| Reason | Description |
|--------|-------------|
| `insufficient_trust_level` | Entity's effective trust is below the system's minimum. |
| `invalid_payload` | Subject payload does not conform to the system's expectations. |
| `policy_violation` | Approval violates the system's business rules or compliance policy. |
| `unknown_entity` | The `from` or `to` entity is not recognized by the system. |
| `expired_credentials` | Required credentials have expired or been revoked. |
| `rate_limited` | Too many approval requests from this entity. |

### 8.11 Response

```json
{
  "atap_approval_response": "1",
  "approval_id": "apr_8f3a9b2c",
  "status": "approved",
  "responded_at": "2026-03-12T14:30:45Z",
  "signature": "<JWS>"
}
```

`status` MUST be `"approved"` or `"declined"`. The JWS `kid` header MUST reference the `to` entity's DID. Delivered via DIDComm to `from` and `via` (if present).

### 8.12 Approvals and Standing Approvals

| `valid_until` | Type | Behavior |
|---------------|------|----------|
| Absent or `null` | **Approval** | Single use. Expires after a default TTL of 60 minutes (RECOMMENDED, system-configurable). Transitions to `consumed` after use, or `expired` after TTL. |
| ISO 8601 datetime | **Standing Approval** | Valid for repeated use until that date, subject to `max_approval_ttl`. |

Standing Approvals are subject to the receiving server's `max_approval_ttl` policy (Section 9.3). A receiving server MUST apply `min(own_max_ttl, approval_valid_until)` regardless of what the issuing server allows.

### 8.13 Chained Approvals

An approval MAY reference a `parent`. The system SHOULD verify the parent's validity before accepting the child. Revoking a parent invalidates all children.

### 8.14 Standing Approval Enforcement

Before executing an action under a Standing Approval, the `via` system MUST perform the verification sequence defined in Section 8.15. If all checks pass: auto-approve. If any check fails: escalate via new Approval request to the approver.

### 8.15 Revocation

Standing Approvals are revoked by the approver via DIDComm message (`approval/1.0/revoke`). The message is sent to the approver's ATAP server, which records the approval ID in a **revocation list** indexed by the approver's DID. The approver's server also forwards the revocation to the `via` system via DIDComm, so that `via` can maintain its own local revocation records for immediate enforcement.

The ATAP server does not store approvals — it stores only revocation entries. This is a negative attestation model: absence from the revocation list means the approval has not been revoked (but the verifier must still check signatures, expiration, and entity liveness independently).

#### Self-Cleaning Revocation Lists

Each revocation entry carries an `expires_at` timestamp equal to the `valid_until` of the original Standing Approval, or `revoked_at` + 60 minutes for Approvals (which have no `valid_until`). Servers SHOULD remove expired revocation entries. This bounds the revocation list size to the number of active, revoked Standing Approvals — it does not grow indefinitely.

#### Revocation List API

The approver's ATAP server exposes:

`GET {api_base}/v1/revocations?entity={approver-did}`

Returns all active revoked approval IDs for that entity:

```json
{
  "entity": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "revocations": [
    { "approval_id": "apr_4d7e9f1a", "revoked_at": "2026-06-01T10:00:00Z", "expires_at": "2026-09-12T00:00:00Z" },
    { "approval_id": "apr_8f3a9b2c", "revoked_at": "2026-07-15T14:30:00Z", "expires_at": "2026-07-15T15:30:00Z" }
  ],
  "checked_at": "2026-08-01T12:00:00Z"
}
```

#### Verification Before Execution

Before executing an action under a Standing Approval, the `via` system MUST perform the following checks:

1. **Signatures valid** — verify all JWS signatures (offline).
2. **`valid_until` not passed** — apply `min(own_max_ttl, approval_valid_until)` (offline).
3. **Approval not revoked** — check own local revocation records first, then query the approver's ATAP server revocation list.
4. **Approver DID active** — resolve the approver's DID Document. A deactivated DID Document means the entity has been revoked entirely.
5. **Agent DID active** — resolve the `from` entity's DID Document. Verify the `atap:principal` claim still references the approver.
6. **Parent valid** — if `parent` is set, recursively verify the parent approval.
7. **Payload rules satisfied** — apply the system's own business rules.

Steps 1–2 are offline. Step 3 checks local records first (fast path), then the approver's server (authoritative). Steps 4–5 are DID resolution. Step 6 is recursive. Step 7 is system-local.

For Approvals (without `valid_until`), the `via` system decides which checks to perform based on its own risk assessment. A low-value transaction may require only signature verification. A high-value transaction may require all checks.

### 8.16 Unanswered Requests

Unanswered requests SHOULD be treated as expired after a system-defined timeout.

---

## 9. Server Trust

### 9.1 Overview

Server trust is established via WebPKI, DNSSEC, and VC-based attestations. ATAP does not define a custom trust hierarchy.

### 9.2 RECOMMENDED Default Trust Policy

| Trust Level | Requirements |
|-------------|-------------|
| 0 | No TLS, self-signed certificate, or DV certificate without DNSSEC. |
| 1 | Domain-Validated (DV) TLS certificate AND DNSSEC. |
| 2 | Organization-Validated (OV) or Extended Validation (EV) TLS certificate AND DNSSEC. |
| 3 | OV/EV certificate AND DNSSEC AND a Verifiable Credential attesting audit compliance (SOC 2, ISO 27001, or eIDAS), issued by an accredited audit body. |

DNSSEC is REQUIRED for trust level 1 and above. Without DNSSEC, the `did:web` resolution mechanism is vulnerable to DNS poisoning.

Audit VCs MUST be issued by bodies accredited under recognized schemes: AICPA (SOC 2), ISO/IEC accreditation bodies (ISO 27001), or eIDAS Trusted Lists (qualified trust service providers).

Verifiers MAY override this mapping with their own policy, but SHOULD document their trust assessment criteria.

### 9.3 Server Discovery

Every ATAP server MUST publish at `https://{domain}/.well-known/atap.json`:

```json
{
  "atap_server": "1",
  "domain": "<domain>",
  "api_base": "<base-URL>",
  "didcomm_endpoint": "<DIDComm-service-URL>",
  "credentials": [
    "<URL-to-audit-attestation-VC>"
  ],
  "claim_types": {
    "ATAPEmailVerification":  { "level": 1 },
    "ATAPPhoneVerification":  { "level": 1 },
    "ATAPPersonhood":         { "level": 2 },
    "ATAPIdentity":           { "level": 3 },
    "ATAPPrincipal":          { "level": 0 },
    "ATAPOrgMembership":      { "level": 0 }
  },
  "max_approval_ttl": "P90D",
  "legal_entity": "<registered-name>",
  "jurisdiction": "<ISO-3166-1>"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `atap_server` | REQUIRED | MUST be `"1"`. |
| `domain` | REQUIRED | Server's domain. |
| `api_base` | REQUIRED | Base URL for ATAP API endpoints. |
| `didcomm_endpoint` | REQUIRED | DIDComm messaging endpoint. |
| `credentials` | OPTIONAL | URLs to VCs attesting organizational identity and audit status. |
| `claim_types` | REQUIRED | Mapping from ATAP credential types to trust levels. |
| `max_approval_ttl` | RECOMMENDED | Maximum `valid_until` duration. ISO 8601 duration. |
| `legal_entity` | RECOMMENDED | Registered legal name. |
| `jurisdiction` | RECOMMENDED | ISO 3166-1 jurisdiction. |

### 9.4 Effective Trust

```
effective_trust(entity) = min(trust_level(entity), server_trust(entity.server))
```

---

## 10. Authorization: OAuth 2.1 + DPoP

### 10.1 Overview

API access to ATAP servers uses OAuth 2.1 with DPoP (RFC 9449) for sender-constrained tokens.

### 10.2 Token Acquisition

Agent entities use the Client Credentials grant. Human entities authenticate via device biometric and receive tokens via Authorization Code grant with PKCE.

### 10.3 DPoP Token Binding

All API tokens MUST be DPoP-bound. Each request includes a DPoP proof JWT demonstrating possession of the private key.

### 10.4 Token Scoping

| Scope | Permits |
|-------|---------|
| `atap:inbox` | Read inbox, SSE stream |
| `atap:send` | Send DIDComm messages |
| `atap:revoke` | Submit approval revocations |
| `atap:manage` | Manage credentials, key rotation |

### 10.5 Token Lifecycle

Token expiration, refresh, and revocation follow OAuth 2.1 conventions. Default access token lifetime: 1 hour. Refresh tokens: up to 90 days.

---

## 11. Templates

### 11.1 Overview

A template defines how an approval is rendered on the approver's device. Templates use Microsoft Adaptive Cards, an open, platform-agnostic JSON standard for describing rich UI cards. ATAP does not define a custom template format.

Templates are provided exclusively by the mediating system (`via`). Each template carries a JWS proof signed by the `via` entity. Two-party approvals (without `via`) do not use templates and are rendered using the fallback renderer: `subject.label` as title and `subject.payload` as formatted JSON.

If a template is unavailable or verification fails, the client falls back to the same default rendering.

### 11.2 Why Adaptive Cards

Adaptive Cards provide layout primitives (columns, containers, grids), 30+ element types (text, images, fact sets, inputs), data binding via `${expression}` syntax, a visual designer at adaptivecards.io, and rendering SDKs for JavaScript, .NET, iOS, Android, and Flutter. ATAP leverages this ecosystem rather than defining its own template language.

### 11.3 Template Format

An ATAP template is an Adaptive Cards JSON document wrapped with an ATAP proof:

```json
{
  "atap_template": "1",
  "card": {
    "type": "AdaptiveCard",
    "version": "1.5",
    "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
    "body": [
      {
        "type": "ColumnSet",
        "columns": [
          {
            "type": "Column",
            "width": "auto",
            "items": [{
              "type": "Image",
              "url": "${brand.logo_url}",
              "size": "Small"
            }]
          },
          {
            "type": "Column",
            "width": "stretch",
            "items": [{
              "type": "TextBlock",
              "text": "${brand.name}",
              "weight": "Bolder",
              "size": "Medium"
            }]
          }
        ]
      },
      {
        "type": "TextBlock",
        "text": "${subject.label}",
        "size": "Large",
        "weight": "Bolder",
        "wrap": true
      },
      {
        "type": "FactSet",
        "facts": [
          { "title": "Product", "value": "${payload.product}" },
          { "title": "Price", "value": "${payload.price.amount} ${payload.price.currency}" },
          { "title": "Delivery", "value": "${payload.delivery_estimate}" }
        ]
      }
    ]
  },
  "proof": {
    "kid": "did:web:merchant.example:machine:checkout#key-1",
    "alg": "EdDSA",
    "sig": "<base64url-JWS>"
  }
}
```

The `card` field contains a standard Adaptive Card. The `proof` field contains a JWS Detached Payload signature over the JCS-serialized template excluding the `proof` field.

### 11.4 Data Binding

The Adaptive Card template uses Adaptive Card Templating syntax to bind data from the approval. The client populates the template with the following data context:

```json
{
  "subject": { ... },
  "payload": { ... },
  "brand": { ... },
  "from": "<requester-DID>",
  "to": "<approver-DID>",
  "via": "<system-DID>"
}
```

Where `subject` and `payload` come from the approval's `subject` field, and `brand` is an optional object provided alongside the template. The `${payload.product}`, `${payload.price.amount}`, and similar expressions in the template are resolved against this context before rendering.

### 11.5 Template Verification

1. Fetch template from `template_url`.
2. Extract `proof.kid`. The DID portion MUST match the `via` entity's DID. If not: reject.
3. Resolve the DID Document, extract the public key by `kid`.
4. Verify JWS over the template (excluding `proof`).
5. Success → bind data and render Adaptive Card. Failure → fallback.

Verified templates MAY be cached indefinitely — the signature guarantees integrity regardless of cache origin.

### 11.6 Rendering

The client renders the Adaptive Card using a platform-appropriate Adaptive Cards SDK:

| Platform | Library |
|----------|---------|
| Flutter | `flutter_adaptive_cards_plus` or `adaptive_card_renderer` |
| iOS | Adaptive Cards iOS SDK |
| Android | Adaptive Cards Android SDK |
| Web | `adaptivecards-js` |

The client SHOULD configure the Adaptive Cards HostConfig to match the ATAP app's visual style. The HostConfig controls fonts, colors, spacing, and other theming properties without modifying the template itself.

### 11.7 Security

- HTTPS only. HTTP MUST be rejected.
- No HTTP redirects.
- IP validation after DNS resolution. Block RFC 1918, link-local (169.254.0.0/16), loopback (127.0.0.0/8), cloud metadata (169.254.169.254).
- Maximum response size: 64 KB.
- Fetch timeout: 5 seconds.
- Adaptive Card `Action.Submit` and `Action.OpenUrl` MUST be disabled in ATAP approval templates. The only permitted actions are the ATAP-native Approve and Decline buttons rendered by the client outside the card.
- Authenticity verified via `proof`, not via URL or transport.
- The `$schema` field in Adaptive Cards is a schema identifier, not a fetch target. Clients MUST NOT fetch the schema URL at runtime.

---

## 12. Mobile Client

### 12.1 Overview

The ATAP mobile client is the primary interface for human entities.

### 12.2 Functions

| Function | Description |
|----------|-------------|
| **Onboarding** | Generate keypair in secure enclave. Create `did:web` DID. Set recovery passphrase. Add credentials. |
| **Inbox** | DIDComm message feed. |
| **Approval Rendering** | Fetch and verify template. Render Adaptive Card or fallback. Approve/decline with biometric. |
| **Credential Management** | View, present, and revoke Verifiable Credentials. |
| **Approval Management** | List locally stored Standing Approvals. Revoke via ATAP server. |
| **Multi-Server** | Multiple DIDs from different servers. Unified inbox. |

### 12.3 Signing

Biometric prompt → JWS signature from secure enclave → send approval response via DIDComm.

---

## 13. API

### 13.1 Base URL

Declared in `/.well-known/atap.json`. Default: `https://api.{domain}/v1`. This is separate from the DID resolution path (Section 5.2).

### 13.2 Authentication

OAuth 2.1 + DPoP (Section 10).

### 13.3 Endpoints

#### Entities

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/entities | Register entity (returns DID) | Varies |
| GET | /v1/entities/{id} | Get entity info | Public |
| DELETE | /v1/entities/{id} | Crypto-shred entity | Owner |

#### DID Resolution (W3C `did:web` standard path)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /{type}/{id}/did.json | Resolve DID Document | Public |

#### Revocations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/revocations | Submit signed approval revocation | `atap:revoke` |
| GET | /v1/revocations | Query revocations by entity DID | Public |

Approvals are transported via DIDComm (Section 7), not via REST API. The server does not store approvals. The revocation endpoint stores only approval IDs that have been revoked, indexed by the approver's DID.

#### Credentials

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/credentials/email/start | Initiate email verification | `atap:manage` |
| POST | /v1/credentials/email/verify | Complete email verification | `atap:manage` |
| POST | /v1/credentials/phone/start | Initiate phone verification | `atap:manage` |
| POST | /v1/credentials/personhood | Submit proof-of-personhood | `atap:manage` |
| GET | /v1/credentials | List own credentials | `atap:manage` |
| GET | /v1/credentials/status/{list-id} | Bitstring Status List | Public |

#### DIDComm

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/didcomm | DIDComm message endpoint | DIDComm auth |

#### Discovery

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | /.well-known/atap.json | Server discovery | Public |

### 13.4 Errors (RFC 7807)

```json
{
  "type": "https://atap.dev/errors/{error-type}",
  "title": "<description>",
  "status": 403,
  "detail": "<detail>"
}
```

---

## 14. Relationship to Other Protocols

**MCP** — ATAP approvals can authorize irreversible MCP tool actions.

**MCP-I** — ATAP's `did:web` DIDs and VC-based credentials are natively compatible with MCP-I's DID/VC identity extensions.

**A2A** — Agents verify each other's principals by resolving DIDs and checking `ATAPPrincipal` credentials.

**AP2 / Verifiable Intent** — An ATAP approval is a verifiable intent record. AP2's VC-signed Mandates and ATAP approvals serve complementary roles.

**GNAP (RFC 9635)** — ATAP's approval model can be expressed as a GNAP extension where the multi-signature approval supplements standard GNAP tokens for operations requiring multi-party consent.

**OAuth 2.1** — ATAP uses OAuth 2.1 for API authentication. Approvals are orthogonal to OAuth tokens — they prove multi-party consent, not API access.

**eIDAS 2.0** — `ATAPIdentity` credentials from qualified trust service providers provide trust level 3. ATAP entities can receive and present credentials from EU Digital Identity Wallets via OpenID4VP.

**W3C Verifiable Presentations** — A completed ATAP approval with all signatures MAY be wrapped in a Verifiable Credential of type `ATAPApproval` for presentation to third parties via OpenID4VP. The approval document becomes `credentialSubject`. This enables existing VC/VP infrastructure to transport and verify ATAP approvals without native ATAP support. Detailed specification deferred to v1.1.

**Visa Trusted Agent Protocol (TAP)** — TAP provides HTTP-level agent authentication for merchant interactions using RFC 9421 message signatures. ATAP approvals provide the multi-party consent proof for actions initiated during a TAP-authenticated session. TAP identifies the agent; ATAP authorizes the action.

---

## 15. Security Considerations

**Signatures** — JWS (RFC 7515) with JWA algorithm identifiers. EdDSA (Ed25519) for v1. Post-quantum migration via composite JWS when IETF composite signatures (draft-ietf-lamps-pq-composite-sigs) are finalized. Implementations SHOULD prepare for composite identifiers such as `EdDSA+ML-DSA-65`.

**Messaging** — DIDComm v2.1 authenticated encryption (ECDH-1PU + A256CBC-HS512). Approval content encrypted for participants only; mediators see routing metadata, not payload.

**Replay Protection** — DPoP (RFC 9449) at the API layer. DIDComm at the message layer.

**Template Security** — Templates use Adaptive Cards (declarative JSON). Provided exclusively by `via` system, self-signed with JWS. Client verifies proof against `via` DID. Adaptive Card actions (Submit, OpenUrl) disabled in approval context. Two-party approvals use fallback rendering. SSRF mitigations per Section 11.7.

**Privacy** — Human DIDs contain no PII. SD-JWT selective disclosure for credentials. DIDComm encryption for message privacy. Crypto-shredding for GDPR erasure. Personhood credentials MUST NOT contain or transmit raw biometric data.

**Key Management** — Server-mediated via DID Document updates. Previous key versions retained. `did:webs` as upgrade path for server-independent authority.

**Server Trust** — RECOMMENDED default policy maps TLS + DNSSEC + audit VCs to trust levels. DNSSEC required for level ≥ 1.

**Standing Approvals** — Server does not store approvals; parties store their own. Revocation via negative attestation (revocation lists on approver's server, forwarded to `via` for local caching). Self-cleaning lists bounded by active revoked Standing Approvals. Verification before execution: signatures, expiration, revocation list (local then remote), entity liveness (DID resolution), principal claim validity. Receiver-side enforcement: MUST apply `min(own_max_ttl, approval_valid_until)`. Risk-based checks for Approvals.

**Revocation Suppression** — Revocation delivery depends on the approver's ATAP server. A compromised server could suppress revocation signals. The damage window is bounded by `valid_until` — a Standing Approval will expire regardless of whether revocation is delivered. In the event of server compromise, DID deactivation invalidates all Standing Approvals immediately via the DID liveness check (Section 8.15 Step 4). Future versions MAY introduce witness-based revocation for environments requiring stronger guarantees.

**Verification Binding** — `kid` in each JWS header MUST be a fully qualified DID URL. DID portion MUST match the corresponding `from`/`via`/`to` field.

**System Rejection** — Explicit `rejected` state and DIDComm message type for when `via` refuses to co-sign. Prevents silent failures.

---

## 16. Roadmap

**Phase 1 (Weeks 1–4):** Entity registration with `did:web`. DIDComm messaging. Two-party and three-party approvals with standard templates. OAuth 2.1 + DPoP API auth.

**Phase 2 (Weeks 5–8):** VC-based credentials (email, phone, personhood). Standing Approvals with `max_approval_ttl`. Signed custom templates. Cross-server DIDComm relay.

**Phase 3 (Weeks 9–12):** Organizations with delegate routing. SD-JWT selective disclosure. Credential revocation via Bitstring Status List. Multi-server client. Crypto-shredding.

**Phase 4 (Months 4–6):** Spec v1.0 final. GNAP extension draft. Formal verification of approval flow (Tamarin Prover). eIDAS 2.0 credential import. ATAPApproval-as-VC wrapping. Community governance.

---

## Appendix A: Three-Party Approval (Purchase)

```json
{
  "atap_approval": "1",
  "id": "apr_8f3a9b2c",
  "created_at": "2026-03-12T14:30:00Z",

  "from": "did:web:provider.example:agent:01jd7xk3m9",
  "to": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "via": "did:web:merchant.example:machine:checkout",

  "subject": {
    "type": "com.merchant.purchase",
    "label": "Order #ORD-7X9K — €129.90",
    "reversible": false,
    "payload": {
      "product": "Running Shoes Model X",
      "sku": "RS-X-BLK-44",
      "quantity": 1,
      "price": { "amount": 129.90, "currency": "EUR" },
      "delivery_estimate": "2-3 business days"
    }
  },

  "template_url": "https://merchant.example/atap/purchase-template.json",

  "signatures": {
    "from": "eyJhbGciOiJFZERTQSIsImtpZCI6ImRpZDp3ZWI6cHJvdmlkZXIuZXhhbXBsZTphZ2VudDowMWpkN3hrM205I2tleS0xIn0..sig",
    "via": "eyJhbGciOiJFZERTQSIsImtpZCI6ImRpZDp3ZWI6bWVyY2hhbnQuZXhhbXBsZTptYWNoaW5lOmNoZWNrb3V0I2tleS0xIn0..sig"
  }
}
```

---

## Appendix B: Two-Party Approval (Direct Delegation)

```json
{
  "atap_approval": "1",
  "id": "apr_direct_01",
  "created_at": "2026-03-12T16:00:00Z",
  "valid_until": "2026-06-12T00:00:00Z",

  "from": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "to": "did:web:provider.example:agent:01jd7xk3m9",

  "subject": {
    "type": "dev.atap.agent-permission",
    "label": "Schedule meetings on my behalf",
    "reversible": true,
    "payload": {
      "permissions": ["calendar.read", "calendar.write"],
      "max_attendees": 10
    }
  },

  "signatures": {
    "from": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig"
  }
}
```

---

## Appendix C: Standing Approval (Three-Party, Recurring) (Recurring)

```json
{
  "atap_approval": "1",
  "id": "apr_4d7e9f1a",
  "created_at": "2026-03-12T15:00:00Z",
  "valid_until": "2026-09-12T00:00:00Z",

  "from": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "to": "did:web:provider.example:agent:01jd7xk3m9",
  "via": "did:web:grocery.example:machine:shop",

  "subject": {
    "type": "com.grocery.recurring-order",
    "label": "Recurring grocery orders — up to €75 per order",
    "reversible": true,
    "payload": {
      "max_per_order": { "amount": 75, "currency": "EUR" },
      "max_per_period": { "amount": 300, "currency": "EUR", "period": "monthly" },
      "categories": ["groceries"],
      "escalate_above": { "amount": 75, "currency": "EUR" }
    }
  },

  "template_url": "https://grocery.example/atap/recurring-template.json",

  "signatures": {
    "from": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig",
    "via": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig"
  }
}
```

---

## Appendix D: Chained Approvals (Budget Delegation)

### Parent (Human → Human)

```json
{
  "atap_approval": "1",
  "id": "apr_budget_q2",
  "created_at": "2026-04-01T09:00:00Z",
  "valid_until": "2026-06-30T00:00:00Z",

  "from": "did:web:corp.example:human:a3b5c7d9e1f2g4h6",
  "to": "did:web:corp.example:human:j8k2m4n6p8q1r3s5",
  "via": "did:web:corp.example:machine:procurement",

  "subject": {
    "type": "com.corp.budget-delegation",
    "label": "Q2 Department Budget — €10.000/month",
    "reversible": true,
    "payload": {
      "department": "Engineering",
      "max_per_period": { "amount": 10000, "currency": "EUR", "period": "monthly" },
      "categories": ["software", "hardware", "services"],
      "escalate_above": { "amount": 5000, "currency": "EUR" }
    }
  },

  "template_url": "https://corp.example/atap/budget-template.json",

  "signatures": {
    "from": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig",
    "via": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig"
  }
}
```

### Child (Human → Agent, with parent)

```json
{
  "atap_approval": "1",
  "id": "apr_agent_sw",
  "created_at": "2026-04-01T10:00:00Z",
  "valid_until": "2026-06-30T00:00:00Z",

  "from": "did:web:corp.example:human:j8k2m4n6p8q1r3s5",
  "to": "did:web:corp.example:agent:01je9yk4n2p7",
  "via": "did:web:corp.example:machine:procurement",

  "parent": "apr_budget_q2",

  "subject": {
    "type": "com.corp.software-procurement",
    "label": "Software procurement — €5.000/month",
    "reversible": true,
    "payload": {
      "max_per_period": { "amount": 5000, "currency": "EUR", "period": "monthly" },
      "max_per_transaction": { "amount": 500, "currency": "EUR" },
      "categories": ["software"],
      "escalate_above": { "amount": 500, "currency": "EUR" }
    }
  },

  "template_url": "https://corp.example/atap/agent-budget-template.json",

  "signatures": {
    "from": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig",
    "via": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig"
  }
}
```

---

## Appendix E: Signed Template (Adaptive Card)

```json
{
  "atap_template": "1",
  "card": {
    "type": "AdaptiveCard",
    "version": "1.5",
    "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
    "body": [
      {
        "type": "ColumnSet",
        "columns": [
          {
            "type": "Column",
            "width": "auto",
            "items": [{
              "type": "Image",
              "url": "${brand.logo_url}",
              "size": "Small"
            }]
          },
          {
            "type": "Column",
            "width": "stretch",
            "items": [{
              "type": "TextBlock",
              "text": "${brand.name}",
              "weight": "Bolder",
              "size": "Medium"
            }]
          }
        ]
      },
      {
        "type": "TextBlock",
        "text": "${subject.label}",
        "size": "Large",
        "weight": "Bolder",
        "wrap": true
      },
      {
        "type": "FactSet",
        "facts": [
          { "title": "Product", "value": "${payload.product}" },
          { "title": "SKU", "value": "${payload.sku}" },
          { "title": "Quantity", "value": "${payload.quantity}" },
          { "title": "Price", "value": "${payload.price.amount} ${payload.price.currency}" },
          { "title": "Delivery", "value": "${payload.delivery_estimate}" }
        ]
      }
    ]
  },
  "proof": {
    "kid": "did:web:merchant.example:machine:checkout#key-1",
    "alg": "EdDSA",
    "sig": "eyJhbGciOiJFZERTQSIsImtpZCI6Ii4uLiJ9..sig"
  }
}
```

---

## Appendix F: Approval Lifecycle

```
Approval (two-party):
  requested ──approved──→ consumed
  requested ──approved──→ active ──revoked──→ (final)  (standing)
  requested ──approved──→ active ──expired──→ (final)  (standing)
  requested ──declined──→ (final)
  requested ──expired───→ (final)

Approval (three-party):
  requested ──cosigned──→ awaiting_approval
  requested ──rejected──→ (final)
  awaiting_approval ──approved──→ consumed
  awaiting_approval ──approved──→ active ──revoked──→ (final)  (standing)
  awaiting_approval ──approved──→ active ──expired──→ (final)  (standing)
  awaiting_approval ──declined──→ (final)
  awaiting_approval ──expired───→ (final)
```

---

## References

- W3C — Decentralized Identifiers (DIDs) v1.0
- W3C — `did:web` Method Specification
- W3C — Verifiable Credentials Data Model 2.0
- W3C — Bitstring Status List v1.0
- DIF — DIDComm Messaging v2.1
- RFC 6749 / OAuth 2.1 — Authorization Framework
- RFC 7515 — JSON Web Signature (JWS)
- RFC 7797 — JWS Unencoded Payload Option
- RFC 7807 — Problem Details for HTTP APIs
- RFC 8032 — EdDSA (Ed25519)
- RFC 8785 — JSON Canonicalization Scheme (JCS)
- RFC 9106 — Argon2
- RFC 9449 — OAuth 2.0 DPoP
- RFC 9635 — GNAP
- RFC 9901 — SD-JWT
- Microsoft — Adaptive Cards (adaptivecards.io)
- IETF draft-ietf-lamps-pq-composite-sigs — Composite Signatures
- Anthropic — Model Context Protocol (MCP)
- DIF — MCP-I
- Google / Linux Foundation — Agent-to-Agent Protocol (A2A)
- Google — Agent Payments Protocol (AP2)
- Mastercard — Verifiable Intent (2026)
- Tools for Humanity — World ID Protocol
- Cloud Security Alliance — ARIA
- Visa — Trusted Agent Protocol (TAP)
- RFC 9421 — HTTP Message Signatures

---

*Copyright 2026 ATAP Contributors. Licensed under the Apache License, Version 2.0.*
