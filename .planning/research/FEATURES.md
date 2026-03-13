# Feature Landscape

**Domain:** DID/DIDComm/VC protocol implementation for AI agent trust and authorization
**Researched:** 2026-03-13
**Confidence:** MEDIUM (based on W3C specs, DIF specs, and ecosystem research; spec file not directly readable)

## Table Stakes

Features that any DID/DIDComm/VC protocol implementation must have for spec compliance and ecosystem credibility. Missing any of these means the protocol is not interoperable.

### Identity (did:web)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| DID Document hosting at `/.well-known/did.json` | W3C did:web spec requirement; the fundamental resolution mechanism | Low | Static JSON file served at well-known path. Must include `verificationMethod`, `authentication`, `assertionMethod`, `keyAgreement` sections |
| DID Document hosting at `/entities/{id}/did.json` | did:web spec for path-based DIDs (`did:web:domain:entities:id`) | Low | Same format, different path. Needed for per-entity DIDs |
| DID resolution (outbound) | Must resolve other entities' DIDs to verify signatures, establish connections | Medium | HTTP fetch + JSON-LD parsing. Must handle DNS failures, TLS validation, caching |
| Ed25519 verification methods in DID Documents | Spec mandates Ed25519 signing; DID Doc must advertise public keys | Low | Already have Ed25519 key generation in existing codebase |
| X25519 key agreement in DID Documents | Required for DIDComm authenticated encryption | Low | Derive X25519 from Ed25519 keys (standard conversion) |
| Entity registration with DID creation | Every entity (human, agent, machine, org) needs a DID | Medium | Must generate keypair, create DID Document, store, and serve. Four entity types with different registration flows |

### Messaging (DIDComm v2.1)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| DIDComm plaintext message format | Base message envelope per DIF spec: `id`, `type`, `from`, `to`, `body`, `created_time` | Low | JSON structure, well-defined |
| DIDComm signed messages (JWS) | Non-repudiation for approvals; parties must prove they signed | Medium | JWS with detached payload (RFC 7515 + RFC 7797). Already have JCS canonicalization |
| DIDComm authenticated encryption (authcrypt) | Privacy + authentication for message content | High | X25519 ECDH + XSalsa20-Poly1305 or XChaCha20-Poly1305. Requires sender+recipient key exchange |
| DIDComm anonymous encryption (anoncrypt) | Privacy without sender authentication; needed for forward messages to mediators | Medium | X25519 ECDH, one-time sender key |
| Message routing via `forward` protocol | Core DIDComm routing: wrap messages for delivery through intermediaries | Medium | Nested encryption envelopes. The server acts as mediator for its hosted entities |
| Service endpoints in DID Documents | How senders discover where to deliver messages | Low | `serviceEndpoint` field in DID Doc pointing to server's DIDComm endpoint |

### Authentication (OAuth 2.1 + DPoP)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| OAuth 2.1 token endpoint | Industry standard API auth; spec mandates it over custom auth | Medium | Replace existing `atap_` bearer tokens with proper OAuth flow |
| DPoP proof validation (RFC 9449) | Sender-constrained tokens; prevents token theft/replay | Medium | Validate DPoP JWT: check `typ`, `htm`, `htu`, verify signature against `jwk` header, bind to access token via `jkt` confirmation |
| DPoP key binding in access tokens | Access tokens must include `cnf.jkt` claim linking to client's DPoP key | Low | SHA-256 thumbprint of client's public key embedded in token |
| Token introspection or self-contained tokens | Verifiers need to validate tokens without calling back to issuer | Medium | JWT access tokens with standard claims, or introspection endpoint |

### Credentials (W3C Verifiable Credentials 2.0)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| VC issuance (server as issuer) | Server issues credentials for email, phone, personhood attestations | Medium | VC-JOSE-COSE format (JWT-based). Must include `issuer`, `credentialSubject`, `issuanceDate`, `credentialSchema` |
| VC verification | Verify credentials presented by entities (resolve issuer DID, check signature, check expiry) | Medium | Signature verification + DID resolution + schema validation + status check |
| VC storage per entity | Entities accumulate credentials that determine trust level | Low | JSONB column or separate table, encrypted at rest |
| Trust level derivation from credentials | Levels 0-3 based on which VCs an entity holds | Low | Business logic mapping credential types to trust levels |
| Credential schemas for core types | Email, phone, personhood, identity, principal, org membership | Low | JSON Schema definitions, static but must match spec exactly |

### Server Discovery

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `/.well-known/atap.json` endpoint | Protocol-specific server discovery; how clients find capabilities | Low | Static JSON: server DID, supported protocols, endpoints, version |
| Server DID Document | Server itself has a DID for signing server-issued credentials | Low | One DID Doc for the server entity |

### Approvals (Core Protocol)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Two-party approval flow (from + to) | Minimum viable approval: requester asks, approver decides | High | Approval creation, DIDComm delivery, signature collection, state machine (requested -> approved/declined/expired) |
| Three-party approval flow (from + via + to) | The differentiating ATAP flow: agent requests via system to human | High | Three independent signatures, template injection by `via`, more complex state machine |
| Approval state machine | Lifecycle: requested -> approved/declined/expired/rejected -> consumed/revoked | Medium | Must be strict, no invalid transitions, all transitions auditable |
| JWS signatures on approvals | Each party signs independently with detached payload JWS | Medium | Canonical JSON (JCS RFC 8785) + JWS. Existing JCS code reusable |
| Approval expiry (TTL) | Approvals must not live forever; security requirement | Low | Expiry timestamp, background cleanup or lazy evaluation |

## Differentiators

Features that set ATAP apart from generic DID/VC platforms. These are not expected by the DID/VC ecosystem at large but are core to ATAP's value proposition.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Multi-signature approval model (ATAP-specific) | No other protocol defines a three-party cryptographic approval with independent signatures from requester, mediating system, and human approver | High | This IS the protocol. The three-party flow with template injection is novel |
| Signed approval templates with brand rendering | `via` systems provide branded UI templates (logo, colors, action descriptions) signed with JWS so the mobile client can verify template authenticity | High | W3C has a VC Render Method spec (Working Draft as of 2025), but ATAP's approach of system-signed templates for approval UIs is unique. Must render safely on mobile |
| Mobile biometric approval signing | Human approvers use biometric auth (FaceID/fingerprint) to unlock private keys and sign approvals on-device | Medium | Flutter + platform secure enclave integration. Keys never leave device. Differentiator because most VC wallets don't do multi-party approval signing |
| Chained approvals (parent/child) | An approval can reference a parent approval, creating auditable delegation chains | Medium | Graph of approvals with parent references. Enables "I approved this because my manager approved that" |
| Persistent approvals with revocation | Approvals can be long-lived (not one-shot) and explicitly revoked | Medium | Unlike typical VC presentations which are point-in-time, persistent approvals act as ongoing authorization with revocation capability |
| Organization delegate routing | Approvals sent to an org DID get routed to the appropriate human delegate | Medium | Org -> member mapping + routing rules. Not standard DIDComm; ATAP-specific |
| Trust level system (0-3) | Quantified trust derived from verifiable credentials, used in approval policy decisions | Low | Simple but powerful: trust levels gate what actions entities can perform. Most DID systems don't have explicit trust scoring |
| Server trust assessment | Evaluate other ATAP servers via WebPKI + DNSSEC + audit VCs | Medium | Novel: servers assessing each other's trustworthiness using verifiable credentials about their infrastructure |
| Crypto-shredding for GDPR | Delete per-entity encryption key to make all their VC data unrecoverable | Medium | Per-entity DEK wrapping PII fields. Delete DEK = effective erasure. Well-established pattern but novel in DID/VC context |

## Anti-Features

Features to explicitly NOT build. These are common in the DID/VC ecosystem but wrong for ATAP.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Blockchain-based DID methods (did:ethr, did:ion, did:sov) | ATAP uses did:web for simplicity, no blockchain dependency, instant resolution. Blockchain adds latency, cost, and complexity with no benefit for ATAP's trust model | Use did:web exclusively. Server controls its own DID Documents |
| Full DIDComm mediator service (public, general-purpose) | A general-purpose mediator is a massive infrastructure project (pickup protocol, coordination protocol, message queuing). ATAP server IS the mediator for its own entities -- it doesn't need to be a public mediator | Server acts as mediator only for entities it hosts. Forward protocol for delivery between servers, not general mediation |
| DID universal resolver | Resolving arbitrary DID methods (did:key, did:ion, did:sov, etc.) is unbounded scope | Resolve did:web only. If an entity presents a non-did:web DID, reject it |
| Credential wallet interop (OpenID4VC, CHAPI) | OpenID for Verifiable Credentials and Credential Handler API are designed for browser-based credential exchange with arbitrary wallets. ATAP has its own mobile client | ATAP mobile app is the only wallet. Credentials flow through DIDComm, not OpenID4VC |
| BBS+ signatures / ZKP selective disclosure | Complex cryptography for unlinkable presentations. SD-JWT covers ATAP's selective disclosure needs without the crypto complexity | Use SD-JWT (RFC 9901) for selective disclosure. Simpler, standardized, good Go library support |
| Verifiable Presentation Request protocol | W3C VP Request spec is for arbitrary credential requests between unknown parties | ATAP defines its own approval request flow. Credentials are presented during registration/attestation, not during approval flows |
| DIDComm out-of-band protocol | Used for initial connection establishment between unknown parties via QR codes or deep links | ATAP entities register with a server. Connection establishment is server-mediated, not peer-to-peer discovery |
| Full JSON-LD processing | JSON-LD is complex (context resolution, graph normalization). VC 2.0 supports VC-JOSE-COSE which uses JWT format | Use VC-JOSE-COSE (JWT-based credentials). Include `@context` for spec compliance but process as plain JSON, don't resolve contexts |
| Federation protocol (v1.0) | Cross-server DIDComm relay is listed in PROJECT.md but marked for later. Building federation before single-server works is premature | Defer to post-v1.0. Design DID Documents and message format to be federation-ready, but don't implement cross-server relay yet |

## Feature Dependencies

```
Entity Registration
  +-> DID Document Creation
  |     +-> DID Document Hosting (/.well-known/ and /entities/{id}/)
  |     +-> Service Endpoints in DID Doc
  |           +-> DIDComm Message Delivery
  +-> Ed25519 Key Generation (existing)
        +-> X25519 Key Derivation
        |     +-> DIDComm Authenticated Encryption
        +-> JWS Signing
              +-> Approval Signatures
              +-> VC Issuance

OAuth 2.1 + DPoP
  +-> Token Endpoint
  +-> DPoP Proof Validation
        +-> All API endpoints (replaces current atap_ bearer auth)

DIDComm Plaintext Messages
  +-> DIDComm Signed Messages (JWS)
  +-> DIDComm Encrypted Messages (authcrypt/anoncrypt)
        +-> Approval Delivery
        +-> Forward Protocol (server as mediator)

VC Issuance
  +-> Credential Schemas (email, phone, personhood, etc.)
  +-> VC Storage
  +-> Trust Level Derivation
  +-> VC Verification (for inbound credentials)
  +-> Bitstring Status List (revocation)
  +-> SD-JWT Selective Disclosure

Two-Party Approval Flow
  +-> Approval State Machine
  +-> JWS Signatures on Approvals
  +-> Approval Expiry (TTL)
        +-> Three-Party Approval Flow
              +-> Signed Templates (via system provides branded UI)
              +-> Template Rendering (mobile)
              +-> Chained Approvals
              +-> Persistent Approvals + Revocation
              +-> Organization Delegate Routing

Crypto-Shredding
  +-> Per-Entity Encryption Keys (DEK/KEK pattern)
  +-> VC Storage Encryption
  +-> Key Deletion = Effective Erasure

Server Discovery (/.well-known/atap.json)
  (independent, can be built anytime)

Mobile Biometric Signing
  +-> Platform Secure Enclave (iOS Keychain / Android Keystore)
  +-> Flutter Platform Channel Integration
  +-> Approval Rendering UI
```

## MVP Recommendation

The v1.0-rc1 spec is large. Prioritize in this order based on dependencies and protocol coherence:

### Phase 1: Identity + Auth Foundation
1. **Entity registration with DID creation** -- everything depends on identity
2. **DID Document hosting and resolution** -- entities must be resolvable
3. **OAuth 2.1 + DPoP** -- replace custom auth before building new features on it
4. **Server discovery (`/.well-known/atap.json`)** -- cheap, enables client bootstrapping

### Phase 2: Messaging + Basic Approvals
5. **DIDComm plaintext + signed messages** -- approval delivery mechanism
6. **Two-party approval flow** -- simplest approval, proves the model works
7. **Approval state machine + JWS signatures** -- core protocol correctness

### Phase 3: Credentials + Trust
8. **VC issuance + verification** -- attestations that feed trust levels
9. **Trust level derivation** -- the policy layer
10. **Credential schemas for core types** -- email, phone, personhood

### Phase 4: Full Protocol
11. **Three-party approval flow** -- the signature feature
12. **DIDComm authenticated encryption** -- privacy for production
13. **Signed templates + brand rendering** -- the UX differentiator
14. **Mobile biometric signing** -- human-in-the-loop approval

### Defer to Post-v1.0
- **SD-JWT selective disclosure** -- valuable but not blocking core flows
- **Bitstring Status List revocation** -- can use simple revocation flag initially
- **Chained approvals** -- extension of core model
- **Organization delegate routing** -- extension of core model
- **Cross-server federation** -- explicitly out of scope per PROJECT.md
- **Crypto-shredding** -- important for production but not protocol-critical
- **Server trust assessment** -- requires multiple servers to be meaningful

## Sources

- [W3C did:web Method Specification](https://w3c-ccg.github.io/did-method-web/) - HIGH confidence
- [DIDComm Messaging Specification v2.1](https://identity.foundation/didcomm-messaging/spec/v2.1/) - HIGH confidence
- [W3C Verifiable Credentials Data Model v2.0](https://www.w3.org/TR/vc-data-model-2.0/) - HIGH confidence (W3C Recommendation May 2025)
- [RFC 9449: OAuth 2.0 DPoP](https://www.rfc-editor.org/rfc/rfc9449.html) - HIGH confidence
- [RFC 9901: SD-JWT](https://datatracker.ietf.org/doc/rfc9901/) - HIGH confidence
- [W3C Bitstring Status List v1.0](https://www.w3.org/TR/vc-bitstring-status-list/) - HIGH confidence
- [W3C Verifiable Credential Rendering Methods v1.0](https://www.w3.org/TR/vc-render-method/) - MEDIUM confidence (Working Draft)
- [DIDComm Routing / Mediators](https://didcomm.org/book/v2/routing/) - MEDIUM confidence
- [DIF Well-Known DID Configuration](https://github.com/decentralized-identity/well-known-did-configuration) - MEDIUM confidence
- [DIDComm V2 Mediator Test Suite](https://input-output-hk.github.io/didcomm-v2-mediator-test-suite/) - MEDIUM confidence
- [Thoughtworks: Crypto-shredding](https://www.thoughtworks.com/radar/techniques/crypto-shredding) - MEDIUM confidence
