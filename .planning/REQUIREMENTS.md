# Requirements: ATAP v1.0-rc1

**Defined:** 2026-03-13
**Core Value:** Any party can cryptographically verify who authorized an AI agent, what it may do, and under what constraints — offline, without callback to an authorization server.

## v1 Requirements

### Identity & DIDs (Spec §5)

- [x] **DID-01**: Every entity is identified by a `did:web` DID with path `{server}:{type}:{id}`
- [x] **DID-02**: DID Documents hosted at standard `did:web` HTTPS path with Ed25519 verification keys
- [x] **DID-03**: DID Documents include ATAP properties (`atap:type`, `atap:principal`) in `https://atap.dev/ns/v1` context
- [x] **DID-04**: Four entity types supported: human, agent, machine, org
- [x] **DID-05**: Human IDs derived from public key: `lowercase(base32(sha256(pubkey))[:16])`
- [x] **DID-06**: Agent DID Documents MUST include `atap:principal` referencing their controlling entity
- [x] **DID-07**: Key rotation via DID Document update; previous key versions retained with validity periods
- [x] **DID-08**: DID resolution uses HTTPS with valid TLS certificate

### Authorization (Spec §10)

- [x] **AUTH-01**: API access uses OAuth 2.1 with DPoP (RFC 9449) for sender-constrained tokens
- [x] **AUTH-02**: Agent entities use Client Credentials grant for token acquisition
- [x] **AUTH-03**: Human entities authenticate via Authorization Code grant with PKCE + device biometric
- [x] **AUTH-04**: All API tokens MUST be DPoP-bound with proof JWT on each request
- [x] **AUTH-05**: Token scopes: `atap:inbox`, `atap:send`, `atap:approve`, `atap:manage`
- [x] **AUTH-06**: Default access token lifetime 1 hour, refresh tokens up to 90 days

### Messaging — DIDComm v2.1 (Spec §7)

- [x] **MSG-01**: All entity-to-entity communication uses DIDComm v2.1
- [x] **MSG-02**: Server acts as DIDComm mediator for hosted entities (untrusted relay layer)
- [x] **MSG-03**: Server acts as ATAP system participant (`via`) for approval co-signing (trusted layer)
- [x] **MSG-04**: DIDComm authenticated encryption (ECDH-1PU + XC20P) for message confidentiality
- [x] **MSG-05**: ATAP message types under `https://atap.dev/protocols/` for all approval lifecycle events
- [ ] **MSG-06**: Organization delegate routing: fan-out capped at 50, per-source rate limiting, first-response-wins

### Approvals (Spec §8)

- [ ] **APR-01**: Two-party approvals: `from` signs, sends to `to` who approves/declines (2 signatures)
- [ ] **APR-02**: Three-party approvals: `from` signs → `via` validates + co-signs → `to` approves/declines (3 signatures)
- [x] **APR-03**: Approval format with `atap_approval: "1"`, `apr_` + ULID IDs, ISO 8601 timestamps
- [x] **APR-04**: Subject contains `type` (reverse-domain), `label`, `reversible` boolean, `payload` (system-specific JSON)
- [ ] **APR-05**: JWS Compact Serialization with detached payload (RFC 7515 + RFC 7797) for each signature
- [ ] **APR-06**: Signed payload is UTF-8 of JCS-serialized (RFC 8785) approval excluding `signatures` field
- [ ] **APR-07**: Full approval lifecycle: requested → approved/declined/expired/rejected → consumed/revoked
- [ ] **APR-08**: System rejection with `approval/1.0/rejected` message type and standardized reason codes
- [x] **APR-09**: One-time approvals (`valid_until` absent) transition to `consumed` after single use
- [x] **APR-10**: Persistent approvals (`valid_until` set) with receiver-side `max_approval_ttl` enforcement
- [x] **APR-11**: Chained approvals via `parent` field; revoking parent invalidates children
- [ ] **APR-12**: Approval verification: extract `kid` from JWS header, resolve DID, verify signature for each party

### Credentials — W3C VCs (Spec §6)

- [ ] **CRD-01**: All verified properties expressed as W3C Verifiable Credentials 2.0 (VC-JOSE-COSE format)
- [ ] **CRD-02**: ATAP credential types: EmailVerification, PhoneVerification, Personhood, Identity, Principal, OrgMembership
- [ ] **CRD-03**: Trust level derivation from credentials: L0 (none), L1 (email/phone), L2 (personhood), L3 (identity)
- [ ] **CRD-04**: Effective trust = `min(entity_trust_level, server_trust)`
- [ ] **CRD-05**: Credential revocation via W3C Bitstring Status List v1.0
- [ ] **CRD-06**: SD-JWT (RFC 9901) for selective disclosure on credentials containing personal information

### Server Trust (Spec §9)

- [x] **SRV-01**: Server discovery via `/.well-known/atap.json` with required fields (domain, api_base, didcomm_endpoint, claim_types)
- [x] **SRV-02**: Server trust levels: L0 (no TLS/self-signed), L1 (DV+DNSSEC), L2 (OV/EV+DNSSEC), L3 (OV/EV+DNSSEC+audit VC)
- [x] **SRV-03**: `max_approval_ttl` policy published in discovery document and enforced on received approvals

### Templates (Spec §11)

- [ ] **TPL-01**: Templates define approval rendering, provided exclusively by `via` system
- [ ] **TPL-02**: Templates carry JWS proof signed by `via` entity; client verifies against `via` DID
- [ ] **TPL-03**: Template fields: brand (name, logo, colors), display (title, fields with types), proof
- [ ] **TPL-04**: Field types: text, currency, date, date_range, list, image, number
- [ ] **TPL-05**: Security: HTTPS only, no redirects, IP validation (block RFC 1918/loopback/metadata), 64KB max, 5s timeout
- [ ] **TPL-06**: Two-party approvals use fallback rendering (label + formatted JSON payload)

### Mobile Client (Spec §12)

- [ ] **MOB-01**: Generate keypair in secure enclave, create `did:web` DID, set recovery passphrase
- [ ] **MOB-02**: DIDComm message inbox feed
- [ ] **MOB-03**: Approval rendering: fetch + verify template, render branded or fallback card, approve/decline with biometric
- [ ] **MOB-04**: Credential management: view, present, revoke VCs
- [ ] **MOB-05**: Persistent approval management: list, revoke
- [ ] **MOB-06**: Biometric prompt → JWS signature from secure enclave → send approval response via DIDComm

### API (Spec §13)

- [x] **API-01**: Entity endpoints: POST /v1/entities (register), GET /v1/entities/{id}, DELETE /v1/entities/{id} (crypto-shred)
- [x] **API-02**: DID resolution: GET /{type}/{id}/did.json (W3C did:web standard path)
- [ ] **API-03**: Approval endpoints: POST /v1/approvals, POST /v1/approvals/{id}/respond, GET /v1/approvals/{id}, GET /v1/approvals/{id}/status, GET /v1/approvals, DELETE /v1/approvals/{id}
- [ ] **API-04**: Credential endpoints: email/phone verification flows, personhood submission, list credentials, status list
- [x] **API-05**: DIDComm endpoint: POST /v1/didcomm
- [x] **API-06**: All errors follow RFC 7807 Problem Details with `https://atap.dev/errors/{type}` URIs

### Privacy & Compliance (Spec §5.7)

- [ ] **PRV-01**: All VC content containing personal information encrypted at rest with per-entity encryption key
- [ ] **PRV-02**: Crypto-shredding: delete per-entity key → all credential data unrecoverable
- [ ] **PRV-03**: Upon crypto-shredding: deactivate DID Document, notify federation partners
- [ ] **PRV-04**: Personhood credentials MUST NOT contain or transmit raw biometric data

### Infrastructure

- [x] **INF-01**: Strip old signal pipeline code (signals, channels, webhooks, custom auth, SSE)
- [x] **INF-02**: Database migration from signal-based schema to DID/approval/VC-based schema
- [x] **INF-03**: Docker Compose updated for new service configuration

## v2 Requirements

Deferred to post-v1.0. Tracked but not in current roadmap.

- **FED-01**: Cross-server DIDComm relay (federation between ATAP servers)
- **FED-02**: `did:webs` support for server-independent key authority (KERI witnesses)
- **EXT-01**: GNAP (RFC 9635) extension for multi-signature approval supplementing GNAP tokens
- **EXT-02**: ATAPApproval-as-VC wrapping for OpenID4VP presentation (Spec §14, deferred to v1.1)
- **EXT-03**: eIDAS 2.0 credential import via OpenID4VP
- **SEC-01**: Formal verification of approval flow (Tamarin Prover)
- **SEC-02**: Post-quantum migration via composite JWS (EdDSA+ML-DSA-65)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Custom signal/inbox/SSE pipeline | Replaced by DIDComm v2.1 |
| Custom webhook channels | Replaced by DIDComm service endpoints |
| Custom auth (Ed25519 signed requests) | Replaced by OAuth 2.1 + DPoP |
| Custom entity URIs (`agent://`) | Replaced by `did:web` |
| Custom claim codes | Replaced by W3C Verifiable Credentials |
| General-purpose DIDComm mediator | ATAP only mediates for own hosted entities |
| Client SDKs | API not stable yet; ship after v1.0 freezes |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| DID-01 | Phase 1 | Complete |
| DID-02 | Phase 1 | Complete |
| DID-03 | Phase 1 | Complete |
| DID-04 | Phase 1 | Complete |
| DID-05 | Phase 1 | Complete |
| DID-06 | Phase 1 | Complete |
| DID-07 | Phase 1 | Complete |
| DID-08 | Phase 1 | Complete |
| AUTH-01 | Phase 1 | Complete |
| AUTH-02 | Phase 1 | Complete |
| AUTH-03 | Phase 1 | Complete |
| AUTH-04 | Phase 1 | Complete |
| AUTH-05 | Phase 1 | Complete |
| AUTH-06 | Phase 1 | Complete |
| MSG-01 | Phase 2 | Complete |
| MSG-02 | Phase 2 | Complete |
| MSG-03 | Phase 2 | Complete |
| MSG-04 | Phase 2 | Complete |
| MSG-05 | Phase 2 | Complete |
| MSG-06 | Phase 4 | Pending |
| APR-01 | Phase 3 | Pending |
| APR-02 | Phase 3 | Pending |
| APR-03 | Phase 3 | Complete |
| APR-04 | Phase 3 | Complete |
| APR-05 | Phase 3 | Pending |
| APR-06 | Phase 3 | Pending |
| APR-07 | Phase 3 | Pending |
| APR-08 | Phase 3 | Pending |
| APR-09 | Phase 3 | Complete |
| APR-10 | Phase 3 | Complete |
| APR-11 | Phase 3 | Complete |
| APR-12 | Phase 3 | Pending |
| CRD-01 | Phase 4 | Pending |
| CRD-02 | Phase 4 | Pending |
| CRD-03 | Phase 4 | Pending |
| CRD-04 | Phase 4 | Pending |
| CRD-05 | Phase 4 | Pending |
| CRD-06 | Phase 4 | Pending |
| SRV-01 | Phase 1 | Complete |
| SRV-02 | Phase 1 | Complete |
| SRV-03 | Phase 1 | Complete |
| TPL-01 | Phase 3 | Pending |
| TPL-02 | Phase 3 | Pending |
| TPL-03 | Phase 3 | Pending |
| TPL-04 | Phase 3 | Pending |
| TPL-05 | Phase 3 | Pending |
| TPL-06 | Phase 3 | Pending |
| MOB-01 | Phase 4 | Pending |
| MOB-02 | Phase 4 | Pending |
| MOB-03 | Phase 4 | Pending |
| MOB-04 | Phase 4 | Pending |
| MOB-05 | Phase 4 | Pending |
| MOB-06 | Phase 4 | Pending |
| API-01 | Phase 1 | Complete |
| API-02 | Phase 1 | Complete |
| API-03 | Phase 3 | Pending |
| API-04 | Phase 4 | Pending |
| API-05 | Phase 2 | Complete |
| API-06 | Phase 1 | Complete |
| PRV-01 | Phase 4 | Pending |
| PRV-02 | Phase 4 | Pending |
| PRV-03 | Phase 4 | Pending |
| PRV-04 | Phase 4 | Pending |
| INF-01 | Phase 1 | Complete |
| INF-02 | Phase 1 | Complete |
| INF-03 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 66 total
- Mapped to phases: 66
- Unmapped: 0

---
*Requirements defined: 2026-03-13*
*Traceability updated: 2026-03-13*
