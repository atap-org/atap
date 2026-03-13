# Project Research Summary

**Project:** ATAP v1.0-rc1 (Agent Trust and Authority Protocol)
**Domain:** Verifiable multi-party authorization protocol (DIDs, DIDComm, VCs, OAuth 2.1 + DPoP)
**Researched:** 2026-03-13
**Confidence:** MEDIUM

## Executive Summary

ATAP v1.0-rc1 is a protocol server implementing W3C Decentralized Identifiers (did:web), DIDComm v2.1 messaging, W3C Verifiable Credentials 2.0, and a novel multi-signature approval engine -- all secured by OAuth 2.1 with DPoP sender-constrained tokens. The existing Go/Fiber/pgx/Redis codebase provides a solid foundation, but the v1.0-rc1 spec replaces the current signal/inbox/webhook model entirely with standards-based DID identity, DIDComm messaging, and VC-backed trust levels. This is not an incremental upgrade; it is a protocol rewrite on top of existing infrastructure.

The recommended approach is to build bottom-up from cryptographic primitives through identity, auth, messaging, and finally the approval engine. The most critical finding across all research: there is no maintained Go DIDComm v2.1 library. The archived aries-framework-go must not be used. Instead, build a thin DIDComm layer on go-jose/v4 and x/crypto primitives. This is the single highest-effort and highest-risk item (2-3 weeks). For Verifiable Credentials, use trustbloc/vc-go despite uncertain long-term maintenance -- building VC processing from scratch would take months. For everything else (did:web resolution, Bitstring Status List, OAuth 2.1 AS), build custom implementations because the specs are simple enough and the Go library ecosystem is thin.

Key risks are: (1) the DIDComm build taking longer than estimated since there is no reference Go implementation to lean on, (2) JWS detached payload handling with the `crit` header being silently wrong across Go and Dart if not tested with shared vectors from day one, (3) approval state machine race conditions in the three-party flow requiring database-level locking, and (4) DPoP validation having six independent checks that must all pass -- missing any one creates an exploitable vulnerability. The mitigation strategy is rigorous: cross-platform test vectors for all crypto operations, database-level concurrency control for state machines, and a DPoP validation test matrix with one-invalid-field-per-test coverage.

## Key Findings

### Recommended Stack

The existing Go 1.25+, Fiber v2, pgx/v5, go-redis/v9, and zerolog stack is retained. The key additions are go-jose/v4 (already an indirect dependency) for all JOSE operations, trustbloc/vc-go for W3C VC 2.0 processing, and go-dpop for DPoP proof validation. On Flutter, add the jose package for JOSE operations, the cryptography package for X25519, and local_auth for biometric signing.

**Core new technologies (Go):**
- `go-jose/v4`: JWS, JWE, JWK -- already indirect dep, promote to direct. Covers detached payloads, Ed25519, X25519. HIGH confidence.
- `trustbloc/vc-go`: W3C VC 2.0 issuance, verification, SD-JWT. Only maintained Go VC library. MEDIUM confidence (maintenance uncertain).
- `go-dpop`: DPoP proof validation (RFC 9449). Small but focused. LOW confidence (6 stars, but code is sound).
- Custom did:web resolver: ~100 LOC. No library needed for a 2-page spec.
- Custom DIDComm v2.1 layer: Built on go-jose/v4 + x/crypto. 2-3 weeks effort. No alternative exists.

**Core new technologies (Flutter/Dart):**
- `jose`: Full JOSE suite for DPoP proofs and VC verification on mobile.
- `cryptography` + `cryptography_flutter`: X25519 key agreement for DIDComm encryption.
- `local_auth`: Biometric authentication for approval signing.

**Build vs. buy summary:** Build did:web (trivial), DIDComm v2.1 (no library), Bitstring Status List (simple spec), OAuth 2.1 AS (custom needed). Buy VC processing (complex), SD-JWT (bundled with vc-go), DPoP validation (edge cases), JOSE primitives (go-jose/v4).

### Expected Features

**Must have (table stakes for spec compliance):**
- DID Document hosting and resolution (did:web) -- entities must be cryptographically addressable
- DIDComm v2.1 plaintext, signed, and encrypted messages -- the messaging transport
- OAuth 2.1 + DPoP authentication -- replaces current atap_ bearer tokens
- W3C VC 2.0 issuance and verification -- attestations that feed trust levels
- Two-party and three-party approval flows -- the core protocol
- Approval state machine with JWS signatures -- non-repudiation
- Server discovery (/.well-known/atap.json) -- client bootstrapping

**Should have (ATAP differentiators):**
- Multi-signature approval model with three independent signers -- this IS the protocol
- Signed approval templates with brand rendering -- UX differentiator for the via party
- Mobile biometric approval signing -- human-in-the-loop with hardware key protection
- Trust level system (0-3) derived from VCs -- quantified trust for policy decisions
- Chained approvals with parent references -- auditable delegation chains

**Defer to post-v1.0:**
- SD-JWT selective disclosure -- valuable but not blocking core flows
- Bitstring Status List revocation -- use simple revocation flag initially
- Organization delegate routing -- extension of core model
- Cross-server federation -- explicitly out of scope
- Crypto-shredding for GDPR -- important for production, not protocol-critical
- Server trust assessment -- requires multiple servers

### Architecture Approach

The server has seven subsystems organized in a strict dependency hierarchy: crypto primitives at the bottom, then DID management, then auth + DIDComm messaging, then credentials + trust + templates, then the approval engine at the top, with HTTP handlers as the transport layer. The existing monolithic handler pattern must be refactored into a domain service layer where thin HTTP handlers delegate to service interfaces. The DIDComm mediator replaces the SSE/Redis pub/sub notification system. Redis Streams (not pub/sub) should be used for guaranteed message delivery.

**Major components:**
1. **Crypto Service** -- Ed25519/X25519 keys, JWS (detached), JCS canonicalization, all JOSE operations
2. **DID Management** -- DID Document creation, storage, did:web hosting, key rotation
3. **OAuth Engine** -- Token issuance, DPoP validation, replaces current auth middleware
4. **DIDComm Mediator** -- Message pack/unpack, routing, queue for offline entities
5. **Approval Engine** -- Multi-sig lifecycle, state machine, signature accumulation
6. **VC Issuer/Verifier** -- Credential issuance, verification, trust level derivation
7. **Template Engine** -- Branded approval templates, JWS verification

### Critical Pitfalls

1. **No maintained Go DIDComm library** -- Build in-house on go-jose/v4 + x/crypto. Budget 2-3 weeks. Do NOT use archived aries-framework-go.
2. **JWS detached payload `crit` header omission** -- Without `"crit": ["b64"]`, signatures verify against wrong data. Wrap JWS construction in a single function enforcing this invariant. Create cross-platform test vectors.
3. **Approval state machine race conditions** -- Use PostgreSQL `SELECT ... FOR UPDATE` or advisory locks. Make transitions idempotent. Check TTL expiry at DB transaction time, not API handler time.
4. **DPoP validation has 6 independent checks** -- Missing any one is exploitable. Write a test matrix with one-invalid-field-per-test. Store jti in Redis with TTL for replay prevention.
5. **JCS canonicalization divergence between Go and Dart** -- Go's json.Marshal escapes HTML by default (violates JCS). Use SetEscapeHTML(false). Create shared test vector file for both platforms.

## Implications for Roadmap

Based on combined research, the architecture dependency graph and feature dependencies converge on a six-phase structure. The ordering is driven by hard technical dependencies, not feature priority.

### Phase 1: Crypto + Identity Foundation
**Rationale:** Everything depends on DIDs. You cannot verify signatures, issue credentials, or send DIDComm messages without DID resolution working. Crypto primitives (JWS detached, X25519 derivation, JCS) must be correct before any signing occurs.
**Delivers:** Ed25519/X25519 key management, JWS with detached payloads, JCS canonicalization, DID Document model, did:web hosting at /.well-known/did.json and /entities/{id}/did.json, did:web resolution (outbound), server DID, server discovery endpoint.
**Addresses:** DID Document hosting, resolution, entity registration with DID creation, Ed25519/X25519 verification methods.
**Avoids:** JWS crit header omission (Pitfall 3), JCS cross-platform divergence (Pitfall 10), DID Document @context errors (Pitfall 13).
**Stack:** go-jose/v4, gowebpki/jcs (existing), x/crypto (existing), custom did:web resolver.

### Phase 2: OAuth 2.1 + DPoP Authentication
**Rationale:** API auth must work before building any new authenticated endpoints. DPoP depends on DID key material from Phase 1. This replaces the current atap_ bearer token system.
**Delivers:** OAuth 2.1 authorization server (token endpoint), DPoP proof validation middleware, PKCE for all clients, key-bound access tokens with cnf.jkt claim.
**Addresses:** OAuth 2.1 token endpoint, DPoP proof validation, token introspection.
**Avoids:** Incomplete DPoP validation (Pitfall 6), missing PKCE for confidential clients (Pitfall 12).
**Stack:** go-dpop, golang.org/x/oauth2, custom OAuth AS in Fiber.

### Phase 3: DIDComm v2.1 Messaging
**Rationale:** Approvals need DIDComm for delivery. This is the highest-effort custom build (2-3 weeks) and replaces the existing SSE/Redis notification system. Must be complete before the approval engine.
**Delivers:** DIDComm plaintext, signed, and encrypted message formats. Mediator with message queuing (Redis Streams). Forward protocol for routing. Service endpoints in DID Documents.
**Addresses:** All DIDComm messaging table stakes, server as mediator for hosted entities.
**Avoids:** Using archived aries-framework-go (Pitfall 1), mediator key confusion (Pitfall 7), Redis pub/sub message loss (Pitfall 14), Fiber context pooling issues (Pitfall 15).
**Stack:** go-jose/v4 (JWE/JWS), x/crypto (X25519), go-redis/v9 (Streams), custom DIDComm layer.

### Phase 4: Approval Engine
**Rationale:** This is the product. Everything before it is infrastructure. The two-party flow proves the model; the three-party flow is the differentiator.
**Delivers:** Two-party and three-party approval flows, approval state machine (requested -> approved/declined/expired/rejected -> consumed/revoked), JWS signature accumulation, TTL expiry, approval delivery via DIDComm.
**Addresses:** All approval table stakes, multi-signature approval model (key differentiator).
**Avoids:** State machine race conditions (Pitfall 4) via PostgreSQL locking and idempotent transitions.
**Stack:** PostgreSQL (advisory locks/FOR UPDATE), Redis (event pub), DIDComm mediator (delivery).

### Phase 5: Credentials + Trust
**Rationale:** Credentials raise trust levels but are not required for basic approvals. Build after the approval engine works so the core protocol is validated first.
**Delivers:** VC issuance (email, phone, personhood), VC verification, credential schemas, trust level derivation (0-3), VC storage with per-entity encryption keys.
**Addresses:** All VC table stakes, trust level system differentiator.
**Avoids:** SD-JWT weak salts (Pitfall 11) if SD-JWT is included.
**Stack:** trustbloc/vc-go, custom Bitstring Status List (~200 LOC).

### Phase 6: Templates + Mobile
**Rationale:** Templates enhance UX but approvals work without them (plain text fallback). Mobile depends on all server APIs being stable. This is where the full end-to-end experience comes together.
**Delivers:** Signed approval templates with brand rendering, mobile approval UI, biometric signing, push notification integration for approval delivery.
**Addresses:** Signed templates differentiator, mobile biometric signing differentiator.
**Avoids:** Insecure key storage on Android (Pitfall 9).
**Stack:** Flutter (jose, cryptography, cryptography_flutter, local_auth, firebase_messaging).

### Phase Ordering Rationale

- **Strict dependency chain:** Each phase produces artifacts consumed by the next. DIDs before auth (DPoP needs keys), auth before DIDComm (authenticated endpoints), DIDComm before approvals (delivery mechanism), approvals before credentials (core protocol first).
- **Risk-first ordering:** The two highest-risk items (custom DIDComm build and DPoP validation) are in Phases 2-3, surfacing problems early.
- **Architecture alignment:** The six phases map directly to the six architecture layers (Layer 0-5), ensuring each phase produces a complete, testable subsystem.
- **Feature coherence:** Table stakes are covered in Phases 1-4; differentiators in Phases 4-6; deferred features stay out entirely.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (DIDComm):** Highest-effort custom build. DIDComm v2.1 authenticated encryption (ECDH-ES+A256KW) implementation details are under-documented for Go. The mediator queue design needs validation against real mobile connectivity patterns.
- **Phase 4 (Approvals):** The three-party approval flow with template injection is novel -- no reference implementation exists. State machine concurrency patterns need careful design.
- **Phase 5 (Credentials):** trustbloc/vc-go integration may surface compatibility issues. SD-JWT (if included) adds complexity.

Phases with standard patterns (skip deep research):
- **Phase 1 (Crypto + Identity):** Well-documented specs (JWS, JCS, did:web). Existing Go crypto code provides foundation.
- **Phase 2 (OAuth):** OAuth 2.1 + DPoP are well-specified RFCs with clear validation requirements.
- **Phase 6 (Mobile):** Flutter biometric auth and JOSE operations are well-documented. Main risk is Android device fragmentation, which is a testing concern, not a research gap.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM-HIGH | Core stack (Go, go-jose, x/crypto) is HIGH. trustbloc/vc-go and go-dpop are MEDIUM due to uncertain maintenance. Dart packages (ssi, jose) are LOW-MEDIUM. |
| Features | MEDIUM | Based on W3C/DIF/IETF specs (HIGH) but spec file itself was not directly parseable. Feature prioritization inferred from dependency analysis and protocol coherence. |
| Architecture | HIGH | Clean dependency hierarchy confirmed across all research files. Build order is unambiguous from architecture layers. |
| Pitfalls | HIGH | 12 of 15 pitfalls sourced from RFCs and W3C specs with explicit security consideration sections. Practical mitigations are concrete. |

**Overall confidence:** MEDIUM -- strong on architecture and pitfalls, moderate on stack (due to ecosystem fragility in DIDComm and VC Go libraries), moderate on features (spec not directly verified).

### Gaps to Address

- **DIDComm v2.1 encryption implementation details:** The Go implementation of ECDH-ES+A256KW with XC20P using go-jose/v4 needs a proof-of-concept before committing to the approach. Validate in Phase 3 planning.
- **trustbloc/vc-go API stability:** The library's last publish was April 2025. Verify current API compatibility and consider vendoring the dependency.
- **Dart SSI package maturity:** The `ssi` package on pub.dev has LOW confidence. May need a custom Dart did:web resolver instead.
- **Existing schema migration path:** The architecture calls for dropping signals, channels, webhooks, claims, and delegations tables. The migration strategy from current schema to v1.0-rc1 schema needs explicit planning.
- **Three-party approval UX on mobile:** How the branded template rendering works in Flutter (safe HTML? Native widgets? WebView?) is unresearched.

## Sources

### Primary (HIGH confidence)
- [DIDComm v2.1 Specification](https://identity.foundation/didcomm-messaging/spec/v2.1/) -- message formats, mediator protocol, routing
- [W3C DIDs v1.1](https://www.w3.org/TR/did-1.1/) -- DID Document model, verification methods
- [W3C VC Data Model v2.0](https://www.w3.org/TR/vc-data-model-2.0/) -- credential structure, issuance, verification
- [W3C VC-JOSE-COSE v1.0](https://www.w3.org/TR/vc-jose-cose/) -- JWT-based credential securing
- [W3C Bitstring Status List v1.0](https://www.w3.org/TR/vc-bitstring-status-list/) -- credential revocation
- [RFC 9449 DPoP](https://datatracker.ietf.org/doc/html/rfc9449) -- sender-constrained tokens
- [RFC 9901 SD-JWT](https://datatracker.ietf.org/doc/rfc9901/) -- selective disclosure
- [RFC 7797 JWS Unencoded Payload](https://datatracker.ietf.org/doc/html/rfc7797) -- detached JWS
- [RFC 8785 JCS](https://datatracker.ietf.org/doc/html/rfc8785) -- JSON canonicalization
- [did:web Method Specification](https://w3c-ccg.github.io/did-method-web/) -- DID resolution via HTTPS
- [go-jose/go-jose v4](https://pkg.go.dev/github.com/go-jose/go-jose/v4) -- Go JOSE library

### Secondary (MEDIUM confidence)
- [trustbloc/vc-go](https://github.com/trustbloc/vc-go) -- W3C VC Go library, active but maintenance uncertain
- [AxisCommunications/go-dpop](https://github.com/AxisCommunications/go-dpop) -- Go DPoP validation
- [jose Dart package](https://pub.dev/packages/jose) -- Dart JOSE library
- [cryptography Dart package](https://pub.dev/packages/cryptography) -- Dart X25519 and crypto
- [W3C VC Rendering Methods](https://www.w3.org/TR/vc-render-method/) -- template rendering (Working Draft)

### Tertiary (LOW confidence)
- [ssi Dart package](https://pub.dev/packages/ssi) -- DID resolution in Dart, version unverified
- [MichaelFraser99/go-sd-jwt](https://github.com/MichaelFraser99/go-sd-jwt) -- standalone SD-JWT alternative
- [trustbloc/did-go](https://github.com/trustbloc/did-go) -- DID framework, only if multi-method resolution needed

---
*Research completed: 2026-03-13*
*Ready for roadmap: yes*
