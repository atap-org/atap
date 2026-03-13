# Technology Stack

**Project:** ATAP v1.0-rc1 (Agent Trust and Authority Protocol)
**Researched:** 2026-03-13

## Existing Stack (Keep)

Already in the codebase, validated and working:

| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.25+ | Backend language |
| Fiber v2 | v2.52.12 | HTTP framework |
| pgx/v5 | v5.7.5 | PostgreSQL driver |
| go-redis/v9 | v9.18.0 | Redis client |
| zerolog | v1.34.0 | Structured logging |
| gowebpki/jcs | v1.0.1 | RFC 8785 JSON Canonicalization |
| oklog/ulid/v2 | v2.1.1 | ULID generation |
| golang.org/x/crypto | v0.48.0 | Ed25519, X25519 |
| Flutter | SDK ^3.11.1 | Mobile app |
| flutter_riverpod | ^3.3.1 | State management |
| go_router | ^17.1.0 | Flutter routing |
| flutter_secure_storage | ^10.0.0 | Keychain/Keystore access |
| ed25519_edwards | ^0.3.1 | Ed25519 in Dart |
| firebase_messaging | ^16.1.2 | Push notifications |

## Recommended New Stack

### JWS / JOSE (Signatures and Tokens)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `github.com/go-jose/go-jose/v4` | v4.x (latest Oct 2025) | JWS signing, JWE encryption, JWK handling | Already an indirect dependency in go.mod. Supports Ed25519, detached JWS payloads (`ParseDetached`, `DetachedCompactSerialize`, `DetachedVerify`), compact and JSON serialization. This is the standard Go JOSE library (successor to square/go-jose). Requires Go 1.24+. | **HIGH** |

**Alternative considered:** `lestrrat-go/jwx/v3` (v3.0.13) -- more feature-rich API with auto-refreshing JWKs and low-level JWS control. However, go-jose/v4 is already a dependency, is simpler, and covers all ATAP needs (JWS with detached payloads, Ed25519, JWK). Adding jwx would introduce a second JOSE library for no clear benefit.

**Do NOT use:** `golang-jwt/jwt` -- JWT-only, no JWS detached payload support, no JWE, no JWK management. Insufficient for the protocol's needs.

### DID Resolution and Documents

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| **Custom `did:web` resolver** | N/A | DID Document hosting and resolution | `did:web` resolution is trivially simple: transform DID to HTTPS URL, fetch JSON. The spec is at W3C CCG. No heavyweight library needed. Build a thin resolver (~100 lines) that fetches `https://{domain}/.well-known/did.json` or `https://{domain}/{path}/did.json`, validates the DID Document structure, and caches results. This avoids pulling in TrustBloc's entire DID framework for one method. | **HIGH** |
| `github.com/trustbloc/did-go` | v1.x | DID Document model types, multi-method resolution | Use ONLY if you need to resolve other DID methods beyond `did:web` (e.g., `did:key` for testing). The library provides DID Document struct types and a VDR (Verifiable Data Registry) interface. Forked from the now-archived Hyperledger Aries Framework Go. Consider carefully -- it brings significant transitive dependencies. | **LOW** |

**Rationale for custom `did:web`:** The `did:web` method spec is 2 pages. Resolution is: (1) replace `:` with `/`, (2) fetch HTTPS URL, (3) parse DID Document JSON. Building this yourself means zero dependency bloat, full control over caching/validation, and no risk of upstream abandonment (the Aries/TrustBloc ecosystem has a history of repo migrations and archival). The ATAP spec only needs `did:web`.

**DID Document model:** Define your own Go structs matching the W3C DID Core v1.1 data model. The DID Document schema is well-defined JSON -- `id`, `verificationMethod`, `authentication`, `keyAgreement`, `service` fields. This is ~200 lines of Go structs + validation.

### DIDComm v2.1 Messaging

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| **Custom DIDComm v2.1 layer** | N/A | Authenticated encryption, message routing | There is no maintained, production-quality standalone DIDComm v2 library for Go as of March 2026. The Hyperledger Aries Framework Go (which had the best DIDComm implementation) was archived. TrustBloc inherited parts but not a clean standalone DIDComm package. The DIF `didcomm-go` project does not exist as a published library. Build DIDComm v2.1 on top of go-jose/v4 (JWE for encryption, JWS for signing) + x/crypto (X25519 key agreement). | **MEDIUM** |

**What DIDComm v2.1 actually requires you to build:**
1. **Message envelope:** JWE (authenticated encryption) using ECDH-ES+A256KW with X25519 keys -- go-jose/v4 supports this
2. **Signed messages:** JWS with Ed25519 -- go-jose/v4 supports this
3. **Plaintext messages:** JSON with `type`, `id`, `from`, `to`, `body` fields -- trivial struct
4. **Routing/forwarding:** Mediator wraps inner message in outer JWE -- composition of the above
5. **Service endpoints:** HTTP POST to DID Document service endpoints -- standard HTTP client

**DIDComm v2.1 changes from v2:** Minor -- allows absent `body` if empty, updated `serviceEndpoint` format for ION DIDs. Backward compatible with v2.

**Risk:** Building DIDComm from primitives is the right call given the ecosystem state, but it is the single highest-effort item. Budget 2-3 weeks for a solid implementation. The alternative (adopting abandoned/archived framework code) is worse.

### W3C Verifiable Credentials 2.0

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `github.com/trustbloc/vc-go` | v1.3.x (latest April 2025) | VC issuance, verification, VC-JOSE-COSE, SD-JWT | The only actively maintained Go library implementing W3C VC 2.0. Supports JWT-based credentials (VC-JOSE-COSE), SD-JWT selective disclosure, Ed25519 signatures, status list validation. Forked from archived Aries Framework Go verifiable package. Published sub-packages updated as recently as April 2025. | **MEDIUM** |

**Why use a library here (but not for DIDs):** VC processing is significantly more complex than DID resolution -- credential schema validation, proof verification chains, selective disclosure mechanics, status list bitstring handling. Building from scratch would be 5-10x the effort vs. using trustbloc/vc-go.

**Risk:** TrustBloc's maintenance velocity is uncertain. The library works but the project's long-term health is unclear. Mitigation: vendor the dependency, write integration tests against the W3C VC test suite, and maintain the ability to fork if needed.

**Alternative considered:** Building VC handling from scratch using go-jose/v4 for JWS + custom VC structs. Feasible for basic JWT-VCs but becomes very complex once you add SD-JWT and Bitstring Status Lists.

### SD-JWT (Selective Disclosure)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `github.com/trustbloc/vc-go/sdjwt` | (bundled) | SD-JWT issuance and verification | Included in trustbloc/vc-go. Implements RFC 9901 (SD-JWT, standardized November 2025). Supports issuer, holder, and verifier flows. | **MEDIUM** |
| `github.com/MichaelFraser99/go-sd-jwt` | latest (Aug 2025) | Standalone SD-JWT alternative | Lighter alternative if you want SD-JWT without the full vc-go dependency. Implements RFC 9901 with key binding JWT support. Consider as fallback if trustbloc/vc-go proves problematic. | **LOW** |

**Recommendation:** Use trustbloc/vc-go's SD-JWT since you're already pulling in vc-go for VCs. Avoid adding a second SD-JWT library.

### Bitstring Status List (Credential Revocation)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| **Custom implementation** | N/A | Credential revocation/suspension status | The Bitstring Status List v1.0 spec (W3C Rec, May 2025) is straightforward: a GZIP-compressed bitstring where each credential gets a fixed index position. Bit = 1 means revoked. No Go library specifically implements this, but trustbloc/vc-go has a `statuslist2021` validator package. Build the issuer-side (set bits, compress, publish as VC) yourself (~200 lines), use trustbloc/vc-go for verification if available. | **MEDIUM** |

### OAuth 2.1 + DPoP

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `github.com/AxisCommunications/go-dpop` | latest | DPoP proof creation and validation (RFC 9449) | The only dedicated Go DPoP library. Supports Ed25519 alongside ES256, RS256, PS256. Small, focused library from Axis Communications. 6 GitHub stars -- small but the code is straightforward and the RFC is well-defined. | **LOW** |
| `golang.org/x/oauth2` | v0.35.0 (already indirect dep) | OAuth 2.1 token flows | Standard Go OAuth2 library. Does NOT support DPoP natively (open issue #651). Use for standard OAuth flows, layer DPoP on top via go-dpop or custom middleware. | **HIGH** |
| **Custom OAuth 2.1 server** | N/A | Authorization server, token endpoint, DPoP validation | OAuth 2.1 is not a radical change from 2.0 -- it mandates PKCE, deprecates implicit flow, requires exact redirect URI matching. Since ATAP is the authorization server (not a client of someone else's), build the token endpoint into the Fiber app. Use go-dpop for DPoP proof validation in middleware. | **MEDIUM** |

**Architecture note:** ATAP acts as both OAuth AS (Authorization Server) and RS (Resource Server). The mobile app and agents are OAuth clients. The OAuth 2.1 + DPoP flow replaces the current `atap_` bearer token system. DPoP binds tokens to the client's key pair, preventing token theft.

**Do NOT use:** `github.com/go-oauth2/oauth2` -- a full OAuth2 server framework. Overkill and opinionated. ATAP needs a thin token endpoint, not a framework.

### Flutter / Dart New Dependencies

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `jose` (pub.dev) | latest | JWS/JWE/JWK/JWT in Dart | The most comprehensive JOSE library for Dart. Supports JWS creation and verification, JWE, JWK, needed for DPoP proof creation and VC verification on mobile. | **MEDIUM** |
| `ssi` (pub.dev, Affinidi) | latest | DID resolution, DID Document handling | Dart SSI library supporting `did:web`, `did:key`, `did:peer` resolution. Provides DID Document model types and resolution. Saves building DID handling from scratch in Dart. | **LOW** |
| `cryptography` | ^2.9.0 | X25519 key agreement, additional crypto | Broader crypto support beyond Ed25519 -- needed for X25519 (DIDComm encryption key agreement). Cross-platform (mobile, desktop, web). The existing `ed25519_edwards` package only does signing. | **MEDIUM** |
| `cryptography_flutter` | ^2.3.4 | Platform-optimized crypto for Flutter | Companion to `cryptography` package. Uses platform-native implementations for better performance on mobile. | **MEDIUM** |
| `local_auth` | latest | Biometric authentication | Required for biometric signing of approvals (spec requirement). Provides fingerprint/face authentication on iOS and Android. | **HIGH** |

**Do NOT use in Flutter:**
- `dart_jsonwebtoken` -- JWT-only, no JWS detached payload support
- `pointycastle` -- low-level crypto, use `cryptography` package instead which wraps it with a usable API

## Stack Summary by Protocol Layer

| Protocol Layer | Standard | Go Library | Dart Library |
|----------------|----------|------------|--------------|
| Identity | W3C DIDs v1.1 (`did:web`) | Custom resolver (~100 LOC) | `ssi` package |
| Claims | W3C VC 2.0 (VC-JOSE-COSE) | `trustbloc/vc-go` | Custom + `jose` |
| Selective Disclosure | SD-JWT (RFC 9901) | `trustbloc/vc-go/sdjwt` | Custom + `jose` |
| Revocation | Bitstring Status List v1.0 | Custom (~200 LOC) | Verify via API |
| Messaging | DIDComm v2.1 | Custom on `go-jose/v4` + `x/crypto` | Custom on `jose` + `cryptography` |
| Authorization | OAuth 2.1 + DPoP (RFC 9449) | `go-dpop` + custom AS | `jose` for DPoP proofs |
| Signatures | JWS (RFC 7515) + JCS (RFC 8785) | `go-jose/v4` + `gowebpki/jcs` | `jose` |
| Encryption | JWE (ECDH-ES+A256KW, X25519) | `go-jose/v4` | `jose` + `cryptography` |

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| JOSE (Go) | go-jose/v4 | lestrrat-go/jwx/v3 | Already a dependency; simpler API; covers all needs |
| DID resolution (Go) | Custom did:web | trustbloc/did-go | did:web is trivial; avoid dependency bloat from archived ecosystem |
| DIDComm (Go) | Custom on go-jose | Aries Framework Go | Archived; no standalone DIDComm package available |
| VC (Go) | trustbloc/vc-go | Build from scratch | VC processing too complex for DIY; SD-JWT alone is weeks of work |
| OAuth DPoP (Go) | go-dpop | Custom DPoP validation | RFC 9449 has edge cases; library handles nonce, replay, expiry |
| JOSE (Dart) | jose | dart_jsonwebtoken | jose supports full JOSE suite; jwt-only packages insufficient |
| DID (Dart) | ssi (Affinidi) | Custom resolver | Provides multi-method resolution; saves effort on mobile |
| Crypto (Dart) | cryptography + ed25519_edwards | pointycastle | Higher-level API; platform-native acceleration via cryptography_flutter |

## Installation

### Go (add to platform/go.mod)

```bash
# JWS/JWE/JWK (may already be indirect -- promote to direct)
go get github.com/go-jose/go-jose/v4

# W3C Verifiable Credentials 2.0
go get github.com/trustbloc/vc-go@latest

# OAuth 2.1 DPoP
go get github.com/AxisCommunications/go-dpop

# Already present (keep)
# github.com/gowebpki/jcs (JCS canonicalization)
# golang.org/x/crypto (Ed25519, X25519)
```

### Flutter (add to mobile/pubspec.yaml)

```yaml
dependencies:
  # JOSE (JWS/JWE/JWK/JWT)
  jose: ^0.3.0
  # SSI / DID resolution
  ssi: ^1.0.0
  # Extended crypto (X25519 key agreement)
  cryptography: ^2.9.0
  cryptography_flutter: ^2.3.4
  # Biometric auth for approval signing
  local_auth: ^2.3.0
  # Already present (keep)
  # ed25519_edwards: ^0.3.1
  # flutter_secure_storage: ^10.0.0
  # crypto: ^3.0.7
```

## Build vs. Buy Summary

| Component | Decision | Effort | Rationale |
|-----------|----------|--------|-----------|
| `did:web` resolver | **Build** | 1-2 days | Trivially simple; no good library |
| DID Document types | **Build** | 1-2 days | Well-defined schema; avoid dependency |
| DIDComm v2.1 | **Build** | 2-3 weeks | No maintained Go library exists |
| VC issuance/verify | **Buy** (trustbloc/vc-go) | 1 week integration | Complex; library saves months |
| SD-JWT | **Buy** (trustbloc/vc-go) | Bundled | RFC 9901 has subtle edge cases |
| Bitstring Status List | **Build** | 2-3 days | Simple spec; no standalone library |
| OAuth 2.1 AS | **Build** | 1-2 weeks | Custom AS needed; thin layer |
| DPoP validation | **Buy** (go-dpop) | 1-2 days integration | Small, focused, correct |
| JWS/JWE primitives | **Buy** (go-jose/v4) | Already present | Industry standard |

## Specification Versions (Pinned)

These are the W3C / IETF specifications the stack implements:

| Specification | Version | Status | Date |
|---------------|---------|--------|------|
| W3C DIDs | v1.1 CR | Candidate Recommendation | March 2026 |
| W3C VC Data Model | v2.0 | W3C Recommendation | May 2025 |
| VC-JOSE-COSE | v1.0 | W3C Recommendation | May 2025 |
| Bitstring Status List | v1.0 | W3C Recommendation | May 2025 |
| DIDComm Messaging | v2.1 | DIF Approved | 2024 |
| did:web Method | v1.0 | W3C CCG Report | Stable |
| OAuth 2.1 | draft | IETF Draft | Ongoing |
| DPoP | RFC 9449 | RFC | September 2023 |
| SD-JWT | RFC 9901 | RFC | November 2025 |
| JWS | RFC 7515 | RFC | May 2015 |
| JCS | RFC 8785 | RFC | June 2020 |

## Sources

- [go-jose/go-jose v4 on pkg.go.dev](https://pkg.go.dev/github.com/go-jose/go-jose/v4) -- HIGH confidence
- [lestrrat-go/jwx on GitHub](https://github.com/lestrrat-go/jwx) -- HIGH confidence
- [trustbloc/vc-go on GitHub](https://github.com/trustbloc/vc-go) -- MEDIUM confidence
- [trustbloc/did-go on GitHub](https://github.com/trustbloc/did-go) -- MEDIUM confidence
- [AxisCommunications/go-dpop on GitHub](https://github.com/AxisCommunications/go-dpop) -- LOW confidence (small project)
- [DIDComm v2.1 Specification](https://identity.foundation/didcomm-messaging/spec/v2.1/) -- HIGH confidence
- [W3C VC-JOSE-COSE Specification](https://w3c.github.io/vc-jose-cose/) -- HIGH confidence
- [W3C Bitstring Status List v1.0](https://www.w3.org/TR/2025/REC-vc-bitstring-status-list-20250515/) -- HIGH confidence
- [RFC 9449 DPoP](https://datatracker.ietf.org/doc/html/rfc9449) -- HIGH confidence
- [RFC 9901 SD-JWT](https://datatracker.ietf.org/doc/rfc9901/) -- HIGH confidence
- [did:web Method Specification](https://w3c-ccg.github.io/did-method-web/) -- HIGH confidence
- [ssi Dart package on pub.dev](https://pub.dev/packages/ssi) -- LOW confidence (unverified version)
- [jose Dart package on pub.dev](https://pub.dev/packages/jose) -- MEDIUM confidence
- [cryptography Dart package on pub.dev](https://pub.dev/packages/cryptography) -- MEDIUM confidence
- [Hyperledger Aries Framework Go (archived)](https://github.com/hyperledger-aries/aries-framework-go) -- context only
- [W3C DIDs v1.1 Candidate Recommendation](https://www.w3.org/TR/did-1.1/) -- HIGH confidence
- [golang/oauth2 DPoP issue #651](https://github.com/golang/oauth2/issues/651) -- HIGH confidence
