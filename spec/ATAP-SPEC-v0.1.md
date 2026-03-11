# ATAP: Agent Trust and Authority Protocol

## Draft Specification v0.1

**Status:** Draft  
**Date:** March 2026  
**Authors:** Sven (SIMRelay GmbH)  
**License:** Apache 2.0 (spec, platform, SDKs, mobile app — all open source)  

---

## Table of Contents

1. [Abstract](#1-abstract)
2. [Introduction](#2-introduction)
3. [Terminology](#3-terminology)
4. [Design Principles](#4-design-principles)
5. [Entity Model](#5-entity-model)
6. [Trust Levels](#6-trust-levels)
7. [Cryptographic Primitives](#7-cryptographic-primitives)
8. [Signal Format](#8-signal-format)
9. [Delegation Documents](#9-delegation-documents)
10. [Inbox and Signal Delivery](#10-inbox-and-signal-delivery)
11. [Channels](#11-channels)
12. [Claim Flow](#12-claim-flow)
13. [Branded Approval Templates](#13-branded-approval-templates)
14. [Verification](#14-verification)
15. [Revocation](#15-revocation)
16. [Federation and Key Discovery](#16-federation-and-key-discovery)
17. [Security Considerations](#17-security-considerations)
18. [Relationship to Other Protocols](#18-relationship-to-other-protocols)
19. [IANA Considerations](#19-iana-considerations)
20. [Appendix A: Full Signal Example](#appendix-a-full-signal-example)
21. [Appendix B: Full Delegation Example](#appendix-b-full-delegation-example)
22. [Appendix C: JSON Schemas](#appendix-c-json-schemas)

---

## 1. Abstract

The Agent Trust and Authority Protocol (ATAP) is an open protocol for establishing verifiable delegation of trust between AI agents, machines, humans, and organizations. ATAP provides a universal identity layer for the agent economy, enabling any party receiving a request from an AI agent to cryptographically verify who authorized that agent, what it is permitted to do, and under what constraints.

ATAP defines four entity types (`agent://`, `machine://`, `human://`, `org://`), a signal format for entity-to-entity communication, a delegation document format for expressing scoped authorization chains, and a federated key discovery mechanism that ensures no single operator controls the trust graph.

ATAP is transport-agnostic and complements existing protocols such as MCP (Model Context Protocol), A2A (Agent-to-Agent), and AP2 (Agent Payments Protocol) by providing the identity and trust layer they assume but do not define.

---

## 2. Introduction

### 2.1 Problem Statement

AI agents are increasingly capable of acting autonomously — booking flights, executing payments, accessing services, and collaborating with other agents. However, the current ecosystem lacks a standardized answer to three fundamental questions:

1. **Identity:** Who is this agent?
2. **Authorization:** Who authorized this agent to act, and with what scope?
3. **Accountability:** If something goes wrong, who is responsible?

Existing protocols address adjacent concerns: MCP standardizes agent-to-tool communication, A2A standardizes agent-to-agent communication, and AP2 standardizes agent-initiated payments. All three assume that identity and trust have been established elsewhere. ATAP fills this gap.

### 2.2 Goals

ATAP is designed to:

- Provide every agent, machine, human, and organization with a unique, cryptographically verifiable identity.
- Enable humans to delegate scoped, time-limited, revocable authority to agents and machines.
- Allow any party to verify a delegation chain without depending on a central authority.
- Support progressive trust levels from anonymous agents to government-verified identities.
- Remain transport-agnostic, working over HTTP, WebSocket, SSE, or any other transport.
- Be simple enough that a developer can integrate basic functionality in under 30 minutes.

### 2.3 Non-Goals

ATAP does not:

- Define how agents reason, plan, or execute tasks.
- Prescribe a specific transport protocol (though it recommends SSE for signal delivery).
- Replace payment protocols (AP2), communication protocols (A2A), or tool protocols (MCP).
- Require blockchain, though it supports optional blockchain anchoring for revocation proofs.
- Mandate a specific identity provider, though it integrates with World ID and other providers.

---

## 3. Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC 2119.

| Term | Definition |
|------|-----------|
| **Entity** | Any participant in the ATAP network: an agent, machine, human, or organization. |
| **Agent** | An ephemeral, goal-driven AI actor that performs tasks. Agents are typically short-lived and may not be continuously reachable. |
| **Machine** | A persistent application, service, or system. Machines are long-lived and typically have stable network presence. |
| **Human** | A natural person who serves as a trust anchor. Humans participate in the signal protocol via mobile app: they send and receive signed signals, approve delegations, and instruct agents. |
| **Attestation** | A verified claim about an entity (e.g., email address, phone number, World ID proof). Attestations are properties of an identity, not the identity itself. |
| **Organization** | A legal or organizational entity that serves as an umbrella for machines and humans. |
| **Signal** | A message sent between entities via the ATAP signal format. |
| **Inbox** | A durable, addressable message queue associated with an entity. |
| **Channel** | A unique inbound pathway to an inbox (e.g., a webhook URL). One inbox may have many channels. |
| **Delegation** | A cryptographically signed document granting scoped authority from a principal to a delegate. |
| **Principal** | The entity granting authority in a delegation. |
| **Delegate** | The entity receiving authority in a delegation. |
| **Delegation Chain** | An ordered sequence of delegations from a root principal (typically a human) through intermediate entities to a final delegate. |
| **Trust Level** | A numeric classification (0–3) indicating the verification depth of an entity's trust chain. |
| **Claim** | The process by which a human asserts principal authority over an agent or machine. |
| **Registry** | A service that stores and serves public keys and entity metadata. Multiple registries may coexist. |

---

## 4. Design Principles

### 4.1 Machine-Native Vocabulary

ATAP avoids borrowing metaphors from human communication systems. Entities emit and receive **signals**, not letters. Signals are delivered to **inboxes** via **channels**, not mailboxes. The protocol defines **routes**, not envelopes. This vocabulary reflects how event-driven distributed systems actually operate.

### 4.2 Self-Verifying Documents

Delegation documents MUST be self-contained and cryptographically verifiable without contacting a central server. A verifier with access to the relevant public keys can validate any delegation chain offline. This is analogous to a passport: the document itself is the proof.

### 4.3 Progressive Trust

Trust is not binary. ATAP defines four trust levels (0–3) that entities can achieve through increasing levels of verification. Services receiving signals can set their own minimum trust level requirements.

### 4.4 Agent-First Design

The protocol is designed from the agent's perspective. Registration is instant and requires no human involvement. Humans are introduced only when an agent needs to elevate its trust level. This ensures zero-friction onboarding.

### 4.5 Federation by Default

No single operator controls the ATAP trust graph. Public keys can be discovered via DNS, well-known endpoints, or any compliant registry. Revocations are published as signed lists that any party can host and verify.

### 4.6 Transport Agnostic

ATAP defines a signal format and a delegation format, not a transport protocol. Signals can be delivered over SSE, HTTP polling, webhooks, WebSocket, or any future transport. The recommended delivery mechanism is SSE (Server-Sent Events) for its simplicity, HTTP compatibility, and built-in reconnection semantics.

---

## 5. Entity Model

### 5.1 Entity Types

ATAP defines four entity types. Each type has a URI scheme, a distinct lifecycle, and a specific role in the trust model.

#### 5.1.1 Agent (`agent://`)

An agent is an ephemeral, goal-driven AI actor. Agents are the primary consumers of ATAP infrastructure.

- **Lifecycle:** Short-lived to medium-lived. May spin up for a single task and disappear.
- **Reachability:** May be offline for extended periods. Requires durable inbox.
- **Trust:** Begins at Level 0 (anonymous). Elevates through claim flow.
- **Creation:** Self-registration via API. No human approval required.
- **Identity:** Ed25519 keypair generated at registration.

#### 5.1.2 Machine (`machine://`)

A machine is a persistent application, service, or platform.

- **Lifecycle:** Long-lived. Expected to be continuously or near-continuously available.
- **Reachability:** Typically has stable network endpoints. Prefers push delivery.
- **Trust:** Inherits trust from the human or organization that registered it.
- **Creation:** Registered by a human or organization.
- **Identity:** Ed25519 keypair generated at registration. May also publish keys via DNS or well-known endpoints.

#### 5.1.3 Human (`human://`)

A human is a natural person who serves as a trust anchor and participates in the signal protocol.

- **Lifecycle:** Persistent.
- **Reachability:** Receives signals via the ATAP signal protocol through the mobile app. Has an inbox like any other entity. Push notifications alert the human to new signals.
- **Trust:** Established through attestations (email, phone, World ID, eID). Attestations are properties of the identity, not the identity itself.
- **Creation:** Registration through mobile app. Keypair generated in device secure enclave.
- **Identity:** Derived from public key. The human ID is a truncated hash of the Ed25519 public key (see Section 5.2.1). This ensures the identity is self-sovereign — no external provider can revoke it.
- **Communication:** Humans send and receive signed signals. Instructions from a human to an agent are signed with the human's key, ensuring agents can cryptographically verify that instructions come from their principal.

#### 5.1.4 Organization (`org://`)

An organization is a legal or organizational entity.

- **Lifecycle:** Persistent.
- **Reachability:** Does NOT receive signals directly. Serves as a namespace and trust umbrella.
- **Trust:** Established through domain verification, company registry, or equivalent.
- **Creation:** Registered by a human.
- **Identity:** Ed25519 keypair. May publish keys via domain DNS.

### 5.2 Entity URI Scheme

Every entity is identified by a URI of the form:

```
{type}://{identifier}
```

Where `{type}` is one of `agent`, `machine`, `human`, or `org`, and `{identifier}` is a globally unique opaque string.

Examples:

```
agent://a1b2c3d4e5f6
machine://simrelay-prod
human://h7x9k2m4p3n8j5w2
org://simrelay-gmbh
```

Identifiers MUST be:
- Between 4 and 64 characters.
- Composed of lowercase alphanumeric characters and hyphens.
- Unique within their entity type across the registry that issued them.

#### 5.2.1 Human Identity Derivation

Human identifiers are derived from the human's Ed25519 public key, making the identity self-sovereign and independent of any external provider (email, phone number, etc.):

```
human_id = lowercase(base32_encode(sha256(ed25519_public_key))[:16])
```

This means:
- The identity IS the key. No external dependency or provider can revoke it.
- Email and phone are attestations that raise trust level, not the identity itself.
- A human can change their email, phone number, or employer without breaking their delegation chains.
- The identifier contains no personally identifiable information (GDPR-friendly by design).

#### 5.2.2 Agent and Machine Identity

Agent and machine identifiers are assigned by the registry at creation. They MAY be random (e.g., ULIDs) or human-readable (e.g., `simrelay-prod`). Machine identifiers SHOULD be descriptive when possible.

### 5.3 Attestations

Attestations are verified claims about an entity. They are properties of the identity, not the identity itself. An entity can add, update, or remove attestations without changing its identity or breaking its delegation chains.

```json
{
  "attestations": {
    "email": {
      "address": "sven@simrelay.com",
      "verified_at": "2026-03-10T14:00:00Z"
    },
    "phone": {
      "number": "+49...",
      "verified_at": "2026-03-10T14:01:00Z",
      "method": "reverse_sms"
    },
    "world_id": {
      "proof_type": "orb",
      "uniqueness": "verified",
      "identity_disclosed": false,
      "verified_at": "2026-03-10T14:02:00Z"
    },
    "eid": {
      "country": "DE",
      "verified_at": "2026-03-10T14:03:00Z"
    }
  }
}
```

Attestations serve two purposes:
1. **Trust level elevation:** Each attestation type raises the entity's trust level (see Section 6).
2. **Reachability:** Email and phone attestations provide backup contact channels, but they are NOT used as identifiers.

Removing an attestation (e.g., exercising GDPR right to erasure) MUST NOT invalidate the entity's identity or any existing delegation chains. Only the attestation record is deleted.

### 5.4 Entity Record

Every entity has a record containing at minimum:

```json
{
  "uri": "human://h7x9k2m4p3n8j5w2",
  "type": "human",
  "public_key": {
    "algorithm": "ed25519",
    "key_id": "key_h7x9k_01",
    "public": "base64-encoded-ed25519-public-key"
  },
  "attestations": {
    "email": { "address": "sven@simrelay.com", "verified_at": "2026-03-10T14:00:00Z" },
    "phone": { "number": "+49...", "verified_at": "2026-03-10T14:01:00Z", "method": "reverse_sms" },
    "world_id": { "proof_type": "orb", "uniqueness": "verified", "identity_disclosed": false, "verified_at": "2026-03-10T14:02:00Z" }
  },
  "trust_level": 2,
  "created_at": "2026-03-10T14:00:00Z",
  "registry": "atap.app",
  "discovery": [
    { "method": "registry", "url": "https://atap.app/v1/entities/human/h7x9k2m4p3n8j5w2" }
  ],
  "revocation_url": "https://atap.app/v1/revocations/human/h7x9k2m4p3n8j5w2",
  "recovery": {
    "method": "encrypted_backup",
    "backup_exists": true
  }
}
```

### 5.5 Key Recovery

Since human identity is derived from the keypair, losing the private key means losing the identity. ATAP defines three recovery mechanisms:

#### 5.5.1 Encrypted Backup (Recommended for Launch)

The private key is encrypted with a user-chosen passphrase and stored on the platform. Recovery requires:
1. The passphrase.
2. Re-verification of at least one attestation (email or phone).

The platform MUST NOT store the passphrase or be able to decrypt the backup without user participation. The encryption MUST use a key derivation function (Argon2id) to derive an encryption key from the passphrase.

```json
{
  "recovery_backup": {
    "method": "encrypted_backup",
    "kdf": "argon2id",
    "kdf_params": { "memory": 65536, "iterations": 3, "parallelism": 4 },
    "encrypted_private_key": "base64-encrypted-key",
    "nonce": "base64-nonce",
    "created_at": "2026-03-10T14:00:00Z"
  }
}
```

#### 5.5.2 Multi-Device (Phase 2)

The human installs ATAP on a second device. The first device signs a delegation to the second device's key. If one device is lost, the other can sign a key rotation document attesting to a new primary key.

#### 5.5.3 Social Recovery (Future)

The human designates N trusted humans (minimum 3). To recover, M of N (e.g., 2 of 3) must sign a key rotation attestation. This provides decentralized recovery without depending on the platform.

---

## 6. Trust Levels

### 6.1 Level Definitions

| Level | Name | Verification Required | Typical Use Cases |
|-------|------|----------------------|-------------------|
| 0 | Anonymous | Self-registration only | Inbox, basic signal exchange, testing |
| 1 | Claimed | Human principal verified via email + phone | Service integrations, API access, non-financial actions |
| 2 | Personhood | Human principal verified via proof-of-personhood (e.g., World ID) | Payments, commerce, data access, agent-to-agent trust |
| 3 | Identity | Human principal verified via government-issued ID + organization verified | Regulated transactions, financial services, legal actions |

### 6.2 Trust Level Inheritance

An agent's effective trust level is the minimum of:
1. The trust level of its principal (the human at the root of the delegation chain).
2. The trust level required by any intermediate entity in the chain.

For example, if a human is verified at Level 2 (World ID) and delegates to an agent through a machine that requires Level 1, the agent's effective trust level is 2.

### 6.3 Trust Level Requirements

Services receiving signals MAY specify a minimum trust level. If a signal arrives from an entity whose effective trust level is below the minimum, the service SHOULD reject the signal with a `trust_level_insufficient` error and MAY include the required level in the response to allow the agent to initiate a trust upgrade.

### 6.4 Verification Methods

#### 6.4.1 Level 1: Email + Phone

- Email: Verified via confirmation link or code.
- Phone: Verified via reverse SMS (mobile-originated verification) or standard SMS OTP.

#### 6.4.2 Level 2: Proof-of-Personhood

- World ID: Zero-knowledge proof of unique personhood via the World ID protocol (IDKit v4).
- Other providers: The protocol is extensible; any proof-of-personhood provider that issues cryptographic attestations MAY be used.

The verification record includes:

```json
{
  "method": "world_id",
  "proof_type": "orb",
  "uniqueness": "verified",
  "identity_disclosed": false,
  "verified_at": "2026-03-10T14:32:00Z"
}
```

The `identity_disclosed: false` field indicates that the human proved they are a unique real person without revealing who they are. This enables privacy-preserving delegation.

#### 6.4.3 Level 3: Government ID + Organization

- eID: Government-issued identity document verified through an eIDAS-compliant or equivalent service.
- Organization: Verified through commercial registry (e.g., HRB in Germany), domain ownership, or equivalent.

---

## 7. Cryptographic Primitives

### 7.1 Signing

ATAP uses Ed25519 (EdDSA over Curve25519) for all signatures. Ed25519 is chosen for:

- Small key size (32 bytes public, 64 bytes private).
- Small signature size (64 bytes).
- Fast signing and verification.
- Deterministic signatures (no randomness required).
- Wide library support across all major programming languages.

Every entity has an Ed25519 keypair. The private key MUST remain under the entity's exclusive control. The public key is published in the entity's record.

### 7.2 Encryption

ATAP uses X25519 key exchange with XSalsa20-Poly1305 symmetric encryption (NaCl/libsodium `crypto_box`) for end-to-end encrypted signals.

When a sender wishes to encrypt a signal for a specific recipient:

1. Sender generates an ephemeral X25519 keypair.
2. Sender performs X25519 key exchange between the ephemeral private key and the recipient's long-term X25519 public key.
3. Sender encrypts the signal body using XSalsa20-Poly1305 with a random nonce.
4. Sender includes the ephemeral public key and nonce in the signal's trust block.
5. Recipient performs X25519 key exchange between their long-term private key and the ephemeral public key.
6. Recipient decrypts the signal body.

This ensures the platform operator (e.g., Kette) cannot read encrypted signal bodies. The platform routes based on the unencrypted `route` block only.

### 7.3 Key Derivation

Entities that wish to support encryption MUST publish an X25519 public key in addition to their Ed25519 signing key. The X25519 key MAY be derived from the Ed25519 key using the standard Curve25519 conversion, or MAY be an independent key.

### 7.4 Key Rotation

Entities SHOULD rotate keys periodically. When rotating:

1. Publish the new key with a new `key_id`.
2. Sign a key rotation statement with the old key, attesting to the new key.
3. Keep the old key in the entity record with an `expires_at` field.
4. After expiration, remove the old key.

Delegation documents reference keys by `key_id`, so existing delegations remain valid until they expire or are revoked, even after key rotation.

---

## 8. Signal Format

### 8.1 Overview

A signal is the fundamental unit of communication in ATAP. Signals are JSON objects with four top-level blocks: `route`, `trust`, `signal`, and `context`.

### 8.2 Signal Structure

```json
{
  "v": "1",
  "id": "{signal-id}",
  "ts": "{ISO-8601-timestamp}",

  "route": { ... },
  "trust": { ... },
  "signal": { ... },
  "context": { ... }
}
```

### 8.3 Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `v` | string | REQUIRED | Protocol version. Current: `"1"`. |
| `id` | string | REQUIRED | Globally unique signal identifier. MUST be a `sig_` prefixed opaque string. |
| `ts` | string | REQUIRED | ISO 8601 timestamp of signal creation in UTC. |
| `route` | object | REQUIRED | Routing information. |
| `trust` | object | OPTIONAL | Cryptographic trust block. REQUIRED for Level 1+ entities. |
| `signal` | object | REQUIRED | The payload. |
| `context` | object | OPTIONAL | Platform-level metadata. |

### 8.4 Route Block

```json
{
  "route": {
    "origin": "agent://sender-id",
    "target": "agent://recipient-id",
    "reply_to": "agent://sender-id",
    "channel": "chn_8f3a...",
    "thread": "thr_92fa...",
    "ref": "sig_01HQ3..."
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `origin` | string | REQUIRED | Entity URI of the sender. |
| `target` | string | REQUIRED | Entity URI of the recipient. |
| `reply_to` | string | OPTIONAL | Entity URI where replies should be sent. Defaults to `origin`. |
| `channel` | string | OPTIONAL | Identifier of the inbound channel (e.g., webhook) through which the signal was received. Set by the platform, not the sender. |
| `thread` | string | OPTIONAL | Thread identifier for grouping related signals. |
| `ref` | string | OPTIONAL | Signal ID of a previous signal this is in response to. |

### 8.5 Trust Block

```json
{
  "trust": {
    "scheme": "ed25519",
    "key_id": "key_a1b2c3",
    "sig": "base64-encoded-signature",
    "delegation": "del_7f8a9b",
    "enc": {
      "scheme": "x25519-xsalsa20-poly1305",
      "ephemeral_key": "base64-encoded-public-key",
      "nonce": "base64-encoded-nonce"
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `scheme` | string | REQUIRED | Signing algorithm. MUST be `"ed25519"`. |
| `key_id` | string | REQUIRED | Identifier of the signing key. |
| `sig` | string | REQUIRED | Base64-encoded Ed25519 signature over the canonical form of `route` + `signal` blocks. |
| `delegation` | string | OPTIONAL | Identifier of the delegation document authorizing this signal. |
| `enc` | object | OPTIONAL | Encryption parameters, present only for encrypted signals. |
| `enc.scheme` | string | REQUIRED if `enc` | Encryption algorithm. MUST be `"x25519-xsalsa20-poly1305"`. |
| `enc.ephemeral_key` | string | REQUIRED if `enc` | Base64-encoded ephemeral X25519 public key. |
| `enc.nonce` | string | REQUIRED if `enc` | Base64-encoded 24-byte nonce. |

#### 8.5.1 Signature Computation

The signature is computed over the canonical JSON serialization of the concatenation of the `route` and `signal` blocks:

1. Serialize the `route` block as canonical JSON (keys sorted alphabetically, no whitespace).
2. Serialize the `signal` block as canonical JSON.
3. Concatenate: `canonical(route) + "." + canonical(signal)`.
4. Sign with the sender's Ed25519 private key.
5. Base64-encode the 64-byte signature.

### 8.6 Signal Block

```json
{
  "signal": {
    "type": "application/json",
    "encrypted": false,
    "data": {
      "type": "approval_granted",
      "payload": {
        "request_id": "req_123",
        "approved_by": "human://h7x9k2m4",
        "scopes": ["sms:read", "sms:receive"]
      }
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | REQUIRED | MIME type of the data. Typically `"application/json"`. |
| `encrypted` | boolean | REQUIRED | Whether the `data` field is encrypted. |
| `data` | any | REQUIRED | The payload. If `encrypted` is true, this is a base64-encoded ciphertext string. If false, this is the payload in the declared type. |

The signal block is deliberately opaque to the platform. The platform MUST NOT parse, validate, or modify the `data` field. This ensures the protocol can carry any payload type without versioning conflicts.

### 8.7 Context Block

```json
{
  "context": {
    "source": "webhook",
    "idempotency": "idk_xyz789",
    "tags": ["auth", "onboarding"],
    "ttl": 86400,
    "priority": 1
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | OPTIONAL | Origin type: `"webhook"`, `"agent"`, `"machine"`, or `"system"`. |
| `idempotency` | string | OPTIONAL | Idempotency key. Platforms MUST deduplicate signals with the same idempotency key within a 24-hour window. |
| `tags` | array | OPTIONAL | Array of string labels for filtering. |
| `ttl` | integer | OPTIONAL | Time-to-live in seconds. The platform SHOULD discard the signal after TTL expires. 0 means no expiration. |
| `priority` | integer | OPTIONAL | Priority level. 0 = lowest, 9 = highest. Default: 1. |

---

## 9. Delegation Documents

### 9.1 Overview

A delegation document is the core trust primitive of ATAP. It is a cryptographically signed JSON object that grants scoped authority from a principal to a delegate, optionally through intermediate entities.

Delegation documents are portable: the delegate carries the document and presents it to any party that needs to verify authorization. Verification requires only the public keys of the entities in the chain, not access to a central server.

### 9.2 Delegation Structure

```json
{
  "atap_delegation": "1",
  "id": "del_7f8a9b",
  "created_at": "2026-03-10T14:32:00Z",

  "principal": "human://h7x9k2m4",
  "delegate": "agent://a1b2c3d4e5f6",
  "via": ["machine://simrelay-prod"],

  "scope": {
    "actions": ["sms:read", "sms:receive", "purchase:execute"],
    "spend_limit": {
      "amount": 500,
      "currency": "EUR",
      "period": "monthly"
    },
    "data_classes": ["non-sensitive"],
    "expires": "2026-06-10T00:00:00Z"
  },

  "constraints": {
    "geo": ["EU"],
    "time_window": {
      "start": "08:00",
      "end": "22:00",
      "timezone": "Europe/Berlin"
    },
    "confirm_above": {
      "amount": 100,
      "currency": "EUR"
    }
  },

  "human_verification": {
    "level": 2,
    "methods": [
      { "type": "email", "verified_at": "2026-03-10T14:00:00Z" },
      { "type": "phone", "verified_at": "2026-03-10T14:01:00Z", "method": "reverse_sms" },
      { "type": "world_id", "proof_type": "orb", "uniqueness": "verified", "identity_disclosed": false, "verified_at": "2026-03-10T14:02:00Z" }
    ]
  },

  "signatures": [
    {
      "entity": "human://h7x9k2m4",
      "key_id": "key_sven_01",
      "sig": "base64-encoded-signature",
      "signed_at": "2026-03-10T14:32:00Z"
    },
    {
      "entity": "machine://simrelay-prod",
      "key_id": "key_simrelay_01",
      "sig": "base64-encoded-signature",
      "signed_at": "2026-03-10T14:32:01Z"
    }
  ]
}
```

### 9.3 Field Definitions

#### 9.3.1 Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `atap_delegation` | string | REQUIRED | Document type and version. MUST be `"1"`. |
| `id` | string | REQUIRED | Globally unique delegation identifier. `del_` prefix. |
| `created_at` | string | REQUIRED | ISO 8601 creation timestamp. |
| `principal` | string | REQUIRED | Entity URI of the root authority. Typically `human://`. |
| `delegate` | string | REQUIRED | Entity URI of the entity receiving authority. |
| `via` | array | OPTIONAL | Ordered array of intermediate entity URIs. Empty array or omitted for direct delegation. |
| `scope` | object | REQUIRED | What the delegate is authorized to do. |
| `constraints` | object | OPTIONAL | Additional restrictions on the delegation. |
| `human_verification` | object | OPTIONAL | Verification details of the principal. |
| `signatures` | array | REQUIRED | Array of signatures from each entity in the chain. |

#### 9.3.2 Scope Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `actions` | array | REQUIRED | Array of action strings the delegate may perform. Action strings follow the pattern `domain:action` (e.g., `sms:read`, `purchase:execute`, `booking:create`). The wildcard `*:*` grants all actions (use with caution). |
| `spend_limit` | object | OPTIONAL | Financial spending limit. |
| `spend_limit.amount` | number | REQUIRED if `spend_limit` | Maximum amount. |
| `spend_limit.currency` | string | REQUIRED if `spend_limit` | ISO 4217 currency code. |
| `spend_limit.period` | string | REQUIRED if `spend_limit` | One of: `"per_transaction"`, `"daily"`, `"weekly"`, `"monthly"`. |
| `data_classes` | array | OPTIONAL | Data classification levels the delegate may access. |
| `expires` | string | REQUIRED | ISO 8601 expiration timestamp. Delegations MUST have an expiration. |

#### 9.3.3 Constraints Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `geo` | array | OPTIONAL | ISO 3166-1 alpha-2 country codes or region identifiers where the delegation is valid. |
| `time_window` | object | OPTIONAL | Time-of-day restriction. |
| `time_window.start` | string | REQUIRED if `time_window` | Start time in HH:MM format. |
| `time_window.end` | string | REQUIRED if `time_window` | End time in HH:MM format. |
| `time_window.timezone` | string | REQUIRED if `time_window` | IANA timezone identifier. |
| `confirm_above` | object | OPTIONAL | Require re-confirmation from principal for actions above this threshold. |
| `ip_allowlist` | array | OPTIONAL | IP addresses or CIDR ranges from which the delegate may act. |
| `max_actions_per_period` | object | OPTIONAL | Rate limit on delegated actions. |

#### 9.3.4 Signatures Array

Each entry in the `signatures` array represents one link in the delegation chain.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `entity` | string | REQUIRED | Entity URI of the signer. |
| `key_id` | string | REQUIRED | Identifier of the signing key. |
| `sig` | string | REQUIRED | Base64-encoded Ed25519 signature. |
| `signed_at` | string | REQUIRED | ISO 8601 timestamp of signing. |

#### 9.3.5 Signature Computation for Delegations

Each entity in the chain signs the canonical JSON of the delegation document excluding the `signatures` array:

1. Create a copy of the delegation document without the `signatures` field.
2. Serialize as canonical JSON (sorted keys, no whitespace).
3. Sign with the entity's Ed25519 private key.
4. Append the signature entry to the `signatures` array.

The principal signs first. Each subsequent entity in the `via` chain signs next, in order. This creates an ordered chain of attestations.

### 9.4 Delegation Chain Verification

To verify a delegation chain, a verifier MUST:

1. Check that `atap_delegation` is a supported version.
2. Check that `scope.expires` has not passed.
3. Check that the `signatures` array contains entries for the `principal` and all entities in `via`.
4. For each signature, retrieve the signer's public key (via registry, DNS, or well-known endpoint).
5. Verify each signature against the canonical delegation document (excluding `signatures`).
6. Verify that the `principal` is the first signer.
7. Verify that entities in `via` are signed in order.
8. Check the revocation status of each entity in the chain (see Section 15).
9. Verify that the requested action falls within `scope.actions`.
10. Verify that any `constraints` are satisfied (geo, time, amount).

If all checks pass, the delegation is valid. If any check fails, the verifier MUST reject the delegation and SHOULD indicate which check failed.

---

## 10. Inbox and Signal Delivery

### 10.1 Inbox

Every entity of type `agent://` or `machine://` receives an inbox upon registration. An inbox is a durable, ordered queue of signals addressed to that entity.

Properties of an inbox:

- **Durable:** Signals persist even if the entity is offline.
- **Ordered:** Signals are ordered by arrival time.
- **Addressable:** The inbox is identified by the entity's URI.
- **Multi-channel:** Multiple inbound channels (webhooks) can deliver to the same inbox.

Entities of type `org://` do NOT have inboxes. Organizations serve as namespaces and trust umbrellas.

Entities of type `human://` DO have inboxes. The mobile app is the human's ATAP client — it renders incoming signals as approval cards, messages, and notifications. Instructions from humans to agents are sent as signed signals from the human's inbox. This ensures that all human-agent communication is cryptographically verified, preventing impersonation attacks through unsigned channels.

### 10.2 Delivery Tiers

ATAP defines three delivery tiers. The entity chooses its preferred tier at registration and MAY change it at any time.

#### 10.2.1 Tier 1: SSE (Server-Sent Events) — Recommended

The entity maintains a persistent HTTP connection to the platform. Signals are pushed as SSE events in real-time.

```
GET /v1/inbox/{entity-id}/stream HTTP/1.1
Accept: text/event-stream
Authorization: Bearer {entity-token}
Last-Event-ID: sig_01HQ3K9X7V
```

Response:

```
event: signal
id: sig_01HQ3K9X8W
data: {"v":"1","id":"sig_01HQ3K9X8W","ts":"2026-03-10T14:33:00Z","route":{...},"signal":{...}}

event: signal
id: sig_01HQ3K9X9X
data: {"v":"1","id":"sig_01HQ3K9X9X","ts":"2026-03-10T14:34:00Z","route":{...},"signal":{...}}
```

When the connection drops and the entity reconnects with `Last-Event-ID`, the platform MUST replay all signals after that ID before switching to live delivery.

SSE is RECOMMENDED because:
- It is unidirectional (server-to-client), matching the inbox delivery pattern.
- It works over standard HTTP through all proxies and firewalls.
- It has built-in reconnection with `Last-Event-ID` for durable delivery.
- It aligns with MCP, which uses SSE for server-to-client streaming.

#### 10.2.2 Tier 2: Webhook Push

The entity registers a callback URL. The platform POSTs signals to it.

```
POST {webhook-url} HTTP/1.1
Content-Type: application/json
X-ATAP-Signature: {ed25519-signature-of-body}
X-ATAP-Signal-ID: sig_01HQ3K9X8W

{signal JSON}
```

The entity MUST respond with HTTP 200 to acknowledge receipt. If the platform receives no acknowledgment, it MUST retry with exponential backoff (1s, 5s, 30s, 5m, 30m) up to 5 attempts before marking the signal as undeliverable.

#### 10.2.3 Tier 3: Polling

The entity periodically requests new signals.

```
GET /v1/inbox/{entity-id}?after={last-seen-id}&limit=50 HTTP/1.1
Authorization: Bearer {entity-token}
```

Response:

```json
{
  "signals": [ ... ],
  "has_more": false,
  "cursor": "sig_01HQ3K9X9X"
}
```

Polling is a fallback for environments where persistent connections are not possible (serverless, cron jobs).

---

## 11. Channels

### 11.1 Overview

A channel is a unique inbound pathway to an entity's inbox. Channels allow an entity to give a different ingress point to each external service, enabling contextual routing, per-service revocation, and audit trails.

### 11.2 Channel Creation

```
POST /v1/entities/{entity-id}/channels HTTP/1.1
Authorization: Bearer {entity-token}
Content-Type: application/json

{
  "label": "simrelay-notifications",
  "tags": ["auth", "sms"],
  "expires": "2026-06-10T00:00:00Z"
}
```

Response:

```json
{
  "id": "chn_8f3a9b2c",
  "webhook_url": "https://kette.ai/v1/channels/chn_8f3a9b2c/signals",
  "label": "simrelay-notifications",
  "tags": ["auth", "sms"],
  "created_at": "2026-03-10T14:32:00Z",
  "expires": "2026-06-10T00:00:00Z"
}
```

### 11.3 Channel Properties

- **Unique URL:** Each channel has a unique webhook URL. External services POST signals to this URL.
- **Labeling:** Channels have a human-readable label and tags for organization.
- **Revocable:** Revoking a channel invalidates its webhook URL without affecting other channels.
- **Auditable:** The `channel` field in the signal's `route` block records which channel received it.
- **Expirable:** Channels may have an expiration date.

### 11.4 External Service Integration

When an agent gives its channel URL to an external service (e.g., SIMRelay for SMS notifications):

1. The external service POSTs signals to the channel's webhook URL.
2. The platform validates the signal, stamps it with the channel ID, and enqueues it in the entity's inbox.
3. The entity receives the signal via its preferred delivery tier (SSE, webhook, or polling).
4. The `route.channel` field tells the entity which service the signal came from.

---

## 12. Claim Flow

### 12.1 Overview

The claim flow is the process by which a human asserts principal authority over an agent. This elevates the agent from Trust Level 0 (anonymous) to Level 1 or higher.

Critically, the agent initiates the claim flow, not the human. This ensures zero-friction agent onboarding.

### 12.2 Flow Steps

#### Step 1: Agent Requests Claim

```
POST /v1/claims HTTP/1.1
Authorization: Bearer {agent-token}
Content-Type: application/json

{
  "agent": "agent://a1b2c3d4e5f6",
  "requested_scopes": ["sms:read", "sms:receive"],
  "context": {
    "description": "Travel booking assistant",
    "machine": "machine://simrelay-prod"
  }
}
```

Response:

```json
{
  "claim_id": "clm_xyz789",
  "claim_url": "https://kette.ai/claim/clm_xyz789",
  "claim_code": "ATAP-7X9K-2M4P",
  "expires": "2026-03-11T14:32:00Z",
  "status": "pending"
}
```

#### Step 2: Agent Presents Claim to Human

The agent presents the `claim_url` or `claim_code` to the human through any available channel: chat message, email, terminal output, QR code, etc.

#### Step 3: Human Approves via Mobile App

The human opens the claim URL or enters the claim code in the Kette mobile app. The app displays:

- Agent details (name, description, machine association).
- Requested scopes.
- The machine's branded approval template (if registered).

The human reviews, optionally adjusts scopes, authenticates via biometric (Face ID / fingerprint), and approves.

#### Step 4: Delegation is Minted

The platform creates a delegation document, signs it with the human's key (held on device), and stores it.

#### Step 5: Agent Receives Confirmation

The platform sends a `claim_approved` signal to the agent's inbox:

```json
{
  "signal": {
    "type": "application/json",
    "encrypted": false,
    "data": {
      "type": "claim_approved",
      "claim_id": "clm_xyz789",
      "delegation_id": "del_7f8a9b",
      "trust_level": 2,
      "scopes": ["sms:read", "sms:receive"]
    }
  }
}
```

### 12.3 Claim Expiration

Unclaimed claims expire after a configurable period (default: 24 hours). Expired claims MUST be deleted.

---

## 13. Branded Approval Templates

### 13.1 Overview

Machines MAY register branded approval templates with the platform. When an agent triggers an approval flow involving that machine, the human sees a branded, structured consent screen rendered from the template.

### 13.2 Template Structure

```json
{
  "machine": "machine://lufthansa",
  "template_id": "tpl_flight_booking",
  "version": 1,

  "brand": {
    "name": "Lufthansa",
    "logo_url": "https://lufthansa.com/kette/logo.svg",
    "colors": {
      "primary": "#05164d",
      "accent": "#ffc72c",
      "background": "#ffffff"
    },
    "verified_domain": "lufthansa.com"
  },

  "approval_schema": {
    "type": "flight_booking",
    "title": "Flight Booking Approval",
    "description": "Your agent wants to book a flight",
    "display_fields": [
      { "key": "route", "label": "Route", "type": "text", "format": "{{origin}} → {{destination}}" },
      { "key": "dates", "label": "Travel Dates", "type": "date_range" },
      { "key": "passengers", "label": "Passengers", "type": "list" },
      { "key": "cabin", "label": "Cabin Class", "type": "text" },
      { "key": "max_price", "label": "Budget Limit", "type": "currency" }
    ],
    "required_trust_level": 2,
    "scopes_requested": ["booking:create", "payment:execute"]
  }
}
```

### 13.3 Template Rendering

When an approval is triggered, the platform:

1. Matches the machine in the request to its registered templates.
2. Extracts data fields from the approval request payload.
3. Renders the template with the machine's brand assets and the extracted data.
4. Displays the result on the human's mobile device as a native approval card.

### 13.4 Template Verification

The `verified_domain` field indicates that the platform has verified the machine's control of the domain via DNS TXT record or well-known endpoint. Templates from verified domains display a verification badge.

### 13.5 Open Template Format

Templates are defined in JSON and follow an open schema. Third-party applications (wallets, alternative platforms) MAY render ATAP approval templates using the same schema. This ensures the approval experience is not locked to a single platform implementation.

---

## 14. Verification

### 14.1 Verification API

Any party MAY verify a delegation document by submitting it to a verification endpoint:

```
POST /v1/verify HTTP/1.1
Content-Type: application/json

{
  "delegation": { ... },
  "action": "purchase:execute",
  "amount": { "value": 250, "currency": "EUR" },
  "timestamp": "2026-03-10T15:00:00Z"
}
```

Response:

```json
{
  "valid": true,
  "trust_level": 2,
  "chain": [
    { "entity": "human://h7x9k2m4", "verified": true, "key_source": "registry" },
    { "entity": "machine://simrelay-prod", "verified": true, "key_source": "dns" }
  ],
  "scope_check": {
    "action_allowed": true,
    "spend_limit_ok": true,
    "constraints_met": true
  },
  "warnings": []
}
```

### 14.2 Offline Verification

Verification MAY be performed entirely offline if the verifier has cached or pre-fetched the public keys of all entities in the delegation chain. The verification algorithm is defined in Section 9.4.

### 14.3 Verification Caching

Verifiers SHOULD cache public keys and revocation lists with appropriate TTLs to reduce network requests. Recommended TTLs:

- Public keys: 1 hour.
- Revocation lists: 5 minutes.
- Delegation document validity: check `expires` field.

---

## 15. Revocation

### 15.1 Delegation Revocation

A principal or any intermediate entity in a delegation chain MAY revoke the delegation at any time. Revocation is immediate and permanent.

```
POST /v1/delegations/{delegation-id}/revoke HTTP/1.1
Authorization: Bearer {entity-token}
Content-Type: application/json

{
  "reason": "Agent compromised",
  "revoked_by": "human://h7x9k2m4"
}
```

### 15.2 Cascading Revocation

Revoking an intermediate entity invalidates all delegations that pass through it. For example, revoking `machine://simrelay-prod` invalidates every delegation where it appears in the `via` chain. The platform MUST maintain a reverse index to identify affected delegations.

### 15.3 Revocation Lists

Each entity publishes a signed revocation list at a known URL:

```json
{
  "entity": "human://h7x9k2m4",
  "revocations": [
    {
      "delegation_id": "del_7f8a9b",
      "revoked_at": "2026-03-11T10:00:00Z",
      "reason": "Agent compromised"
    }
  ],
  "published_at": "2026-03-11T10:01:00Z",
  "signature": {
    "key_id": "key_sven_01",
    "sig": "base64-encoded-signature"
  }
}
```

Revocation lists are signed by the entity that published them. Verifiers MUST check revocation lists as part of delegation verification.

### 15.4 Revocation Discovery

Revocation lists are discovered via:

1. The `revocation_url` field in the entity's record.
2. A well-known endpoint: `https://{domain}/.well-known/atap-revocations.json`.
3. The entity's registry.

### 15.5 Revocation Transparency (Optional)

Platforms MAY operate a revocation transparency log: an append-only, publicly auditable log of all revocations. This provides stronger guarantees than individual revocation lists by allowing any party to verify that a revocation was published at a specific time.

The transparency log format follows Certificate Transparency (RFC 6962) principles adapted for ATAP:

- Each entry is a signed revocation statement.
- The log is append-only.
- The log publishes a signed tree head at regular intervals.
- Any party may operate a log server.

---

## 16. Federation and Key Discovery

### 16.1 Principle

ATAP is designed so that no single operator controls the trust graph. Public keys, entity records, and revocation lists can be discovered through multiple independent mechanisms.

### 16.2 Discovery Methods

#### 16.2.1 Registry Lookup

A registry is an ATAP-compliant service that stores entity records and serves them via HTTP.

```
GET /v1/entities/{type}/{identifier} HTTP/1.1
```

Multiple registries MAY coexist. Entities MAY be registered with multiple registries. The `registry` field in the entity record indicates which registry issued the entity.

#### 16.2.2 DNS TXT Records

Entities MAY publish their public keys as DNS TXT records:

```
{identifier}._atep.{domain} IN TXT "v=atap1; k=ed25519; p={base64-public-key}; kid={key-id}"
```

Example:

```
simrelay-prod._atep.simrelay.com IN TXT "v=atap1; k=ed25519; p=MCowBQ...==; kid=key_simrelay_01"
```

This allows machines and organizations to host their own keys under their own domains, removing dependency on any registry.

#### 16.2.3 Well-Known Endpoint

Entities associated with a domain MAY publish their ATAP records at:

```
https://{domain}/.well-known/atap.json
```

```json
{
  "atap_discovery": "1",
  "entities": [
    {
      "uri": "machine://simrelay-prod",
      "public_key": {
        "algorithm": "ed25519",
        "key_id": "key_simrelay_01",
        "public": "base64-encoded-key"
      },
      "revocation_url": "https://simrelay.com/.well-known/atap-revocations.json"
    }
  ]
}
```

### 16.3 Discovery Priority

Verifiers SHOULD resolve public keys in the following priority order:

1. **Locally cached keys** (if cache is fresh).
2. **Well-known endpoint** (if the entity is associated with a known domain).
3. **DNS TXT record** (if the entity has a DNS-discoverable domain).
4. **Registry lookup** (fallback to the registry that issued the entity).

### 16.4 Registry Federation

Registries MAY synchronize entity records between themselves. The synchronization protocol is out of scope for ATAP v1 but SHOULD be addressed in a future version.

### 16.5 Entity Portability

Entities MUST be able to migrate from one registry to another without losing their identity. Migration involves:

1. Publishing the entity's public key on the new registry (or via DNS / well-known).
2. Signing a migration statement with the entity's private key, attesting to the new registry.
3. Updating delegation documents to reference the new discovery endpoints.

The entity URI does NOT change during migration. Only the discovery mechanism changes.

---

## 17. Security Considerations

### 17.1 Key Management

- Private keys MUST be stored securely. For humans, this means on-device storage (mobile secure enclave). For agents, this means encrypted storage with access controls.
- Key compromise requires immediate revocation of all delegations signed with the compromised key, followed by key rotation.
- Implementations SHOULD support hardware-backed key storage where available.

### 17.2 Replay Protection

- Signal IDs MUST be globally unique. Verifiers SHOULD reject signals with previously seen IDs.
- The `ts` field provides temporal context. Verifiers MAY reject signals older than a configurable threshold.
- The `context.idempotency` field provides application-level replay protection.

### 17.3 Delegation Scope Minimization

- Principals SHOULD grant the minimum scope necessary.
- Delegations MUST have an expiration (`scope.expires`).
- Implementations SHOULD warn principals when granting broad scopes (e.g., `*:*`).

### 17.4 Privacy

- Human entity URIs MUST NOT contain personally identifiable information.
- World ID verification uses zero-knowledge proofs: the verifier learns that the principal is a unique human without learning who they are.
- The platform SHOULD NOT log signal payloads (only routing metadata).
- End-to-end encryption (Section 7.2) ensures the platform cannot read signal contents.

### 17.5 Denial of Service

- Platforms SHOULD rate-limit signal delivery, entity registration, and channel creation.
- Platforms SHOULD limit inbox size and enforce TTL-based expiration.
- Webhook delivery SHOULD use exponential backoff with a maximum retry count.

### 17.6 Quantum Readiness

ATAP v1 uses Ed25519 and X25519, which are not quantum-safe. When NIST post-quantum signature standards (ML-DSA) reach sufficient library maturity and performance, ATAP SHOULD define a v2 cryptographic suite. The `trust.scheme` field enables this transition without breaking the signal format.

---

## 18. Relationship to Other Protocols

### 18.1 MCP (Model Context Protocol)

MCP defines how agents connect to tools. ATAP complements MCP by providing identity for the agents using those tools. An ATAP-aware MCP server can verify delegation chains before granting tool access.

Integration point: MCP tool servers MAY require an ATAP delegation document in the tool request, verifying that the calling agent is authorized to use the tool.

### 18.2 A2A (Agent-to-Agent Protocol)

A2A defines how agents communicate with each other using Agent Cards. ATAP enhances A2A by adding verifiable delegation chains to Agent Cards, so agents can verify not just what another agent can do, but who authorized it.

Integration point: A2A Agent Cards MAY include an `atap_delegation` field referencing the agent's delegation document.

### 18.3 AP2 (Agent Payments Protocol)

AP2 defines mandates for agent-initiated payments. ATAP can serve as the underlying identity and delegation layer for AP2, replacing AP2's proprietary mandate system with ATAP delegation documents.

Integration point: AP2 Intent Mandates can be expressed as ATAP delegation documents with `purchase:execute` scope and `spend_limit` constraints.

### 18.4 World ID

World ID provides proof-of-personhood via zero-knowledge proofs. ATAP uses World ID as one verification method for Trust Level 2.

Integration point: The `human_verification` field in delegation documents records World ID attestations.

---

## 19. IANA Considerations

### 19.1 URI Scheme Registration

This document requests registration of the following URI schemes:

- `agent://` — Identifies an AI agent entity.
- `machine://` — Identifies a persistent application or service entity.
- `human://` — Identifies a human trust anchor entity.
- `org://` — Identifies an organizational entity.

### 19.2 Well-Known URI Registration

This document requests registration of the following well-known URIs:

- `/.well-known/atap.json` — ATAP entity discovery.
- `/.well-known/atap-revocations.json` — ATAP revocation list.

### 19.3 Media Type

This document defines the media type `application/atap+json` for ATAP signal and delegation documents.

---

## Appendix A: Full Signal Example

```json
{
  "v": "1",
  "id": "sig_01HQ3K9X8W",
  "ts": "2026-03-10T14:33:00Z",

  "route": {
    "origin": "machine://lufthansa",
    "target": "agent://a1b2c3d4e5f6",
    "channel": "chn_8f3a9b2c",
    "thread": "thr_booking_92fa",
    "ref": "sig_01HQ3K9X7V"
  },

  "trust": {
    "scheme": "ed25519",
    "key_id": "key_lufthansa_01",
    "sig": "Tm90IGEgcmVhbCBzaWduYXR1cmUsIGJ1dCB5b3Uga25ldyB0aGF0"
  },

  "signal": {
    "type": "application/json",
    "encrypted": false,
    "data": {
      "type": "booking_confirmed",
      "payload": {
        "booking_ref": "LH-7X9K2M",
        "route": "FRA → NRT",
        "dates": { "depart": "2026-06-15", "return": "2026-06-22" },
        "cabin": "Business",
        "total": { "amount": 2840, "currency": "EUR" }
      }
    }
  },

  "context": {
    "source": "machine",
    "idempotency": "idk_lh_booking_7x9k",
    "tags": ["travel", "booking", "confirmed"],
    "ttl": 604800
  }
}
```

---

## Appendix B: Full Delegation Example

```json
{
  "atap_delegation": "1",
  "id": "del_7f8a9b",
  "created_at": "2026-03-10T14:32:00Z",

  "principal": "human://h7x9k2m4",
  "delegate": "agent://a1b2c3d4e5f6",
  "via": ["machine://simrelay-prod"],

  "scope": {
    "actions": ["sms:read", "sms:receive", "purchase:execute"],
    "spend_limit": {
      "amount": 500,
      "currency": "EUR",
      "period": "monthly"
    },
    "data_classes": ["non-sensitive"],
    "expires": "2026-06-10T00:00:00Z"
  },

  "constraints": {
    "geo": ["DE", "NL", "PL"],
    "time_window": {
      "start": "08:00",
      "end": "22:00",
      "timezone": "Europe/Berlin"
    },
    "confirm_above": {
      "amount": 100,
      "currency": "EUR"
    }
  },

  "human_verification": {
    "level": 2,
    "methods": [
      {
        "type": "email",
        "verified_at": "2026-03-10T14:00:00Z"
      },
      {
        "type": "phone",
        "verified_at": "2026-03-10T14:01:00Z",
        "method": "reverse_sms"
      },
      {
        "type": "world_id",
        "proof_type": "orb",
        "uniqueness": "verified",
        "identity_disclosed": false,
        "verified_at": "2026-03-10T14:02:00Z"
      }
    ]
  },

  "signatures": [
    {
      "entity": "human://h7x9k2m4",
      "key_id": "key_sven_01",
      "sig": "SHVtYW4gc2lnbmF0dXJlIHBsYWNlaG9sZGVyIC0gbm90IHJlYWw=",
      "signed_at": "2026-03-10T14:32:00Z"
    },
    {
      "entity": "machine://simrelay-prod",
      "key_id": "key_simrelay_01",
      "sig": "TWFjaGluZSBzaWduYXR1cmUgcGxhY2Vob2xkZXIgLSBub3QgcmVhbA==",
      "signed_at": "2026-03-10T14:32:01Z"
    }
  ]
}
```

---

## Appendix C: JSON Schemas

JSON Schemas for all ATAP data structures are published at:

```
https://atap.dev/schemas/v1/signal.json
https://atap.dev/schemas/v1/delegation.json
https://atap.dev/schemas/v1/entity-record.json
https://atap.dev/schemas/v1/revocation-list.json
https://atap.dev/schemas/v1/approval-template.json
https://atap.dev/schemas/v1/claim-request.json
```

---

## Acknowledgments

ATAP builds on ideas from established protocols and systems including X.509 certificate chains, DNS, Certificate Transparency (RFC 6962), OAuth 2.0, Verifiable Credentials (W3C), and the World ID protocol. The signal delivery mechanism draws from Server-Sent Events (W3C) and the Model Context Protocol (Anthropic).

---

## References

- **RFC 2119** — Key words for use in RFCs to Indicate Requirement Levels
- **RFC 6962** — Certificate Transparency
- **RFC 8032** — Edwards-Curve Digital Signature Algorithm (Ed25519)
- **RFC 8037** — CFRG Elliptic Curve Diffie-Hellman (X25519)
- **W3C SSE** — Server-Sent Events specification
- **W3C VC** — Verifiable Credentials Data Model
- **MCP** — Model Context Protocol (Anthropic)
- **A2A** — Agent-to-Agent Protocol (Google / Linux Foundation)
- **AP2** — Agent Payments Protocol (Google)
- **World ID** — Proof-of-Personhood Protocol (Tools for Humanity)

---

*This document is a draft and subject to change. Feedback and contributions are welcome.*

*Copyright 2026 SIMRelay GmbH. Licensed under Apache 2.0.*
