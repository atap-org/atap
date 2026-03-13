# ATAP: Agent Trust and Authority Protocol

## What This Is

ATAP is an open protocol for verifiable multi-party authorization in AI agent ecosystems. It defines a multi-signature approval model where requesting entities, mediating systems, and approving entities each sign independently — producing a self-contained, non-repudiable proof of consent. Built entirely on established standards (W3C DIDs, DIDComm v2.1, W3C VCs, OAuth 2.1 + DPoP, JWS/JCS).

## Core Value

Any party receiving a request from an AI agent can cryptographically verify who authorized that agent, what it is permitted to do, and under what constraints — offline, without callback to an authorization server.

## Requirements

### Validated

- ✓ Go backend with Fiber, PostgreSQL, Redis — existing
- ✓ Docker Compose infrastructure — existing
- ✓ Ed25519 key generation and JCS canonicalization (RFC 8785) — existing
- ✓ RFC 7807 error responses — existing
- ✓ Structured JSON logging (zerolog) — existing
- ✓ Flutter app scaffold with Riverpod, GoRouter, secure storage — existing
- ✓ Firebase push notification integration — existing

### Active

- [ ] Entity registration with `did:web` DIDs (human, agent, machine, org)
- [ ] DID Document hosting and resolution
- [ ] DIDComm v2.1 messaging (authenticated encryption)
- [ ] OAuth 2.1 + DPoP API authentication
- [ ] Two-party approvals (from + to, 2 signatures)
- [ ] Three-party approvals (from + via + to, 3 signatures)
- [ ] Approval lifecycle (requested → approved/declined/expired/rejected → consumed/revoked)
- [ ] JWS signatures with detached payload (RFC 7515 + RFC 7797)
- [ ] Server discovery via `/.well-known/atap.json`
- [ ] W3C Verifiable Credentials (email, phone, personhood, identity, principal, org membership)
- [ ] Trust level derivation from credentials (levels 0-3)
- [ ] Server trust assessment (WebPKI + DNSSEC + audit VCs)
- [ ] Signed approval templates with brand rendering
- [ ] Persistent approvals with TTL and revocation
- [ ] Chained approvals (parent/child)
- [ ] Mobile approval rendering with biometric signing
- [ ] Organization delegate routing
- [ ] SD-JWT selective disclosure
- [ ] Credential revocation via Bitstring Status List
- [ ] Crypto-shredding for GDPR erasure
- [ ] Cross-server DIDComm relay (federation)

### Out of Scope

- Custom signal/inbox/SSE pipeline — replaced by DIDComm v2.1
- Custom webhook channels — replaced by DIDComm service endpoints
- Custom auth (Ed25519 signed requests) — replaced by OAuth 2.1 + DPoP
- Custom entity URIs (`agent://`) — replaced by `did:web`
- Custom claim codes — replaced by W3C Verifiable Credentials
- GNAP extension draft — deferred to post-v1.0
- Formal verification (Tamarin Prover) — deferred to post-v1.0
- eIDAS 2.0 credential import — deferred to post-v1.0
- ATAPApproval-as-VC wrapping — deferred to v1.1

## Context

**Existing codebase:** Go backend + Flutter mobile app from Phase 1 "The Doorbell" — a signal broker prototype. Infrastructure (Docker, PostgreSQL, Redis, Go skeleton, Flutter scaffold) survives. Protocol layer (signals, custom auth, custom identity) gets stripped and rebuilt.

**Spec:** v1.0-rc1 at `/Users/svenloth/Downloads/ATAP-SPEC-v1.0-rc1.md`. Defines 16 sections covering identity (DIDs), claims (VCs), messaging (DIDComm), authorization (OAuth), approvals (multi-sig), templates, server trust, mobile client, and API.

**Standards stack:**
| Layer | Standard |
|-------|----------|
| Identity | W3C DIDs (`did:web`) |
| Claims | W3C Verifiable Credentials 2.0 (VC-JOSE-COSE) |
| Messaging | DIDComm v2.1 |
| Authorization | OAuth 2.1 + DPoP (RFC 9449) |
| Server Trust | WebPKI + DNSSEC + VC attestations |
| Signatures | JWS (RFC 7515) with JCS (RFC 8785) |

**Key protocol decisions from spec:**
- `did:web` for all entity types (human, agent, machine, org)
- Human IDs derived from public key: `lowercase(base32(sha256(pubkey))[:16])`
- Approval IDs: `apr_` + ULID
- Three-party flow: from signs → via validates + co-signs → to approves/declines
- Two-party flow: from signs → to approves/declines (no via)
- Templates provided exclusively by `via` system, self-signed with JWS
- Crypto-shredding for GDPR: delete per-entity encryption key → all VC data unrecoverable

## Constraints

- **Tech stack**: Go 1.22+ backend, Flutter mobile, PostgreSQL, Redis — keep existing infrastructure
- **DIDComm library**: Use existing Go DIDComm library (e.g., `github.com/decentralized-identity/didcomm-go`) — don't build from scratch
- **Crypto**: Ed25519 signing, X25519 encryption (NaCl/libsodium). Private keys never leave generating device.
- **Spec compliance**: Implementation must match v1.0-rc1 spec exactly — field names, flows, lifecycle states
- **Mobile**: Keep Flutter scaffold, rebuild features for approval rendering instead of signal inbox

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Strip old signal pipeline | New spec uses DIDComm, not custom signals | — Pending |
| Use existing DIDComm Go library | DIDComm v2.1 is complex, battle-tested libs exist | — Pending |
| Keep Flutter scaffold | Navigation, secure storage, Firebase setup reusable | — Pending |
| Archive old planning files | Clean slate for v1.0-rc1 implementation | ✓ Good |
| `did:web` for identity | Spec mandates it, widely supported | — Pending |
| OAuth 2.1 + DPoP for API auth | Spec mandates it, industry standard | — Pending |

---
*Last updated: 2026-03-13 after protocol redesign to v1.0-rc1*
