# Architecture Patterns

**Domain:** DID/DIDComm/VC protocol server with multi-signature approval engine
**Researched:** 2026-03-13

## Recommended Architecture

The ATAP v1.0-rc1 server is a protocol server with seven distinct subsystems. The existing Go/Fiber/pgx/Redis stack survives but the signal/inbox/webhook layer gets replaced by DIDComm messaging, and custom auth gets replaced by OAuth 2.1 + DPoP.

```
                          HTTPS (Fiber v2)
                               |
            +------------------+------------------+
            |                  |                  |
     /.well-known/        /v1/ API           DIDComm
     atap.json          (OAuth 2.1          Endpoint
     did.json            + DPoP)           /didcomm
            |                  |                  |
            v                  v                  v
    +------------+    +---------------+    +-------------+
    | Discovery  |    | OAuth Engine  |    | DIDComm     |
    | Service    |    | (Token +      |    | Mediator    |
    |            |    |  DPoP verify) |    | (unpack,    |
    +------------+    +---------------+    |  route,     |
                              |            |  queue)     |
                              v            +-------------+
                      +---------------+          |
                      | API Handlers  |          |
                      | (entities,    |<---------+
                      |  approvals,   |
                      |  credentials, |
                      |  templates)   |
                      +---------------+
                              |
            +--------+--------+--------+--------+
            |        |        |        |        |
            v        v        v        v        v
      +---------+ +------+ +------+ +------+ +--------+
      |Approval | | VC   | |DID   | |Trust | |Template|
      |Engine   | |Issuer| |Mgmt  | |Assess| |Engine  |
      |         | |/Verif| |      | |      | |        |
      +---------+ +------+ +------+ +------+ +--------+
            |        |        |        |        |
            +--------+--------+--------+--------+
                              |
                    +---------+---------+
                    |         |         |
                    v         v         v
              PostgreSQL   Redis    Push (FCM)
              (pgx/v5)   (pub/sub)
```

### Component Boundaries

| Component | Responsibility | Communicates With | Package |
|-----------|---------------|-------------------|---------|
| **Discovery Service** | Serve `/.well-known/atap.json` and `/.well-known/did.json` (server DID doc); resolve remote `did:web` DIDs by fetching their documents | API Handlers, DID Management | `internal/discovery` |
| **OAuth Engine** | Issue + validate access tokens; verify DPoP proofs (RFC 9449); manage token lifecycle; bind tokens to entity public keys | API Handlers (middleware) | `internal/auth` |
| **DIDComm Mediator** | Receive DIDComm encrypted messages at `/didcomm` endpoint; unpack (decrypt + verify); route to target entity's queue; pack outbound messages; hold messages for offline entities | Approval Engine, DID Management, Redis (queues) | `internal/didcomm` |
| **API Handlers** | HTTP endpoints for entity CRUD, approval management, credential operations, template management | All other components | `internal/api` |
| **Approval Engine** | Multi-signature approval lifecycle: create, collect signatures (2-party and 3-party), state machine (requested -> approved/declined/expired/rejected -> consumed/revoked), TTL expiry, chained approvals | DIDComm Mediator (notifications), VC Issuer (proof bundles), Template Engine | `internal/approval` |
| **VC Issuer/Verifier** | Issue W3C VCs (VC-JOSE-COSE format) for email, phone, personhood, identity, principal, org membership; verify presented VCs; manage Bitstring Status List for revocation; SD-JWT selective disclosure | DID Management (signing keys), Trust Assessor (trust level derivation) | `internal/credential` |
| **DID Management** | Create and store DID Documents; manage key material (Ed25519 signing, X25519 encryption); host per-entity DID documents at `did:web` paths; key rotation | Discovery Service, Store | `internal/did` |
| **Trust Assessor** | Derive trust levels (0-3) from entity credentials; assess remote server trust (WebPKI + DNSSEC + audit VCs) | VC Verifier, Discovery Service | `internal/trust` |
| **Template Engine** | Store and serve branded approval templates; verify template JWS signatures; render template metadata for mobile clients | Approval Engine, Store | `internal/template` |
| **Store** | PostgreSQL data access for all domain objects | PostgreSQL | `internal/store` |
| **Crypto** | Ed25519/X25519 key operations; JWS signing (RFC 7515, RFC 7797 detached); JCS canonicalization (RFC 8785); JOSE operations | All components needing crypto | `internal/crypto` |

## Data Flow: Approval Lifecycle

The approval lifecycle is the core protocol flow. Here is how data moves through a three-party approval (the most complex case).

### Three-Party Approval Flow

```
Agent (from)          ATAP Server (via)           Human (to)
     |                      |                         |
     | 1. POST /v1/approvals                          |
     |   {from_did, to_did, |                         |
     |    scope, template_id,                         |
     |    from_signature}   |                         |
     |--------------------->|                         |
     |                      |                         |
     |                      | 2. Validate from_signature
     |                      |    (verify JWS over approval body)
     |                      |                         |
     |                      | 3. Resolve template     |
     |                      |    (verify template JWS)|
     |                      |                         |
     |                      | 4. Server co-signs      |
     |                      |    (add via_signature)  |
     |                      |                         |
     |                      | 5. Store approval       |
     |                      |    state: "requested"   |
     |                      |                         |
     |                      | 6. DIDComm message ---->|
     |                      |    (encrypted, to       |
     |                      |     target's X25519 key)|
     |                      |                         |
     |                      |    + Push notification  |
     |                      |                         |
     |                      |                         | 7. Human opens
     |                      |                         |    mobile app
     |                      |                         |
     |                      |                         | 8. Fetch approval
     |                      |<------------------------| GET /v1/approvals/:id
     |                      |                         |
     |                      | 9. Return approval with |
     |                      |    template render data |
     |                      |------------------------>|
     |                      |                         |
     |                      |                         | 10. Human reviews
     |                      |                         |     template UI
     |                      |                         |
     |                      |                         | 11. Biometric auth
     |                      |                         |     + sign approval
     |                      |                         |
     |                      | 12. POST approve/decline|
     |                      |<------------------------|
     |                      |    {to_signature}       |
     |                      |                         |
     |                      | 13. Verify to_signature |
     |                      |     Update state:       |
     |                      |     "approved"/"declined"|
     |                      |                         |
     |  14. DIDComm notify  |                         |
     |<---------------------|                         |
     |  (or webhook/poll)   |                         |
     |                      |                         |
     | 15. GET /v1/approvals/:id                      |
     |--------------------->|                         |
     |                      |                         |
     | 16. Full approval    |                         |
     |  with all 3 sigs     |                         |
     |<---------------------|                         |
```

### Two-Party Approval Flow

Same as above but skip steps 3-4 (no template, no server co-sign). Only `from_signature` and `to_signature` needed. State machine is identical.

### Approval State Machine

```
                 +-- expired (TTL)
                 |
requested -------+-- declined (to signs decline)
                 |
                 +-- rejected (server policy)
                 |
                 +-- approved (to signs approve)
                         |
                    +----+----+
                    |         |
                consumed   revoked
                (used)    (from/to revokes)
```

### Signature Accumulation

Each approval carries a `signatures` array. Signatures are JWS with detached payload (RFC 7797):

```json
{
  "id": "apr_01HXYZ...",
  "from": "did:web:atap.app:agents:abc",
  "to": "did:web:atap.app:humans:def",
  "via": "did:web:atap.app",
  "scope": { "actions": ["read:email"], "resources": ["mailbox:*"] },
  "status": "approved",
  "signatures": [
    { "signer": "did:web:...agents:abc", "role": "from", "jws": "eyJ..." },
    { "signer": "did:web:atap.app",      "role": "via",  "jws": "eyJ..." },
    { "signer": "did:web:...humans:def",  "role": "to",   "jws": "eyJ..." }
  ]
}
```

The detached payload is `JCS(approval_body)` -- the canonical JSON of the approval minus the signatures array. Each signer signs the same canonical payload, making verification straightforward: deserialize, strip signatures, canonicalize, verify each JWS against the signer's public key from their DID Document.

## DIDComm Mediator Integration with Approval Engine

The DIDComm mediator serves two roles in ATAP:

### Role 1: Notification Transport

When an approval is created or its state changes, the Approval Engine publishes an event to Redis. The DIDComm Mediator subscribes to these events and:

1. Resolves the target entity's DID Document to find their `serviceEndpoint`
2. If the entity has a direct endpoint (agent/machine): pack and send DIDComm message directly
3. If the entity is mediated (human/mobile): queue the message for pickup via the mediator protocol

```
Approval Engine ---(Redis pub/sub)--> DIDComm Mediator
                                           |
                            +--------------+--------------+
                            |                             |
                     Direct delivery              Queue for pickup
                     (POST to endpoint)          (stored in Redis,
                                                  delivered on
                                                  next connection)
```

### Role 2: Inbound Message Processing

External DIDComm messages arriving at `/didcomm` may contain:
- Approval requests from remote ATAP servers (federation)
- Approval responses from mobile clients using DIDComm
- Credential presentation messages

The mediator unpacks these, identifies the message type from the DIDComm `type` field, and routes to the appropriate handler (Approval Engine, VC Verifier, etc.).

### Message Queue Design

For mobile entities that are intermittently connected:

```
Redis key: didcomm:queue:{entity-did}
Type: List (RPUSH/LPOP)
TTL: 30 days (configurable)
```

When a mobile client connects (via long-poll or WebSocket at `/didcomm/pickup`), queued messages are delivered and acknowledged. This replaces the old SSE inbox pattern.

## Patterns to Follow

### Pattern 1: Domain Service Layer

Separate HTTP handlers from business logic. Each domain component (approval, credential, did, etc.) exposes a Go interface consumed by API handlers.

**What:** Thin HTTP handlers that parse requests, call domain services, return responses.
**When:** Always. Every handler should delegate to a service.
**Example:**
```go
// internal/approval/service.go
type Service struct {
    store    Store
    crypto   *crypto.Service
    didmgr   *did.Manager
    didcomm  *didcomm.Mediator
    template *template.Engine
}

func (s *Service) CreateApproval(ctx context.Context, req CreateRequest) (*Approval, error) {
    // 1. Resolve from DID, verify from_signature
    // 2. Resolve template if via flow
    // 3. Server co-signs if via flow
    // 4. Persist approval
    // 5. Notify target via DIDComm
    return approval, nil
}

// internal/api/approvals.go
func (h *Handler) CreateApproval(c *fiber.Ctx) error {
    var req approval.CreateRequest
    if err := c.BodyParser(&req); err != nil {
        return problem(c, 400, "invalid_request", "Invalid request body", err.Error())
    }
    apr, err := h.approvalService.CreateApproval(c.Context(), req)
    if err != nil {
        return mapError(c, err)
    }
    return c.Status(201).JSON(apr)
}
```

### Pattern 2: Event-Driven Notifications via Redis

Use Redis pub/sub for loose coupling between the approval engine and notification transports (DIDComm, push notifications).

**What:** Approval state changes publish events. Multiple subscribers react independently.
**When:** Any state transition that needs to notify external parties.
**Example:**
```go
// Approval Engine publishes
redis.Publish(ctx, "approval:events", ApprovalEvent{
    Type:       "approval.state_changed",
    ApprovalID: apr.ID,
    NewState:   "approved",
    TargetDID:  apr.From,  // notify the requester
})

// DIDComm Mediator subscribes
// Push Service subscribes
```

### Pattern 3: DID Document as Source of Truth for Keys

Never store public keys separately from DID Documents. The DID Document IS the key registry.

**What:** Key lookup always goes through DID resolution. Local DIDs resolve from the database; remote DIDs resolve via HTTP fetch of their `did.json`.
**When:** Any operation requiring a public key (signature verification, message encryption).

### Pattern 4: Store Interface per Domain

Each domain component defines its own store interface with only the methods it needs. The concrete `store.Store` implements all of them.

**What:** Interface segregation. The approval engine only sees `ApprovalStore`, not the full store.
**When:** Always. Keeps domain components testable and boundaries clean.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Monolithic Handler with Crypto Logic
**What:** Putting signature verification, JWS construction, and business logic in HTTP handlers.
**Why bad:** The existing codebase does this (see `api.go` SendSignal handler at 140+ lines). It makes testing require HTTP context, mixes transport with domain logic, and makes it impossible to reuse logic from DIDComm message handlers.
**Instead:** Extract to domain service. HTTP handler and DIDComm message handler both call the same service method.

### Anti-Pattern 2: Storing Signatures Separately from Approvals
**What:** Putting signatures in a separate table joined by approval ID.
**Why bad:** An approval is a self-contained proof bundle. Splitting it across tables means you cannot hand someone a single JSON object that is independently verifiable.
**Instead:** Store the full approval (with signatures JSONB array) as a single row. Index on status and participant DIDs.

### Anti-Pattern 3: DIDComm Library as Black Box
**What:** Treating the DIDComm library as a magic layer that handles everything.
**Why bad:** aries-framework-go (the main Go DIDComm library) is archived since March 2024. TrustBloc vc-go has VC tooling but limited DIDComm. The Go DIDComm ecosystem is thin. You will likely need to implement DIDComm v2.1 message packing/unpacking using lower-level JOSE primitives.
**Instead:** Build a thin DIDComm layer on top of go-jose (github.com/go-jose/go-jose/v4) and lestrrat-go/jwx. Implement pack/unpack for the three message formats (plaintext, signed, encrypted) directly using JWE/JWS primitives. This is more work upfront but avoids depending on an archived or insufficiently maintained framework.

### Anti-Pattern 4: Global Server Key for Everything
**What:** Using a single Ed25519 key for server DID, OAuth token signing, template signing, and approval co-signing.
**Why bad:** Key compromise affects everything. Different key purposes have different rotation schedules.
**Instead:** Server DID Document should list multiple keys with different purposes (authentication, assertionMethod, keyAgreement). Use separate keys for: (1) OAuth token signing, (2) VC issuance, (3) approval co-signing, (4) DIDComm encryption.

## Component Dependencies (Build Order)

Dependencies flow downward. Build bottom-up.

```
Layer 0 (Foundation - no domain dependencies):
  crypto/         - Ed25519, X25519, JWS, JCS, JOSE primitives
  store/          - PostgreSQL migrations + data access
  config/         - Environment configuration

Layer 1 (Identity - depends on Layer 0):
  did/            - DID Document creation, storage, resolution
  discovery/      - .well-known endpoints, remote DID fetch

Layer 2 (Auth + Messaging - depends on Layer 1):
  auth/           - OAuth 2.1 + DPoP token issuance/verification
  didcomm/        - Message pack/unpack, mediator queue

Layer 3 (Domain - depends on Layers 1-2):
  credential/     - VC issuance, verification, status list
  trust/          - Trust level derivation, server assessment
  template/       - Template storage, JWS verification, rendering

Layer 4 (Core Protocol - depends on Layer 3):
  approval/       - Multi-sig approval engine, state machine

Layer 5 (Transport - depends on all):
  api/            - HTTP handlers, routes, middleware
```

### Suggested Build Order (Phases)

**Phase 1: Foundation + Identity**
Build crypto extensions (JWS detached, X25519), DID Document model, did:web hosting, discovery endpoints. This gives you `/.well-known/atap.json`, `/.well-known/did.json`, and per-entity DID documents at their `did:web` paths.

Rationale: Everything else depends on DIDs. You cannot verify signatures, issue credentials, or send DIDComm messages without DID resolution working.

**Phase 2: Auth**
OAuth 2.1 authorization server with DPoP. Entity registration now returns a DID instead of a custom URI. Replace the old Ed25519 signed-request auth middleware with OAuth bearer + DPoP verification.

Rationale: API auth must work before you build any authenticated endpoints. DPoP depends on DID key material from Phase 1.

**Phase 3: DIDComm Messaging**
Implement DIDComm v2.1 message pack/unpack (plaintext, signed, encrypted). Build the mediator with message queuing. This replaces Redis pub/sub SSE with DIDComm message delivery.

Rationale: Approvals need DIDComm to notify participants. Build messaging before the approval engine so you have a delivery mechanism.

**Phase 4: Approval Engine**
The core protocol. Two-party and three-party approval flows. State machine. Signature accumulation and verification. Chained approvals. TTL expiry.

Rationale: This is the product. Everything before it is infrastructure.

**Phase 5: Credentials + Trust**
VC issuance (email, phone, personhood), Bitstring Status List revocation, SD-JWT selective disclosure, trust level derivation. Server trust assessment.

Rationale: Credentials raise trust levels but are not required for basic approvals. Build after the approval engine works.

**Phase 6: Templates + Mobile**
Signed approval templates with brand rendering. Mobile approval UI with biometric signing.

Rationale: Templates are provided by the `via` system. They enhance the UX but approvals work without them (plain text fallback). Mobile depends on all server APIs being stable.

## Database Schema Evolution

The existing schema (entities, signals, channels, webhook_configs, claims, delegations, delivery_attempts, push_tokens) gets partially replaced:

**Keep:** `entities` table (evolves to store DID documents), `push_tokens`
**Drop:** `signals`, `channels`, `webhook_configs`, `delivery_attempts`, `claims`, `delegations`
**Add:**
- `did_documents` - Full DID Document JSONB, indexed by DID
- `key_material` - Encrypted private keys (server-side keys only, never human keys)
- `approvals` - Core approval records with signatures JSONB array
- `credentials` - Issued VCs with encrypted JSONB (for crypto-shredding)
- `credential_status_lists` - Bitstring Status List entries
- `oauth_tokens` - Access/refresh tokens with DPoP binding
- `templates` - Approval templates with JWS signatures
- `didcomm_queue` - Queued DIDComm messages for offline entities
- `entity_encryption_keys` - Per-entity encryption keys for crypto-shredding

## Scalability Considerations

| Concern | At 100 entities | At 10K entities | At 1M entities |
|---------|----------------|-----------------|----------------|
| DID resolution | Local DB lookup | Local DB + cache remote DIDs | DID Document CDN cache, Redis DID cache |
| DIDComm queue | Redis list per entity | Redis with TTL eviction | Redis Cluster, or move queues to PostgreSQL with partition |
| Approval throughput | Single PostgreSQL | Connection pooling, read replicas | Partition approvals by date, archive consumed |
| VC status list | Single bitstring per issuer | Multiple status lists | Content-addressed status lists on CDN |
| Template rendering | Inline JSON | Template cache in Redis | Template CDN |

## Sources

- [DIDComm Messaging Specification v2.1](https://identity.foundation/didcomm-messaging/spec/v2.1/) - Message formats, mediator protocol
- [did:web Method Specification](https://w3c-ccg.github.io/did-method-web/) - DID Document hosting at well-known paths
- [W3C VC-JOSE-COSE](https://www.w3.org/TR/vc-jose-cose/) - Securing VCs with JOSE (now W3C Recommendation, May 2025)
- [RFC 9449 - OAuth 2.0 DPoP](https://datatracker.ietf.org/doc/html/rfc9449) - Proof of possession for access tokens
- [RFC 7797 - JWS Unencoded Payload](https://datatracker.ietf.org/doc/html/rfc7797) - Detached JWS for approval signatures
- [W3C Bitstring Status List v1.0](https://www.w3.org/TR/vc-bitstring-status-list/) - Credential revocation mechanism
- [go-jose/go-jose v4](https://pkg.go.dev/github.com/go-jose/go-jose/v4) - Go JOSE library with detached payload support
- [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx) - Complete JOSE implementation for Go (JWS, JWE, JWK, JWT)
- [TrustBloc vc-go](https://github.com/trustbloc/vc-go) - W3C VC Go library (active, maintained)
- [Hyperledger Aries Framework Go](https://github.com/hyperledger-aries/aries-framework-go) - DIDComm Go framework (ARCHIVED March 2024 -- do not depend on)
- [github.com/matoous/authlib/dpop](https://pkg.go.dev/github.com/matoous/authlib/dpop) - Go DPoP implementation
- [did-web-server](https://dws.identinet.io/) - Reference implementation for did:web hosting
