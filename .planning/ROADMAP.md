# Roadmap: ATAP v1.0-rc1

## Overview

ATAP v1.0-rc1 replaces the existing signal broker prototype with a standards-based protocol for verifiable multi-party authorization. The build follows a strict dependency chain: identity and auth must exist before messaging, messaging before approvals, and the core approval engine before credentials and mobile. Infrastructure cleanup (stripping the old signal pipeline) happens first, then the protocol stack is built bottom-up across four phases. The approval engine in Phase 3 is the product -- everything before it is plumbing, everything after it is trust enrichment and end-user experience.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Identity and Auth Foundation** - Strip old pipeline, establish DID identity, OAuth 2.1 + DPoP auth, server discovery
- [ ] **Phase 2: DIDComm Messaging** - Build DIDComm v2.1 messaging layer with server mediator
- [ ] **Phase 3: Approval Engine** - Two-party and three-party approval flows with templates
- [ ] **Phase 4: Credentials and Mobile** - W3C VCs, trust levels, privacy controls, mobile approval client

## Phase Details

### Phase 1: Identity and Auth Foundation
**Goal**: Any entity can register with a `did:web` DID, authenticate via OAuth 2.1 + DPoP, and have its DID Document resolved by any standards-compliant client
**Depends on**: Nothing (first phase)
**Requirements**: INF-01, INF-02, INF-03, DID-01, DID-02, DID-03, DID-04, DID-05, DID-06, DID-07, DID-08, AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, SRV-01, SRV-02, SRV-03, API-01, API-02, API-06
**Success Criteria** (what must be TRUE):
  1. Old signal/channel/webhook/SSE code is removed and the database schema reflects the new DID/approval/VC model
  2. An entity can register and receive a `did:web` DID, and its DID Document is resolvable at the standard HTTPS path with correct Ed25519 verification methods and ATAP context
  3. An agent can obtain a DPoP-bound OAuth access token via Client Credentials grant, and a human via Authorization Code + PKCE, and use it to call authenticated API endpoints
  4. `GET /.well-known/atap.json` returns a valid server discovery document with domain, api_base, didcomm_endpoint, claim_types, and max_approval_ttl
  5. All API errors return RFC 7807 Problem Details with `https://atap.dev/errors/` URIs
**Plans**: 4 plans

Plans:
- [ ] 01-01-PLAN.md — Strip old pipeline, new schema, domain model rebuild
- [ ] 01-02-PLAN.md — DID identity, entity registration, DID Document resolution
- [ ] 01-03-PLAN.md — Server discovery endpoint, RFC 7807 error standardization
- [ ] 01-04-PLAN.md — OAuth 2.1 authorization server with DPoP middleware

### Phase 2: DIDComm Messaging
**Goal**: Entities can exchange authenticated, encrypted DIDComm v2.1 messages through the server acting as mediator, replacing the old SSE/Redis pub/sub delivery system
**Depends on**: Phase 1
**Requirements**: MSG-01, MSG-02, MSG-03, MSG-04, MSG-05, API-05
**Success Criteria** (what must be TRUE):
  1. One entity can send a DIDComm v2.1 message to another entity via POST /v1/didcomm, with the server mediating delivery
  2. Messages are encrypted with ECDH-1PU + XC20P authenticated encryption -- only the intended recipient can decrypt
  3. The server queues messages for offline entities and delivers them when the recipient reconnects
  4. ATAP protocol message types under `https://atap.dev/protocols/` are defined and routable for all approval lifecycle events
**Plans**: TBD

Plans:
- [ ] 02-01: TBD
- [ ] 02-02: TBD

### Phase 3: Approval Engine
**Goal**: An agent can request approval from a human (two-party) or through a mediating system (three-party), with each party independently signing via JWS, producing a self-contained, offline-verifiable proof of consent
**Depends on**: Phase 2
**Requirements**: APR-01, APR-02, APR-03, APR-04, APR-05, APR-06, APR-07, APR-08, APR-09, APR-10, APR-11, APR-12, TPL-01, TPL-02, TPL-03, TPL-04, TPL-05, TPL-06, API-03
**Success Criteria** (what must be TRUE):
  1. A two-party approval completes end-to-end: `from` signs a request, `to` receives it via DIDComm, approves or declines with their own JWS signature, and the resulting approval contains two independently verifiable signatures
  2. A three-party approval completes end-to-end: `from` signs, `via` validates and co-signs (injecting a branded template), `to` sees the branded rendering and approves/declines, producing three signatures
  3. Approvals follow the full lifecycle (requested, approved, declined, expired, rejected, consumed, revoked) with correct state transitions enforced by the server
  4. Any party holding an approval can verify each signature by resolving the signer's DID and checking the JWS against their public key -- offline, without callback
  5. Persistent approvals respect TTL and max_approval_ttl policy; revoking a parent approval invalidates its children
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD
- [ ] 03-03: TBD

### Phase 4: Credentials and Mobile
**Goal**: Entities can earn trust through verifiable credentials, humans can manage approvals and credentials from a mobile app with biometric signing, and privacy controls enable GDPR-compliant data erasure
**Depends on**: Phase 3
**Requirements**: CRD-01, CRD-02, CRD-03, CRD-04, CRD-05, CRD-06, PRV-01, PRV-02, PRV-03, PRV-04, MOB-01, MOB-02, MOB-03, MOB-04, MOB-05, MOB-06, API-04, MSG-06
**Success Criteria** (what must be TRUE):
  1. A human entity can verify their email or phone and receive a W3C Verifiable Credential, raising their trust level from L0 to L1; personhood verification raises to L2; identity verification to L3
  2. Effective trust is computed as `min(entity_trust_level, server_trust)` and is visible to any party evaluating an approval
  3. The mobile app generates a keypair in the secure enclave, creates a did:web DID, displays a DIDComm inbox, renders branded approval cards, and signs approval responses with biometric confirmation
  4. Deleting an entity crypto-shreds all personal credential data (per-entity encryption key deleted, VC content unrecoverable, DID Document deactivated)
  5. Organization delegate routing delivers approval requests to up to 50 delegates with first-response-wins semantics
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD
- [ ] 04-03: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Identity and Auth Foundation | 0/4 | Planning complete | - |
| 2. DIDComm Messaging | 0/2 | Not started | - |
| 3. Approval Engine | 0/3 | Not started | - |
| 4. Credentials and Mobile | 0/3 | Not started | - |
