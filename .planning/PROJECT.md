# ATAP: Agent Trust and Authority Protocol

## What This Is

ATAP is an open protocol for verifiable multi-party authorization in AI agent ecosystems. It implements a complete standards-based stack: W3C DIDs for identity, DIDComm v2.1 for encrypted messaging, W3C Verifiable Credentials for trust, OAuth 2.1 + DPoP for API auth, and a multi-signature approval model where each party signs independently — producing self-contained, offline-verifiable proof of consent.

## Core Value

Any party receiving a request from an AI agent can cryptographically verify who authorized that agent, what it is permitted to do, and under what constraints — offline, without callback to an authorization server.

## Requirements

### Validated

- ✓ Entity registration with `did:web` DIDs (human, agent, machine, org) — v1.0
- ✓ DID Document hosting and resolution — v1.0
- ✓ DIDComm v2.1 messaging (ECDH-1PU authenticated encryption) — v1.0
- ✓ OAuth 2.1 + DPoP API authentication — v1.0
- ✓ Two-party approvals (from + to, 2 JWS signatures) — v1.0
- ✓ Three-party approvals (from + via + to, 3 JWS signatures) — v1.0
- ✓ Approval lifecycle (requested → approved/declined/expired/rejected → consumed/revoked) — v1.0
- ✓ JWS signatures with detached payload (RFC 7515 + RFC 7797) — v1.0
- ✓ Server discovery via `/.well-known/atap.json` — v1.0
- ✓ W3C Verifiable Credentials (email, phone, personhood, identity, principal, org membership) — v1.0
- ✓ Trust level derivation from credentials (levels 0-3) — v1.0
- ✓ Server trust assessment (WebPKI + DNSSEC + audit VCs) — v1.0
- ✓ Signed approval templates with Adaptive Card rendering — v1.0
- ✓ Standing Approvals with TTL and revocation — v1.0
- ✓ Chained approvals (parent/child) — v1.0
- ✓ Mobile approval rendering with biometric signing — v1.0
- ✓ Organization delegate routing (fan-out, first-response-wins) — v1.0
- ✓ SD-JWT selective disclosure — v1.0
- ✓ Credential revocation via Bitstring Status List — v1.0
- ✓ Crypto-shredding for GDPR erasure — v1.0
- ✓ RFC 7807 error responses — v1.0
- ✓ Structured JSON logging (zerolog) — v1.0

### Active

(None — next milestone requirements to be defined via `/gsd:new-milestone`)

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
- Cross-server DIDComm relay (federation) — deferred to v2.0
- Post-quantum migration (EdDSA+ML-DSA-65) — deferred to v2.0
- `did:webs` support — deferred to v2.0

## Context

Shipped v1.0-rc1 with 17,004 Go LOC + 3,058 Dart LOC across 173 commits in 7 days.

**Tech stack:** Go 1.22+ (Fiber v2, pgx/v5, go-jose/v4, zerolog), Flutter (Riverpod, GoRouter, flutter_secure_storage), PostgreSQL, Redis, Docker Compose.

**Standards implemented:** W3C DIDs (did:web), DIDComm v2.1 (ECDH-1PU+A256KW/A256CBC-HS512), W3C VCs 2.0 (VC-JOSE-COSE), OAuth 2.1 + DPoP (RFC 9449), JWS/JCS (RFC 7515/RFC 8785), Adaptive Cards, Bitstring Status List v1.0.

**Known tech debt:**
- IP-based rate limiting not implemented (TODO in api.go)
- Mobile settings screen is placeholder
- Mobile template rendering uses legacy format (not Adaptive Cards)
- Refresh token stored but unused in mobile app

## Constraints

- **Tech stack**: Go 1.22+ backend, Flutter mobile, PostgreSQL, Redis — keep existing infrastructure
- **Crypto**: Ed25519 signing, X25519 encryption. Private keys never leave generating device.
- **Spec compliance**: Implementation must match v1.0-rc1 spec — field names, flows, lifecycle states
- **Mobile**: Flutter with secure enclave keypair, biometric signing

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Strip old signal pipeline | New spec uses DIDComm, not custom signals | ✓ Good |
| Inline ECDH-1PU implementation | go-jose v4 doesn't support X25519; didcomm-go too immature | ✓ Good |
| Keep Flutter scaffold | Navigation, secure storage, Firebase setup reusable | ✓ Good |
| `did:web` for identity | Spec mandates it, widely supported | ✓ Good |
| OAuth 2.1 + DPoP for API auth | Spec mandates it, industry standard | ✓ Good |
| Server-derived X25519 from Ed25519 seed via HKDF | Stable across restarts without new env var | ✓ Good |
| Server stores revocations not approvals | Spec: server stateless w.r.t. approvals | ✓ Good |
| Adaptive Cards for templates | Spec mandates it, industry standard | ✓ Good |
| ConcatKDF with SHA-512 inline | Avoids dependency on immature concatkdf library | ✓ Good |
| 4-phase coarse granularity | Combined Identity+Auth and Credentials+Mobile for speed | ✓ Good |
| Per-entity AES-256-GCM encryption keys | Enables crypto-shredding for GDPR without losing delegation chains | ✓ Good |
| Fan-out in goroutine, 202 response | Org delegation doesn't block approval creation | ✓ Good |

---
*Last updated: 2026-03-17 after v1.0 milestone*
