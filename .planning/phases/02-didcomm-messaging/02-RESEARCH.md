# Phase 2: DIDComm Messaging - Research

**Researched:** 2026-03-13
**Domain:** DIDComm v2.1, JOSE/JWE cryptography, X25519 key agreement, ECDH-1PU, Go implementation
**Confidence:** MEDIUM (DIDComm v2.1 spec verified; no maintained Go DIDComm library — custom build required)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MSG-01 | All entity-to-entity communication uses DIDComm v2.1 | DIDComm v2.1 message envelope structure, content types, and required fields documented below |
| MSG-02 | Server acts as DIDComm mediator for hosted entities (untrusted relay layer) | Forward protocol, message queuing/pickup pattern, PostgreSQL message storage documented below |
| MSG-03 | Server acts as ATAP system participant (`via`) for approval co-signing (trusted layer) | Server identity model (DID document + X25519 key), the server is the `skid` signer — architecture pattern documented below |
| MSG-04 | DIDComm authenticated encryption (ECDH-1PU + XC20P) for message confidentiality | **CRITICAL SPEC DISCREPANCY documented below** — actual required algorithm combination differs from requirement text |
| MSG-05 | ATAP message types under `https://atap.dev/protocols/` for all approval lifecycle events | Custom protocol namespace design — no external spec; patterns for type URI design documented below |
| API-05 | DIDComm endpoint: POST /v1/didcomm | Handler wiring, DPoP-auth vs unauthenticated decision, content type `application/didcomm-encrypted+json` |
</phase_requirements>

---

## Summary

Phase 2 builds a server-side DIDComm v2.1 mediator in Go. There is no maintained Go DIDComm library — this is a confirmed ecosystem gap (noted in STATE.md). The implementation must be built on primitives: `crypto/ecdh` (Go stdlib, X25519), `golang.org/x/crypto/chacha20poly1305` (XC20P), `go-jose/v4` (JWE A256CBC-HS512 content encryption), and standard HMAC-SHA512 for ConcatKDF.

**Critical algorithm clarification:** The requirements text says "ECDH-1PU + XC20P" but DIDComm v2.1 specifies these as two separate modes — authcrypt uses ECDH-1PU+A256KW with A256CBC-HS512; anoncrypt uses ECDH-ES+A256KW with XC20P. The phase implements authcrypt (ECDH-1PU) because it provides sender authentication. The project requirement text "ECDH-1PU + XC20P" should be interpreted as "ECDH-1PU for key wrapping + A256CBC-HS512 for content encryption" or may reflect an earlier Aries-RFC envelope that combined these differently. A decision is needed before implementation.

**Primary recommendation:** Build a minimal `internal/didcomm` package with three clear layers: (1) JWE envelope encryption/decryption, (2) message routing handler (POST /v1/didcomm), (3) message queue (PostgreSQL table + Redis Streams for live delivery). The DID document model must be extended to include X25519 keyAgreement keys and a DIDCommMessaging service endpoint.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/ecdh` (Go stdlib) | Go 1.20+ | X25519 key generation + ECDH scalar multiplication | Official Go X25519 — freezes golang.org/x/crypto/curve25519 |
| `golang.org/x/crypto/chacha20poly1305` | v0.x (already in go.sum) | XChaCha20-Poly1305 (XC20P) AEAD | Official Go extended crypto; 24-byte nonce, no collision risk |
| `go-jose/go-jose/v4` | v4.x (already in go.mod as indirect) | JWE A256CBC-HS512 + AES-KW, JWS signing | Already in dependency graph; handles JWE protected headers |
| `golang.org/x/crypto` | already in go.mod | HKDF / general crypto primitives | Already present |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/hmac` + `crypto/sha512` (Go stdlib) | stdlib | ConcatKDF for ECDH-1PU key derivation (SHA-512) | ECDH-1PU KDF per NIST SP 800-56C |
| `github.com/golang-crypto/concatkdf` | v0.1.1 | Concat KDF helper | Optional helper; low-maintenance (v0.x) — manual KDF is safer |
| `github.com/oklog/ulid/v2` | already in go.mod | Message IDs | Use `msg_` + ULID |
| `pgx/v5` (already in go.mod) | v5.x | Message queue storage | Already present |
| `go-redis/v9` (already in go.mod) | v9.x | Live message delivery via Redis Streams | Already present |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom ECDH-1PU | `github.com/hyperledger/aries-framework-go` crypto | Aries is unmaintained/archived; adds enormous dependency tree |
| Manual JWE assembly | `github.com/lestrrat-go/jwx/v2` | Adds another JOSE library; go-jose already present |
| PostgreSQL queue | Redis-only queue | PostgreSQL survives server restart; Redis is ephemeral delivery layer only |

**Installation:**
```bash
# All core deps already in go.mod as indirect — promote to direct when imported
# No new packages required; confirm with:
cd /Users/svenloth/dev/atap/platform && go mod tidy
```

---

## Architecture Patterns

### Recommended Project Structure

```
platform/internal/
├── didcomm/
│   ├── envelope.go        # JWE encrypt/decrypt (ECDH-1PU + A256CBC-HS512 and/or XC20P)
│   ├── envelope_test.go   # Round-trip tests with fixed test vectors
│   ├── message.go         # DIDComm plaintext message types + ATAP protocol types
│   ├── mediator.go        # Forward message routing logic
│   └── queue.go           # Queue interface (store + deliver)
├── api/
│   └── didcomm.go         # POST /v1/didcomm handler
└── store/
    └── messages.go        # Message queue store (PostgreSQL)
platform/migrations/
└── 010_didcomm_messages.up.sql   # messages table + indexes
```

### DID Document Extension (X25519 + DIDCommMessaging service)

Every entity's DID Document must be extended with:
1. An X25519 `keyAgreement` verification method (`X25519KeyAgreementKey2020`)
2. A `DIDCommMessaging` service endpoint

The server must persist a per-entity X25519 keypair alongside the Ed25519 signing key. **This requires a DB schema change**: either add `public_key_x25519` to `entities`, or add a `key_type` column to `key_versions`.

Recommended approach: add `x25519_private_key` (encrypted at rest eventually, plaintext for now) and `x25519_public_key` columns to `entities`. Simpler than versioning X25519 keys separately in Phase 2.

**DID Document service block (per DIDComm v2.1 spec):**
```json
{
  "service": [{
    "id": "did:web:atap.app:agent:01jxxx#didcomm",
    "type": "DIDCommMessaging",
    "serviceEndpoint": {
      "uri": "https://atap.app/v1/didcomm",
      "accept": ["didcomm/v2"],
      "routingKeys": []
    }
  }]
}
```

**keyAgreement block:**
```json
{
  "keyAgreement": ["did:web:atap.app:agent:01jxxx#key-x25519-1"],
  "verificationMethod": [{
    "id": "did:web:atap.app:agent:01jxxx#key-x25519-1",
    "type": "X25519KeyAgreementKey2020",
    "controller": "did:web:atap.app:agent:01jxxx",
    "publicKeyMultibase": "z6LS..."
  }]
}
```

X25519 multibase encoding: `"z"` prefix + base58btc of raw 32-byte public key (same convention as `EncodePublicKeyMultibase` for Ed25519, already in `crypto/did.go`).

### Pattern 1: DIDComm JWE Envelope (Authcrypt)

**What:** ECDH-1PU authenticated encryption — proves sender identity, hides content
**When to use:** All ATAP entity-to-entity messages (server acting as both mediator and `via`)

**Algorithm combination for authcrypt (per DIDComm v2.1 spec):**
- `alg`: `ECDH-1PU+A256KW` (key wrapping)
- `enc`: `A256CBC-HS512` (content encryption)
- Curve: X25519

**Critical:** The ECDH-1PU draft mandates A256CBC-HS512 for content encryption. XC20P is the default for *anoncrypt* (ECDH-ES), not authcrypt.

**ECDH-1PU key derivation (ConcatKDF construction):**
```go
// Source: IETF draft-madden-jose-ecdh-1pu-04
// Ze = ECDH(sender_ephemeral_priv, recipient_pub)  // ephemeral-static
// Zs = ECDH(sender_static_priv, recipient_pub)      // static-static
// Z  = Ze || Zs                                      // concatenate
// CEK = ConcatKDF(Z, alg, apu, apv, keylen)          // SHA-512 based

senderEphemPriv, senderEphemPub, _ := generateX25519KeyPair()
Ze, _ := ecdh.X25519().ECDH(senderEphemPriv, recipientPub)
Zs, _ := ecdh.X25519().ECDH(senderStaticPriv, recipientPub)
Z := append(Ze, Zs...)
wrappingKey := concatKDF(Z, "ECDH-1PU+A256KW", apu, apv, 256)
// wrappingKey wraps the random CEK via AES-256-KW
// CEK encrypts payload with A256CBC-HS512
```

**Required JWE protected headers:**
```json
{
  "alg": "ECDH-1PU+A256KW",
  "enc": "A256CBC-HS512",
  "epk": { "kty": "OKP", "crv": "X25519", "x": "<base64url_ephemeral_pub>" },
  "apu": "<base64url(sender_kid)>",
  "apv": "<base64url(sha256(sorted_recipient_kids))>",
  "skid": "<sender_kid>"
}
```

**Tag-in-KDF requirement:** The CEK wrapping key derivation must include the ciphertext authentication tag from the content encryption step. Build order: encrypt payload first → get tag → then wrap CEK (this is inverse of naive JWE construction).

### Pattern 2: DIDComm Plaintext Message Structure

```go
// Source: DIDComm Messaging Specification v2.1
type PlaintextMessage struct {
    ID          string            `json:"id"`           // unique per sender
    Type        string            `json:"type"`         // URI
    From        string            `json:"from,omitempty"`
    To          []string          `json:"to,omitempty"`
    CreatedTime int64             `json:"created_time,omitempty"` // Unix epoch
    ExpiresTime int64             `json:"expires_time,omitempty"`
    ThreadID    string            `json:"thid,omitempty"`
    ParentID    string            `json:"pthid,omitempty"`
    Body        map[string]any    `json:"body"`
    Attachments []Attachment      `json:"attachments,omitempty"`
}

// Content types
const (
    ContentTypePlain     = "application/didcomm-plain+json"
    ContentTypeSigned    = "application/didcomm-signed+json"
    ContentTypeEncrypted = "application/didcomm-encrypted+json"
)
```

### Pattern 3: ATAP Protocol Message Types

Custom protocol URIs under `https://atap.dev/protocols/`. Define as constants:

```go
// ATAP protocol message types (MSG-05)
const (
    // Approval lifecycle
    TypeApprovalRequest  = "https://atap.dev/protocols/approval/1.0/request"
    TypeApprovalResponse = "https://atap.dev/protocols/approval/1.0/response"
    TypeApprovalRevoke   = "https://atap.dev/protocols/approval/1.0/revoke"
    TypeApprovalStatus   = "https://atap.dev/protocols/approval/1.0/status"
    TypeApprovalRejected = "https://atap.dev/protocols/approval/1.0/rejected"

    // System messages
    TypePing         = "https://atap.dev/protocols/basic/1.0/ping"
    TypePong         = "https://atap.dev/protocols/basic/1.0/pong"
    TypeProblemReport = "https://atap.dev/protocols/report-problem/1.0/problem-report"
)
```

The `body` content for each type is defined per-type. Phase 2 only needs to define the routing infrastructure — the actual approval body structure is Phase 3.

### Pattern 4: Message Queue (Offline Delivery)

**Mediator responsibility (MSG-02):** If the recipient is not currently connected, the server queues the raw encrypted JWE bytes (the server cannot decrypt them — end-to-end encrypted). When the recipient reconnects and calls a pickup endpoint, the server delivers queued messages.

**Simplified design for Phase 2:** Since the old SSE/Redis pub/sub is gone, use:
- PostgreSQL `didcomm_messages` table for durable queue (survives restart)
- Redis `PUBLISH` on `inbox:{entity_id}` for live delivery to polling clients
- A pickup endpoint `GET /v1/didcomm/inbox` (paginated, returns JWE bytes)

DIDComm specifies a full "Pickup Protocol 3.0" (`https://didcomm.org/messagepickup/3.0/`) with status-request/delivery-request/message-received DIDComm messages. For Phase 2, a simpler REST-based pickup is acceptable since the full pickup protocol is itself a DIDComm protocol that would be circular.

**Message queue schema:**
```sql
CREATE TABLE didcomm_messages (
    id             TEXT PRIMARY KEY,          -- "msg_" + ULID
    recipient_did  TEXT NOT NULL,
    sender_did     TEXT,                       -- NULL for anoncrypt
    message_type   TEXT,                       -- decrypted type (if server is recipient)
    payload        BYTEA NOT NULL,             -- raw JWE bytes (encrypted, opaque to server)
    state          TEXT NOT NULL DEFAULT 'pending', -- pending | delivered | expired
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ,
    delivered_at   TIMESTAMPTZ
);
CREATE INDEX idx_didcomm_messages_recipient ON didcomm_messages(recipient_did, state);
CREATE INDEX idx_didcomm_messages_expires ON didcomm_messages(expires_at) WHERE state = 'pending';
```

### Pattern 5: POST /v1/didcomm Handler

```
POST /v1/didcomm
Content-Type: application/didcomm-encrypted+json
Body: <JWE JSON or Compact Serialization>

Server logic:
1. Parse JWE — find recipient `kid` in protected header
2. Look up which local entity owns that kid
3. Determine if server is the recipient (MSG-03) or just mediating (MSG-02)
4. If server is mediating: queue JWE for recipient entity, return 202 Accepted
5. If server is recipient (forward message): unwrap, process routing
6. Return 202 (async delivery) or 200 (synchronous reply)
```

The `POST /v1/didcomm` endpoint does NOT require OAuth/DPoP authentication — DIDComm messages are self-authenticating via encryption. The sender identity is proven by ECDH-1PU (using `skid` header). Accept `application/didcomm-encrypted+json` only.

### Anti-Patterns to Avoid

- **Building a general-purpose DIDComm mediator:** Out of scope per REQUIREMENTS.md — "ATAP only mediates for own hosted entities." Only accept messages addressed to DIDs registered on this server.
- **Decrypting messages the server is not a recipient of:** The server queues opaque JWE bytes. It cannot and must not try to decrypt messages destined for other entities.
- **Using go-jose v4 for X25519 ECDH:** go-jose v4 does not support X25519 or ECDH-1PU. Use `crypto/ecdh` (stdlib) for all X25519 operations.
- **Using Redis as sole message store:** Redis is volatile; JWE payloads must persist in PostgreSQL, with Redis used only for live push notification (PUBLISH to `inbox:{entity_id}`).
- **Skipping the tag-in-KDF step for ECDH-1PU:** This is not standard ECDH-ES — the ciphertext tag must be part of the wrapping key derivation. Skipping it produces non-compliant encrypted messages.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| X25519 key generation | Custom Curve25519 math | `crypto/ecdh.X25519().GenerateKey()` | Stdlib; RFC 7748 compliant |
| XChaCha20-Poly1305 AEAD | Raw salsa20 | `golang.org/x/crypto/chacha20poly1305.NewX()` | Vetted; correct 24-byte nonce |
| AES-256-Key-Wrap | Custom key wrap | `go-jose/v4` AES KW utilities or stdlib `crypto/aes` + RFC 3394 | AES-KW has specific padding; go-jose has correct implementation |
| ConcatKDF (NIST SP 800-56C) | SHA loop manually | Use stdlib HMAC-SHA512 + custom KDF following NIST spec (10 lines) | `golang-crypto/concatkdf` is v0.x and low-maintenance; write the KDF inline for reliability |
| ULID generation | UUID or timestamp | `oklog/ulid/v2` (already in go.mod) | Already present; consistent with rest of codebase |
| Multibase encoding for X25519 | Custom encoder | Re-use `EncodePublicKeyMultibase()` (already in `crypto/did.go`) | Same base58btc "z" prefix convention |

**Key insight:** The entire ECDH-1PU JWE construction is ~150 lines of Go using only stdlib and already-present dependencies. The complexity is in getting the algorithm details exactly right (tag-in-KDF, apu/apv encoding, CEK wrapping order), not in finding a library.

---

## Common Pitfalls

### Pitfall 1: Algorithm Combination Mismatch

**What goes wrong:** Using ECDH-1PU (authcrypt) with XC20P content encryption produces non-interoperable messages. XC20P is specified only for anoncrypt (ECDH-ES).

**Why it happens:** The project requirements text says "ECDH-1PU + XC20P" — this is ambiguous. Multiple DIF spec issues confirm XC20P is ECDH-ES only for authcrypt.

**How to avoid:** For authcrypt (MSG-04), use `ECDH-1PU+A256KW` + `A256CBC-HS512`. If the team decides XC20P is desired for content encryption, it must be with ECDH-ES (anoncrypt), not ECDH-1PU.

**Warning signs:** JWE `alg: ECDH-1PU+A256KW` with `enc: XC20P` — this combination is not defined in the ECDH-1PU draft v4.

### Pitfall 2: Missing Tag-in-KDF for ECDH-1PU

**What goes wrong:** CEK wrapping key is derived without including the ciphertext authentication tag. Decryption of well-formed messages from compliant clients fails.

**Why it happens:** ECDH-1PU draft v4 requires the ciphertext tag to be appended to Z before ConcatKDF, but this is not how ECDH-ES works. Easy to miss.

**How to avoid:** Encrypt payload with CEK first → extract tag → include tag in Z = Ze || Zs || tag → then run ConcatKDF for wrapping key.

**Warning signs:** Interop tests with a known-good implementation (Python or Rust `didcomm` library) fail with decryption errors.

### Pitfall 3: DID Document Missing keyAgreement

**What goes wrong:** Senders cannot look up the recipient's X25519 key — they resolve the DID Document and find no `keyAgreement` section. Message encryption fails client-side.

**Why it happens:** The current `BuildDIDDocument()` only includes Ed25519 verification methods. X25519 key agreement is a separate key type.

**How to avoid:** Phase 2 must update `BuildDIDDocument()`, the `DIDDocument` model, and the entity registration flow to generate and persist X25519 keypairs.

**Warning signs:** DID Document `/.well-known` at entity path has no `keyAgreement` field.

### Pitfall 4: Exposing Private Key Material in X25519 Storage

**What goes wrong:** X25519 private key for server-mediated entities stored unencrypted in PostgreSQL. This is acceptable for Phase 2 (per Phase 4 deferred encryption) but must be a documented known risk.

**Why it happens:** Per-entity encryption is a Phase 4 requirement (PRV-01). Phase 2 stores X25519 private key in `entities` table.

**How to avoid:** Note explicitly in code that the X25519 private key column will be encrypted in Phase 4. Use a consistent column name (`x25519_private_key`) that Phase 4 can audit.

### Pitfall 5: Accepting Messages for Foreign DIDs

**What goes wrong:** Server receives a JWE addressed to `did:web:other-server:agent:abc` and attempts to deliver it. This makes the server a general-purpose router.

**Why it happens:** DIDComm mediators by design route messages; easy to forget the ATAP scope constraint.

**How to avoid:** In the POST /v1/didcomm handler, after extracting the recipient kid, check that the DID's domain matches `PlatformDomain`. Return 400 if not.

### Pitfall 6: go-jose v4 X25519 Assumption

**What goes wrong:** Developer assumes go-jose v4 supports X25519 key agreement (it looks like a JOSE library and JOSE covers ECDH). It does not — only P-256/P-384/P-521.

**Why it happens:** go-jose v4 handles ECDH-ES over NIST curves but not Curve25519. This is a known limitation.

**How to avoid:** Use `crypto/ecdh.X25519()` for all X25519 operations. Only use go-jose for A256CBC-HS512 content encryption or AES-KW if needed.

---

## Code Examples

Verified patterns from official sources:

### X25519 Key Generation (Go stdlib)

```go
// Source: https://pkg.go.dev/crypto/ecdh
import "crypto/ecdh"

curve := ecdh.X25519()
privateKey, err := curve.GenerateKey(rand.Reader)
publicKey := privateKey.PublicKey()
// publicKey.Bytes() = raw 32-byte X25519 public key
```

### X25519 ECDH Scalar Multiply

```go
// Source: https://pkg.go.dev/crypto/ecdh
sharedSecret, err := myPrivKey.ECDH(theirPubKey)
// sharedSecret = 32-byte shared secret
```

### XChaCha20-Poly1305 (XC20P) — for anoncrypt content encryption

```go
// Source: https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305
import "golang.org/x/crypto/chacha20poly1305"

// KeySize = 32, NonceSizeX = 24, Overhead = 16
aead, err := chacha20poly1305.NewX(cek32bytes)
nonce := make([]byte, chacha20poly1305.NonceSizeX)
rand.Read(nonce)
ciphertext := aead.Seal(nil, nonce, plaintext, additionalData)
```

### ConcatKDF for ECDH-1PU (A256KW wrapping key)

```go
// Source: NIST SP 800-56C, IETF draft-madden-jose-ecdh-1pu-04
// Z = Ze || Zs  (or Ze || Zs || tag for v4)
// KDF input: 4-byte big-endian counter || Z || otherInfo
// otherInfo: algID || apu || apv || keyDataLen
func concatKDF(z []byte, alg, apu, apv string, keyLenBits int) []byte {
    h := sha512.New
    // keyLenBits = 256 for A256KW
    // SHA-512 for A256CBC-HS512; SHA-256 for A128KW
    // Full implementation: ~25 lines; use standard NIST construction
}
```

### DIDComm JWE Envelope Structure

```json
{
  "protected": "<base64url({alg,enc,epk,apu,apv,skid})>",
  "recipients": [{
    "header": { "kid": "<recipient_key_id>" },
    "encrypted_key": "<base64url(wrapped_cek)>"
  }],
  "iv": "<base64url(nonce)>",
  "ciphertext": "<base64url(encrypted_payload)>",
  "tag": "<base64url(auth_tag)>"
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Old ATAP SSE/Redis pub/sub delivery | DIDComm v2.1 JWE envelope | Phase 1 cleanup | No SSE code remains; must build new delivery layer |
| Custom Ed25519 signed requests | DIDComm authcrypt (ECDH-1PU) | Phase 2 | Sender auth is now cryptographic via key agreement |
| Bearer token auth | DPoP-bound OAuth 2.1 | Phase 1 | POST /v1/didcomm should NOT require OAuth — DIDComm is self-authenticating |
| `agent://` URIs | `did:web` DIDs | Phase 1 | Entity identity model is complete |

**Deprecated/outdated:**
- Redis pub/sub for SSE: Removed in Phase 1 cleanup. Redis is retained but pub/sub is gone. Phase 2 can re-use Redis with `PUBLISH` for live inbox notification.
- `Aries Framework Go`: Archived/unmaintained. Do not use.
- DIDComm v1 (using `~transport` decorators, Aries-style): Different protocol. ATAP targets v2.1 only.

---

## Open Questions

1. **ECDH-1PU + XC20P: What does the requirement actually mean?**
   - What we know: DIDComm v2.1 spec prohibits ECDH-1PU with XC20P; authcrypt requires A256CBC-HS512; XC20P is anoncrypt only
   - What's unclear: Was the requirement written based on an older Aries RFC envelope (`ECDH-1PU+XC20PKW`) which combined them differently?
   - Recommendation: Plan for `ECDH-1PU+A256KW` + `A256CBC-HS512` as the authcrypt standard per spec. If XC20P is specifically desired for content encryption, use anoncrypt (ECDH-ES) instead. The planner should treat MSG-04 as "ECDH-1PU+A256KW / A256CBC-HS512".

2. **X25519 key persistence: per-entity column vs key_versions table**
   - What we know: Entities currently have one Ed25519 key versioned in `key_versions`. X25519 is a different key type for a different purpose (key agreement, not signing).
   - What's unclear: Should X25519 keys be rotatable in Phase 2 or is one-per-entity sufficient?
   - Recommendation: Add `x25519_public_key BYTEA` and `x25519_private_key BYTEA` columns to `entities` table in migration 010. Rotation deferred to Phase 4.

3. **POST /v1/didcomm authentication: DPoP required or not?**
   - What we know: DIDComm messages are self-authenticating via ECDH-1PU. Requiring DPoP would prevent messages from entities on other servers (federation, Phase 4 concern).
   - Recommendation: Do NOT require DPoP auth on POST /v1/didcomm. Rate-limit by IP instead. Validate that the recipient DID is registered on this server.

4. **Server's own X25519 key for MSG-03 (server as `via`)**
   - What we know: The server needs its own DID + X25519 keypair to act as a trusted participant (co-signer). The server's platform key (Ed25519) already exists in config.
   - Recommendation: Add server X25519 keypair to `Config` struct, generated at startup if absent (stored in DB or env var). Server's DID is `did:web:{domain}:server:platform`.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`testing` package) + `testify` v1.11 |
| Config file | None — standard `go test ./...` |
| Quick run command | `cd /Users/svenloth/dev/atap/platform && go test ./internal/didcomm/... -run TestEnvelope -v` |
| Full suite command | `cd /Users/svenloth/dev/atap/platform && go test ./... -timeout 120s` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MSG-01 | DIDComm plaintext message serializes to correct JSON structure with required fields | unit | `go test ./internal/didcomm/... -run TestPlaintextMessage` | ❌ Wave 0 |
| MSG-02 | Server queues JWE for offline entity; entity retrieves it from pickup endpoint | integration | `go test ./... -run TestMediatorQueue` | ❌ Wave 0 |
| MSG-03 | Server's own DID document has valid X25519 keyAgreement and DIDCommMessaging service | unit | `go test ./internal/api/... -run TestServerDIDDocument` | ❌ Wave 0 |
| MSG-04 | ECDH-1PU+A256KW envelope encrypts and decrypts round-trip | unit | `go test ./internal/didcomm/... -run TestEnvelopeRoundTrip` | ❌ Wave 0 |
| MSG-05 | ATAP protocol type constants compile; message router dispatches by type | unit | `go test ./internal/didcomm/... -run TestMessageTypes` | ❌ Wave 0 |
| API-05 | POST /v1/didcomm with valid JWE returns 202; invalid content type returns 415 | unit | `go test ./internal/api/... -run TestDIDCommEndpoint` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `cd /Users/svenloth/dev/atap/platform && go test ./internal/didcomm/... ./internal/api/... -timeout 30s`
- **Per wave merge:** `cd /Users/svenloth/dev/atap/platform && go test ./... -timeout 120s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `platform/internal/didcomm/envelope_test.go` — covers MSG-04 (JWE round-trip with test vectors)
- [ ] `platform/internal/didcomm/message_test.go` — covers MSG-01, MSG-05 (plaintext structure + type routing)
- [ ] `platform/internal/api/didcomm_test.go` — covers API-05 (POST endpoint behaviour)
- [ ] `platform/internal/store/messages_test.go` — covers MSG-02 (queue store operations)
- [ ] No new framework install required — `go test ./...` works with existing setup

---

## Sources

### Primary (HIGH confidence)

- [DIDComm Messaging Specification v2.1](https://identity.foundation/didcomm-messaging/spec/v2.1/) — message structure, encryption algorithms, keyAgreement, service endpoints, mediator forward protocol
- [Go crypto/ecdh stdlib](https://pkg.go.dev/crypto/ecdh) — X25519 key generation and ECDH
- [golang.org/x/crypto/chacha20poly1305](https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305) — XC20P constants, NewX()
- [go-jose/go-jose v4 README](https://github.com/go-jose/go-jose) — confirmed no X25519 or ECDH-1PU support

### Secondary (MEDIUM confidence)

- [DIF Blog: ECDH-1PU being implemented](https://blog.identity.foundation/ecdh-1pu-implementation/) — algorithm pairing clarification (verified against DIF spec)
- [DIDComm Message Pickup 3.0](https://didcomm.org/messagepickup/3.0/) — offline message delivery protocol
- [DIF Issue #213: XC20P only for anoncrypt](https://github.com/decentralized-identity/didcomm-messaging/issues/213) — confirms XC20P is ECDH-ES only

### Tertiary (LOW confidence — needs validation)

- [github.com/golang-crypto/concatkdf v0.1.1](https://pkg.go.dev/github.com/golang-crypto/concatkdf) — ConcatKDF helper; v0.x, low-maintenance; implement inline instead
- Aries RFC 0334 JWE Envelope — older Aries DIDComm v1 patterns; do NOT use directly but useful for algorithm reference

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all core dependencies verified via pkg.go.dev and existing go.mod
- Architecture: MEDIUM — pattern is sound but DIDComm-in-Go has no reference implementation; tag-in-KDF requirement needs test-vector verification
- Pitfalls: HIGH — algorithm mismatch confirmed by DIF spec issues; go-jose X25519 gap confirmed by library README

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (DIDComm v2.1 is approved spec; Go stdlib stable)
