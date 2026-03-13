# Domain Pitfalls

**Domain:** Verifiable multi-party authorization protocol (DIDs, DIDComm, VCs, OAuth 2.1 + DPoP, multi-signature approvals)
**Researched:** 2026-03-13

## Critical Pitfalls

Mistakes that cause rewrites, security vulnerabilities, or protocol non-compliance.

### Pitfall 1: aries-framework-go Is Archived -- No Maintained Go DIDComm v2 Library Exists

**What goes wrong:** The project constraint says "Use existing Go DIDComm library (e.g., `github.com/decentralized-identity/didcomm-go`)." That library does not exist as a standalone maintained package. `hyperledger/aries-framework-go` was archived March 2024 and is read-only. Its DIDComm v2 support was partial. There is no drop-in, maintained, standalone Go library for DIDComm v2.1 message packing, routing, and encryption as of March 2026.

**Why it happens:** The DIDComm ecosystem is fragmented. Reference implementations exist in Rust (`didcomm-rs`) and JavaScript, but Go has only the archived Aries framework, which is a massive monolith (VDR, credential handling, wallet, protocols) -- not a focused DIDComm library.

**Consequences:** If you depend on an archived library, you inherit unmaintained dependencies, unpatched CVEs, and API drift from the evolving DIDComm v2.1 spec. If you try to extract DIDComm from Aries, you pull in dozens of transitive dependencies.

**Prevention:**
- Build a thin DIDComm v2.1 layer in-house using Go's `crypto/ed25519`, `golang.org/x/crypto/nacl/box` (X25519), and `github.com/lestrrat-go/jwx` for JOSE operations.
- DIDComm v2.1 message format is JSON with well-defined fields -- the hard part is authenticated encryption (ECDH-ES+A256KW with XC20P or A256GCM), not the message structure itself.
- Use the DIDComm spec directly as your implementation guide rather than wrapping a dead library.
- Consider using only the crypto primitives from `aries-framework-go` (e.g., `pkg/crypto/tinkcrypto`) if they are vendorable, but do not depend on the framework as a whole.

**Detection:** Check `go.mod` for any `hyperledger/aries-framework-go` import. If present, flag immediately.

**Phase:** Must resolve in Phase 1 (DID + DIDComm foundation). This is a blocking architectural decision.

**Confidence:** HIGH -- verified that aries-framework-go is archived via GitHub.

---

### Pitfall 2: did:web Inherits All DNS/TLS Vulnerabilities -- Domain Compromise = Total Identity Takeover

**What goes wrong:** `did:web` resolves DIDs by fetching `https://{domain}/.well-known/did.json` (or a path-based variant). Whoever controls the domain controls the DID Document. Domain hijacking, DNS spoofing, expired domain re-registration, or compromised hosting credentials let an attacker replace the DID Document entirely -- rotating all keys to their own.

**Why it happens:** Unlike `did:key` or `did:peer`, `did:web` has no cryptographic anchoring independent of the web server. It trades decentralization for simplicity. The DID Document is a "naked" file on a web server.

**Consequences:** An attacker who gains control of the domain (even temporarily) can issue credentials, sign approvals, and impersonate any entity whose DID is anchored to that domain. For a multi-tenant platform like ATAP, a single domain compromise affects all entities hosted there.

**Prevention:**
- Implement ATAP's server trust assessment (WebPKI + DNSSEC + audit VCs) from the spec's Section on Server Trust as a first-class feature, not an afterthought.
- Set short `Cache-Control` headers on DID Documents (e.g., `max-age=300`) so resolvers do not cache stale documents indefinitely, but long enough to prevent excessive resolution load.
- Monitor DNS records and certificate transparency logs for the platform domain.
- For high-value entities (orgs), consider supporting `did:web` with DNSSEC as a hard requirement.
- Document the trust model clearly: `did:web` trust = trust in domain operator. This is fine for a hosted platform but must be explicit.

**Detection:** If DID resolution has no cache TTL controls, or if DID Documents are served without DNSSEC verification, flag it.

**Phase:** Phase 1 (DID Document hosting) and Phase 3 (Server Trust assessment).

**Confidence:** HIGH -- did:web spec explicitly documents these security considerations.

---

### Pitfall 3: JWS Detached Payload Without `crit` Header Causes Silent Signature Misinterpretation

**What goes wrong:** The spec requires JWS signatures with detached payload (RFC 7515 + RFC 7797). When using the `"b64": false` header parameter for unencoded payloads, implementations MUST include `"crit": ["b64"]` in the JWS header. Without it, a receiver that does not understand `b64` will base64url-decode the payload before verification -- the signature will still verify, but against the wrong data.

**Why it happens:** RFC 7797 Section 7 explicitly warns about this, but it is easy to forget when constructing JWS headers manually. Many JWS libraries default to standard base64url encoding and do not handle the `b64` parameter at all.

**Consequences:** Signatures verify against incorrect data. A receiver believes a different payload was signed than what was actually signed. This is a silent, exploitable vulnerability in a protocol that depends on signature integrity for authorization.

**Prevention:**
- Enforce `"crit": ["b64"]` in ALL JWS construction code paths. Make it a constant, not a per-call parameter.
- Write a test that constructs a JWS without `crit`, feeds it to the verifier, and asserts rejection.
- Use JCS (RFC 8785) canonicalization of the payload BEFORE signing. The spec already requires this. Canonical JSON + detached payload + `b64: false` + `crit: ["b64"]` is the correct combination.
- Wrap JWS construction in a single function that enforces these invariants. Never construct JWS headers ad-hoc.

**Detection:** Grep for JWS header construction. If `crit` is not always present alongside `b64`, flag as a security vulnerability.

**Phase:** Phase 1 (signature infrastructure). Must be correct from day one.

**Confidence:** HIGH -- RFC 7797 Section 7 explicitly documents this attack.

---

### Pitfall 4: Approval State Machine Race Conditions in Three-Party Flow

**What goes wrong:** The three-party approval flow (from signs -> via validates + co-signs -> to approves/declines) has multiple concurrent mutation points. Two common races: (1) `from` submits the same approval to multiple `via` systems simultaneously, creating duplicate approvals; (2) `to` receives the approval and tries to act on it while `via` is still processing; (3) TTL expiration races with approval action -- the approval expires between the user pressing "approve" and the server processing it.

**Why it happens:** Distributed state machines with multiple independent signers are inherently susceptible to race conditions. Each party maintains their own view of the approval state. Network latency creates windows where state is inconsistent.

**Consequences:** Double-approvals, orphaned approvals that appear valid to one party but expired to another, or approvals that get "consumed" twice. In a protocol designed for non-repudiation, any state inconsistency is a security and audit failure.

**Prevention:**
- Use PostgreSQL advisory locks or `SELECT ... FOR UPDATE` on the approval row when transitioning states. Never use application-level locking.
- Implement optimistic concurrency control: include a version number or `updated_at` timestamp in every state transition. Reject transitions where the version has changed.
- For TTL expiration: use `expires_at` column with a grace period. Check expiry at write time (the DB transaction), not at read time (the API handler).
- Make state transitions idempotent: if `to` sends "approve" twice, the second call returns the same result, not an error.
- The approval ID (`apr_` + ULID) is globally unique. Use it as the idempotency key.

**Detection:** If approval state transitions do not use database-level locking or optimistic concurrency, flag immediately.

**Phase:** Phase 2 (two-party approvals) and Phase 3 (three-party approvals).

**Confidence:** HIGH -- standard distributed systems concern, well-documented in state machine literature.

---

### Pitfall 5: Crypto-Shredding Does Not Automatically Equal GDPR Compliance

**What goes wrong:** The spec calls for crypto-shredding: delete the per-entity encryption key to make all VC data unrecoverable. Teams implement this and assume GDPR Article 17 (right to erasure) is satisfied. It is not. GDPR requires actual deletion, not just rendering data unreadable. Regulators in some jurisdictions do not consider crypto-shredding equivalent to deletion. Additionally, the entity's DID, approval history (approval IDs, timestamps, counterparty DIDs), and metadata may contain personal data even without the VC content.

**Why it happens:** Crypto-shredding is elegant for event-sourced or append-only architectures where you cannot easily delete records. Teams conflate "cryptographically unrecoverable" with "deleted."

**Consequences:** Regulatory non-compliance. Data protection authorities may reject crypto-shredding as insufficient. Worse, if the encryption implementation has flaws (weak key derivation, key material in logs, key cached in Redis), the data is not actually unrecoverable.

**Prevention:**
- Treat crypto-shredding as defense-in-depth, not the sole erasure mechanism. Also DELETE the encrypted data rows after shredding the key.
- Audit all places where entity-identifying information exists: database rows, log files (zerolog output), Redis caches, PostgreSQL WAL/backups. PII must be erasable from all of these.
- Use per-entity encryption keys stored in a dedicated key table, never derived from the entity's signing key (which must remain for delegation chain verification even after erasure).
- Keep approval chain integrity: approval IDs and signatures are not PII. The VC content (email, phone, identity documents) is. Design the schema so shredding VCs does not break approval verification.

**Detection:** If `DELETE FROM` statements are absent from the erasure flow, or if log files contain PII, flag.

**Phase:** Phase 4 (GDPR compliance). But schema design in Phase 1 must anticipate this -- retrofitting crypto-shredding is extremely painful.

**Confidence:** MEDIUM -- legal interpretation varies by jurisdiction. Technical implementation is well-understood.

---

### Pitfall 6: DPoP Proof Validation Has Multiple Subtle Failure Modes

**What goes wrong:** OAuth 2.1 + DPoP (RFC 9449) requires the resource server to validate: (1) the DPoP proof JWT signature, (2) the `ath` claim (SHA-256 hash of the access token), (3) the `htm` and `htu` claims (HTTP method and URL), (4) the `jti` claim uniqueness (replay prevention), (5) the `iat` timestamp (freshness), and optionally (6) a server-provided `nonce`. Missing any one of these checks creates an exploitable vulnerability. Most implementations get 1-3 right but skip 4-6.

**Why it happens:** DPoP validation has 6+ independent checks that must ALL pass. The Go ecosystem has limited DPoP support -- `golang/oauth2` does not support DPoP natively (open issue #651). Available libraries (`AxisCommunications/go-dpop`, `pquerna/dpop`) vary in completeness.

**Consequences:** Without `jti` uniqueness checking, captured DPoP proofs can be replayed within their validity window. Without `ath` validation, a proof can be used with a different access token. Without `nonce` enforcement, proofs can be pre-generated and stockpiled.

**Prevention:**
- Implement DPoP validation as a middleware function that runs ALL checks in a fixed order, failing closed (reject on any check failure).
- Store `jti` values in Redis with TTL matching the proof's maximum acceptable age (e.g., 5 minutes). Use `SET NX` for atomic uniqueness checking.
- The `ath` claim MUST be `base64url(SHA-256(ASCII(access_token)))`. Get the encoding exactly right -- it is the ASCII encoding of the token string, not the raw bytes.
- Implement server nonce (`DPoP-Nonce` header) from the start, even if initially optional. Mobile clients on flaky networks will need clock-skew tolerance, so server nonces are more reliable than `iat` alone.
- Use `AxisCommunications/go-dpop` as a starting point but review its validation completeness.

**Detection:** Write a test matrix: one test per DPoP validation check, each with exactly one invalid field. All must fail. If any passes, the validation is incomplete.

**Phase:** Phase 1 (OAuth 2.1 + DPoP API authentication).

**Confidence:** HIGH -- RFC 9449 documents all required checks explicitly.

---

## Moderate Pitfalls

### Pitfall 7: DIDComm Forward Message Routing Breaks End-to-End Encryption If Mediator Keys Are Mismanaged

**What goes wrong:** DIDComm routing wraps messages in `forward` envelopes, each encrypted to the next mediator. The mediator decrypts the outer envelope to read the routing `next` field but cannot read the inner payload (encrypted to the final recipient). If the ATAP server acts as a mediator (which it does for the three-party flow), it must manage two separate key pairs: one for its mediator role (routing encryption) and one for any direct messaging. Confusing these keys means either the mediator cannot route messages, or it can read message contents it should not.

**Prevention:**
- Maintain explicit key purpose separation in the DID Document: `keyAgreement` keys for DIDComm encryption, `authentication` keys for signing. Different keys for different service endpoints.
- The ATAP server's DID Document should list its mediator service endpoint with its own `keyAgreement` key, separate from any entity-specific keys.
- Test with a three-hop route: sender -> ATAP server (mediator) -> recipient. Verify the server can route but cannot decrypt the inner payload.

**Phase:** Phase 2 (DIDComm messaging) and Phase 3 (three-party flow).

**Confidence:** MEDIUM -- DIDComm routing spec is clear, but implementation details are under-documented for Go.

---

### Pitfall 8: Bitstring Status List Privacy Leak Through Timing and Position Correlation

**What goes wrong:** Bitstring Status List (W3C Recommendation, May 2025) publishes credential revocation status as a compressed bitstring. The spec mandates a minimum bitstring length of 131,072 bits (16KB) to provide group privacy. But if the platform issues credentials sequentially and the status list is updated immediately on revocation, an observer can correlate revocation timing with credential issuance timing to narrow down which credential was revoked.

**Prevention:**
- Randomize the `statusListIndex` assigned to each credential rather than using sequential assignment.
- Batch status list updates on a schedule (e.g., every 15 minutes) rather than publishing immediately on revocation.
- Pre-allocate status list entries in blocks to prevent sequential correlation.
- Set `Cache-Control` headers on the status list endpoint to prevent overly frequent polling.

**Phase:** Phase 4 (credential revocation). Schema design in Phase 2 (VC issuance) should pre-allocate the randomized index.

**Confidence:** HIGH -- W3C spec explicitly discusses this privacy consideration.

---

### Pitfall 9: Flutter Secure Storage Is Not Hardware-Backed by Default on All Android Devices

**What goes wrong:** `flutter_secure_storage` uses Android Keystore on newer devices, but on older devices or certain OEM implementations, it falls back to encrypted shared preferences. Ed25519 private keys stored in "secure storage" may not actually be in hardware-backed secure enclaves. Additionally, Android emulators do not implement hardware Keystore, so tests pass but production fails. On iOS, Keychain is consistently hardware-backed but has its own quirks with biometric access controls.

**Prevention:**
- At app startup, check `android.security.keystore.KeyInfo.isInsideSecureHardware()` (via platform channel) and warn users if their device lacks hardware backing.
- For biometric-gated signing: use `biometric_storage` package (not just `flutter_secure_storage`) which ties key access to biometric authentication at the OS level, not just an app-level check.
- Never test key storage only on emulators. CI must include a real-device test step or at minimum document that emulator testing is insufficient.
- On iOS, set `kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly` to prevent key migration to non-biometric devices.

**Phase:** Phase 2 (mobile approval rendering with biometric signing).

**Confidence:** MEDIUM -- well-documented in Flutter plugin issues, but device-specific behavior is hard to verify comprehensively.

---

### Pitfall 10: JCS Canonicalization Differences Between Go and Dart/Flutter

**What goes wrong:** The spec requires JCS (RFC 8785) canonical JSON for signature payloads. Go's `encoding/json` and Dart's `dart:convert` handle edge cases differently: Unicode escaping (Go escapes `<`, `>`, `&` by default; Dart does not), floating-point serialization (JCS requires specific formatting), and map key ordering (Go's `json.Marshal` sorts keys, but Dart's `jsonEncode` does not by default with `Map<String, dynamic>`). If the Go server and Flutter client canonicalize the same payload differently, signatures created on one side will not verify on the other.

**Prevention:**
- Implement JCS as a dedicated, tested function on both platforms (Go and Dart), not as "just serialize and sort."
- Create a shared test vector file: 20+ JSON payloads with their expected canonical forms. Run the same test vectors on both Go and Dart. Any divergence is a bug.
- In Go, use `json.Marshal` with a custom encoder that does NOT escape HTML characters (set `SetEscapeHTML(false)` on `json.Encoder`). Default `json.Marshal` escapes `<`, `>`, `&` which violates JCS.
- In Dart, sort map keys explicitly before encoding. Do not rely on insertion order.
- Handle edge cases: empty objects `{}`, empty arrays `[]`, null values, nested objects, Unicode characters outside BMP, numbers with trailing zeros.

**Detection:** Cross-platform signature verification test: sign on Go, verify on Dart, and vice versa. If this test does not exist, flag.

**Phase:** Phase 1 (signature infrastructure). Must be correct before any cross-platform signing occurs.

**Confidence:** HIGH -- Go's `SetEscapeHTML` behavior is well-documented and a known JCS pitfall.

---

### Pitfall 11: SD-JWT Disclosure Salt Must Be Cryptographically Random

**What goes wrong:** SD-JWT (RFC 9901, published November 2025) creates selectively disclosable claims by replacing them with salted hashes. The security of selective disclosure depends entirely on the unpredictability of the salt. If salts are generated with a weak PRNG, are too short, or are reused across disclosures, an attacker can brute-force undisclosed claims.

**Prevention:**
- Use `crypto/rand` in Go (never `math/rand`) for salt generation. Use `dart:math`'s `Random.secure()` in Dart.
- Salt length: minimum 128 bits (16 bytes) as recommended by the spec.
- Never reuse salts across different disclosures, even within the same credential.
- Use `github.com/MichaelFraser99/go-sd-jwt` as the Go implementation. Verify it uses `crypto/rand` internally before depending on it.

**Phase:** Phase 3 (SD-JWT selective disclosure).

**Confidence:** HIGH -- RFC 9901 security considerations section explicitly warns about this.

---

### Pitfall 12: OAuth 2.1 Requires PKCE for All Clients -- No Exceptions

**What goes wrong:** OAuth 2.1 (draft) mandates PKCE (Proof Key for Code Exchange) for all clients, including confidential clients. Teams familiar with OAuth 2.0 skip PKCE for server-to-server flows because it was only required for public clients. OAuth 2.1 removes the implicit grant entirely and requires PKCE universally.

**Prevention:**
- Implement PKCE in the authorization server from day one, for all grant types that use authorization codes.
- The Flutter app (public client) and any agent/machine registrations (confidential clients) must both use PKCE.
- Use `S256` challenge method only. `plain` is allowed by the spec but provides no security benefit and should be rejected.
- Store PKCE code verifiers in memory only (not in persistent storage). They are single-use.

**Phase:** Phase 1 (OAuth 2.1 API authentication).

**Confidence:** HIGH -- OAuth 2.1 draft is explicit about this change from OAuth 2.0.

---

## Minor Pitfalls

### Pitfall 13: DID Document `@context` Must Be Exact

**What goes wrong:** DID Documents require specific `@context` entries. Including the wrong context URL, using HTTP instead of HTTPS, or omitting required contexts causes resolver failures. The DID Core context is `https://www.w3.org/ns/did/v1` and DIDComm service endpoints require `https://didcomm.org/messaging/contexts/v2`. Getting these wrong causes interoperability failures with external resolvers.

**Prevention:** Define context URLs as constants. Never construct them dynamically. Test DID Document validation against a reference resolver.

**Phase:** Phase 1 (DID Document hosting).

---

### Pitfall 14: Redis Pub/Sub Message Loss During Reconnection

**What goes wrong:** If the ATAP server uses Redis pub/sub for real-time DIDComm message delivery (replacing the old SSE pipeline), messages published during a subscriber reconnection window are lost. Redis pub/sub is fire-and-forget -- there is no replay.

**Prevention:** Use Redis Streams (XADD/XREAD with consumer groups) instead of pub/sub for message delivery. Streams support replay via consumer group acknowledgment. Alternatively, use PostgreSQL LISTEN/NOTIFY with a WAL-backed message table for guaranteed delivery.

**Phase:** Phase 2 (DIDComm message delivery).

---

### Pitfall 15: Fiber v2 Context Handling With Long-Lived Connections

**What goes wrong:** Fiber v2 uses fasthttp under the hood. `*fiber.Ctx` must NOT be stored or used after the handler returns -- it is pooled and reused. If the DIDComm message handler stores the context for async processing (e.g., waiting for a mediator response), the context will be corrupted.

**Prevention:** Extract all needed values (headers, body, params) from `*fiber.Ctx` into local variables or a request DTO at the start of the handler. Never pass `*fiber.Ctx` to goroutines or async workflows. If upgrading to Fiber v3 (which uses `net/http`), this pitfall disappears.

**Phase:** Phase 1 (API layer).

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| DID + DIDComm foundation | No maintained Go DIDComm library (#1) | Build thin in-house layer on JOSE primitives |
| DID Document hosting | did:web DNS/TLS dependency (#2) | Server trust assessment, DNSSEC, short cache TTL |
| Signature infrastructure | JWS `crit` header omission (#3) | Single JWS construction function, enforce invariants |
| Signature infrastructure | JCS cross-platform divergence (#10) | Shared test vectors, Go `SetEscapeHTML(false)` |
| OAuth 2.1 + DPoP | Incomplete DPoP validation (#6) | Test matrix: one invalid field per test |
| OAuth 2.1 + DPoP | Missing PKCE for confidential clients (#12) | PKCE for all clients, `S256` only |
| Approval lifecycle | State machine race conditions (#4) | PostgreSQL row locking, optimistic concurrency |
| DIDComm messaging | Mediator key confusion (#7) | Explicit key purpose in DID Document |
| Mobile biometric signing | Insecure key storage on Android (#9) | Hardware backing check, `biometric_storage` package |
| VC issuance | SD-JWT weak salts (#11) | `crypto/rand` only, 128-bit minimum |
| Credential revocation | Status list timing correlation (#8) | Randomized index, batched updates |
| GDPR erasure | Crypto-shredding insufficiency (#5) | Delete data rows after key destruction, audit all PII locations |
| API layer | Fiber context pooling (#15) | Extract values immediately, never store ctx |
| Real-time delivery | Redis pub/sub message loss (#14) | Use Redis Streams or PostgreSQL-backed delivery |

## Sources

- [RFC 9449 - OAuth 2.0 DPoP](https://datatracker.ietf.org/doc/html/rfc9449) -- HIGH confidence
- [RFC 7797 - JWS Unencoded Payload Option](https://datatracker.ietf.org/doc/html/rfc7797) -- HIGH confidence
- [RFC 9901 - SD-JWT](https://datatracker.ietf.org/doc/rfc9901/) -- HIGH confidence
- [W3C Bitstring Status List v1.0](https://www.w3.org/TR/vc-bitstring-status-list/) -- HIGH confidence
- [did:web Method Specification](https://w3c-ccg.github.io/did-method-web/) -- HIGH confidence
- [DIDComm v2 Specification](https://identity.foundation/didcomm-messaging/spec/) -- HIGH confidence
- [DIDComm v2 Routing](https://didcomm.org/book/v2/routing/) -- MEDIUM confidence
- [aries-framework-go archived](https://github.com/hyperledger-archives/aries-framework-go) -- HIGH confidence
- [golang/oauth2 DPoP issue #651](https://github.com/golang/oauth2/issues/651) -- HIGH confidence
- [AxisCommunications/go-dpop](https://github.com/AxisCommunications/go-dpop) -- MEDIUM confidence
- [MichaelFraser99/go-sd-jwt](https://github.com/MichaelFraser99/go-sd-jwt) -- MEDIUM confidence
- [flutter_secure_storage](https://pub.dev/packages/flutter_secure_storage) -- MEDIUM confidence
- [DPoP Nonce analysis](https://darutk.medium.com/dpop-nonce-9787b9d276d1) -- MEDIUM confidence
- [Crypto-shredding analysis](https://medium.com/@brentrobinson5/crypto-shredding-how-it-can-solve-modern-data-retention-challenges-da874b01745b) -- MEDIUM confidence
- [Crypto-shredding GDPR limitations](https://secupi.com/crypto-shredding-is-not-nirvana-for-right-of-erasure-or-rtbf-compliance/) -- MEDIUM confidence
