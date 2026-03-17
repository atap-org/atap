# Phase 4: Credentials and Mobile - Research

**Researched:** 2026-03-14
**Domain:** W3C Verifiable Credentials 2.0, SD-JWT, Bitstring Status List, per-entity encryption, crypto-shredding, Flutter biometric signing, org delegate routing
**Confidence:** HIGH (spec is authoritative; libraries verified against pkg.go.dev and pub.dev)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CRD-01 | All verified properties as W3C VCs 2.0 (VC-JOSE-COSE format) | trustbloc/vc-go v1.3.6 handles VC creation/signing/parsing with JWT proofs; JWS signed with server Ed25519 key |
| CRD-02 | ATAP credential types: EmailVerification, PhoneVerification, Personhood, Identity, Principal, OrgMembership | Defined as `type` array values on VC `credentialSubject`; server issues via email/phone OTP flow, World ID for personhood |
| CRD-03 | Trust level derivation from credentials: L0 (none), L1 (email/phone), L2 (personhood), L3 (identity) | Computed from presence of VC types in entity's credential set; `entity.trust_level` column already exists |
| CRD-04 | Effective trust = `min(entity_trust_level, server_trust)` | Computed at query time; server trust stored in config |
| CRD-05 | Credential revocation via W3C Bitstring Status List v1.0 | Hand-rolled in Go (no standalone Go library found); compress/gzip + base64url + JSONB status list VC endpoint |
| CRD-06 | SD-JWT (RFC 9901) for selective disclosure on credentials containing personal information | `github.com/MichaelFraser99/go-sd-jwt` v1.4.0 (Aug 2025); or trustbloc/vc-go built-in SD-JWT support |
| PRV-01 | All VC content containing PII encrypted at rest with per-entity encryption key | AES-256-GCM from Go stdlib `crypto/aes`; per-entity 32-byte key stored in `entity_enc_keys` table |
| PRV-02 | Crypto-shredding: delete per-entity key → all credential data unrecoverable | DELETE from `entity_enc_keys`; VC ciphertext rows remain but are unreadable |
| PRV-03 | Upon crypto-shredding: deactivate DID Document, notify federation partners | Add `deactivated: true` to DID Doc; queue DIDComm `entity/1.0/shredded` message |
| PRV-04 | Personhood credentials MUST NOT contain or transmit raw biometric data | Server-side constraint: only accept a ZK proof token or provider assertion, never store biometric bytes |
| MOB-01 | Generate keypair in secure enclave, create did:web DID, set recovery passphrase | `biometric_signature` 10.2.0 creates hardware-backed ECDSA key; Ed25519 key via `flutter_secure_storage` + `ed25519_edwards` |
| MOB-02 | DIDComm message inbox feed | Rebuild existing InboxScreen to poll `GET /v1/messages` (DIDComm message queue endpoint) |
| MOB-03 | Approval rendering: fetch + verify template, render branded/fallback card, approve/decline biometric | Flutter HTTP fetch template URL; verify JWS proof against `via` DID; render Card widget per template fields |
| MOB-04 | Credential management: view, present, revoke VCs | New CredentialsScreen; calls `GET /v1/credentials`, `DELETE /v1/credentials/{id}` |
| MOB-05 | Persistent approval management: list, revoke | New ApprovalsScreen; calls `GET /v1/approvals?type=persistent`, `DELETE /v1/approvals/{id}` |
| MOB-06 | Biometric prompt → JWS signature from secure enclave → send approval response via DIDComm | `biometric_signature.createSignature()` for ECDSA; or `local_auth` + `ed25519_edwards` sign from `flutter_secure_storage` key |
| API-04 | Credential endpoints: email/phone verification flows, personhood submission, list credentials, status list | 6 new Fiber routes under `/v1/credentials/`; email OTP via smtp, phone OTP via Twilio or stub |
| MSG-06 | Org delegate routing: fan-out capped at 50, per-source rate limiting, first-response-wins | Go goroutine fan-out with `sync.Once` for first-response-wins; SELECT org delegates limited to 50 |
</phase_requirements>

---

## Summary

Phase 4 completes the ATAP v1.0 milestone. It adds three orthogonal capabilities: the trust credential system (W3C VCs with email/phone/personhood verification, SD-JWT selective disclosure, Bitstring Status List revocation), privacy enforcement (per-entity AES-256-GCM encryption + crypto-shredding on DELETE /v1/entities), and the mobile approval client (Flutter app rebuilt around DIDComm inbox, biometric-signed approval responses, and credential management UI).

The backend work is the larger share. The VC issuance pipeline requires new database tables (`credentials`, `entity_enc_keys`, `credential_status_lists`), new API routes, and a new migration. The crypto-shredding requires extending the existing `DeleteEntity` store method to first delete the encryption key before removing entity rows. The trustbloc/vc-go library (v1.3.6, Apache 2.0) handles VC creation and JWT signing with the server's existing Ed25519 key, avoiding any custom VC serialization.

The mobile app is largely a rebuild of existing screens. The existing `InboxScreen` references old `Signal` models and SSE; it must be replaced with DIDComm message polling, an approval card renderer, and new credential/approval management screens. Biometric signing uses `biometric_signature` (ECDSA P-256 from secure enclave) — the ATAP spec requires Ed25519 but does not prohibit the device signing key from being a different algorithm than the server-registered key; however, the approval response JWS `kid` must reference the mobile entity's DID key. The safest interpretation is to keep Ed25519 (generated by `ed25519_edwards`, stored in `flutter_secure_storage` protected by biometric via `local_auth`) and use `biometric_signature` only as the unlock gate.

**Primary recommendation:** Use trustbloc/vc-go for VC issuance on the backend, hand-roll Bitstring Status List (compress/gzip + base64url, ~60 lines), use Go stdlib AES-256-GCM for per-entity encryption, and in Flutter gate Ed25519 signing behind `local_auth` biometric prompt rather than replacing Ed25519 with ECDSA.

---

## Standard Stack

### Core (Backend — Go)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/trustbloc/vc-go/verifiable` | v1.3.6 (Jan 2026) | W3C VC 2.0 create/sign/parse/verify | Active Apache 2.0, supports JWT/JWS/SD-JWT proofs with Ed25519, updated Jan 2026 |
| `github.com/MichaelFraser99/go-sd-jwt` | v1.4.0 (Aug 2025) | SD-JWT RFC 9901 for selective disclosure | Only standalone Go SD-JWT RFC 9901 library found; MIT license; OR use trustbloc built-in SD-JWT support |
| `crypto/aes` + `crypto/cipher` (stdlib) | Go 1.22 stdlib | AES-256-GCM per-entity encryption | Standard library, no dep; authenticated encryption |
| `compress/gzip` (stdlib) | Go 1.22 stdlib | GZIP compress bitstring for status list | Standard library; Bitstring Status List spec requires GZIP |
| `encoding/base64` (stdlib) | Go 1.22 stdlib | Base64url encode compressed bitstring | Standard library |

### Core (Mobile — Flutter/Dart)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `local_auth` | ^2.3.0 | Biometric gate (FaceID/fingerprint) before signing | Flutter official plugin; does NOT sign — just authenticates |
| `flutter_secure_storage` | ^10.0.0 | Store Ed25519 private key hardware-protected | Already in pubspec; AES+Keychain/Keystore backed |
| `ed25519_edwards` | ^0.3.1 | Ed25519 signing for approval JWS responses | Already in pubspec; pure Dart, cross-platform |
| `http` | ^1.6.0 | API calls to credential endpoints | Already in pubspec |

### Supporting (Backend)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/rand` (stdlib) | stdlib | Generate per-entity 32-byte AES keys | Every new entity creation (for CRD types that encrypt) |
| `golang.org/x/crypto/argon2` | stdlib extension | Argon2id for recovery passphrase backup | Key recovery export, spec §5.6 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| trustbloc/vc-go | Hand-roll VC JWT serialization | VC spec has many edge cases (context, types, proof format); use library |
| `local_auth` + `ed25519_edwards` | `biometric_signature` package | `biometric_signature` uses ECDSA P-256, not Ed25519; signing key algorithm mismatch with server-registered Ed25519 key; use `local_auth` as gate only |
| Hand-rolled Bitstring Status List | trustbloc/vc-go status list support | trustbloc/vc-go has StatusList2021 validator, but publishing the list endpoint is server-specific; hand-rolling is ~60 lines and straightforward |

**Installation (backend):**
```bash
cd platform && go get github.com/trustbloc/vc-go/verifiable@v1.3.6
cd platform && go get github.com/MichaelFraser99/go-sd-jwt@v1.4.0
```

**Installation (mobile):**
```bash
cd mobile && flutter pub add local_auth
```

---

## Architecture Patterns

### Recommended Project Structure (Backend Additions)

```
platform/
  internal/
    credentials/         # VC issuance, trust level, status list logic
      credentials.go     # Issue, list, revoke VCs
      trust.go           # TrustLevel derivation from VC set
      statuslist.go      # Bitstring Status List encode/decode
    api/
      credentials.go     # HTTP handlers for /v1/credentials/* routes
    store/
      credentials.go     # DB layer: credentials, entity_enc_keys, status_lists
      org_delegates.go   # Delegate fan-out query
    models/
      credentials.go     # Credential, EncryptionKey, StatusList types (add to models.go)
  migrations/
    012_credentials.up.sql   # credentials, entity_enc_keys, credential_status_lists tables
```

```
mobile/lib/
  features/
    inbox/                   # Replace Signal with DIDCommMessage model
      inbox_screen.dart      # Rebuild: poll /v1/messages, render approval cards
      approval_card.dart     # New: branded + fallback approval card renderer
    credentials/             # New feature
      credentials_screen.dart
    approvals/               # New feature
      approvals_screen.dart
  core/
    models/
      didcomm_message.dart   # New: DIDComm message model
      credential.dart        # New: VC model
      approval.dart          # New: Approval model
    crypto/
      jws_service.dart       # New: Build JWS for approval response
```

### Pattern 1: Per-Entity Encryption (PRV-01, PRV-02)

**What:** Each entity gets a random 32-byte AES-256-GCM key stored in `entity_enc_keys`. All VC credential content (JSONB) is encrypted before insert, decrypted on read. Crypto-shredding = DELETE the key row.

**When to use:** Every credential INSERT and SELECT for human entities.

**Example:**
```go
// Source: Go stdlib crypto/aes, crypto/cipher
func encryptCredential(key []byte, plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("aes new cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("new gcm: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, fmt.Errorf("generate nonce: %w", err)
    }
    // Nonce prepended to ciphertext; GCM Seal appends auth tag
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decryptCredential(key []byte, ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("aes new cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("new gcm: %w", err)
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

### Pattern 2: VC Issuance with trustbloc/vc-go

**What:** Create a W3C VC 2.0, sign it as a JWT using the server's Ed25519 key, store the JWT as the `credential_jwt` column. The `credentialStatus` field points to the server's Bitstring Status List endpoint.

**When to use:** Whenever issuing any credential (email verified, phone, personhood).

**Example:**
```go
// Source: github.com/trustbloc/vc-go/verifiable v1.3.6
import "github.com/trustbloc/vc-go/verifiable"

func issueEmailVC(entityDID, email, issuerDID, serverKeyID string, signer jwt.ProofCreator) (string, error) {
    vc, err := verifiable.CreateCredential(verifiable.CredentialContents{
        Context: []string{
            "https://www.w3.org/ns/credentials/v2",
            "https://atap.dev/ns/v1",
        },
        Types:    []string{"VerifiableCredential", "ATAPEmailVerification"},
        Issuer:   &verifiable.Issuer{ID: issuerDID},
        Issued:   utiltime.NewTime(time.Now()),
        Subject: []verifiable.Subject{{
            ID:           entityDID,
            CustomFields: verifiable.CustomFields{"email": email},
        }},
    }, verifiable.CustomFields{
        "credentialStatus": map[string]any{
            "id":                   "https://api.atap.app/v1/credentials/status/1#42",
            "type":                 "BitstringStatusListEntry",
            "statusPurpose":        "revocation",
            "statusListIndex":      "42",
            "statusListCredential": "https://api.atap.app/v1/credentials/status/1",
        },
    })
    if err != nil {
        return "", fmt.Errorf("create credential: %w", err)
    }
    jwtClaims, err := vc.JWTClaims(false)
    if err != nil {
        return "", fmt.Errorf("jwt claims: %w", err)
    }
    return jwtClaims.MarshalJWSString(verifiable.EdDSA, signer, serverKeyID)
}
```

### Pattern 3: Bitstring Status List (CRD-05)

**What:** Maintain a JSONB column `bits` ([]byte) per status list. A single list supports up to 16 KB * 8 = 131,072 entries. Each credential is assigned a sequential index. The `/v1/credentials/status/{list-id}` endpoint returns a VC whose `credentialSubject.encodedList` is the GZIP-compressed, base64url-encoded bitstring.

**When to use:** Credential revocation. Revoking a credential = SET bit at that index to 1.

**Example:**
```go
// Source: compress/gzip, encoding/base64 (Go stdlib)
func encodeStatusList(bits []byte) (string, error) {
    var buf bytes.Buffer
    w := gzip.NewWriter(&buf)
    if _, err := w.Write(bits); err != nil {
        return "", fmt.Errorf("gzip write: %w", err)
    }
    if err := w.Close(); err != nil {
        return "", fmt.Errorf("gzip close: %w", err)
    }
    return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func setBit(bits []byte, index int) {
    bits[index/8] |= (1 << (7 - (index % 8)))
}
```

### Pattern 4: Org Delegate Fan-Out (MSG-06)

**What:** When an approval targets an org DID, query entities whose `principal_did` matches the org DID (up to 50). Dispatch the DIDComm message concurrently. Use `sync.Once` to honor first-response-wins: the first valid response transitions the approval, subsequent responses are discarded.

**When to use:** Any approval whose `to` field is an org entity DID.

**Example:**
```go
// Standard Go concurrency; no external dependency
func (h *Handler) fanOutToOrgDelegates(ctx context.Context, orgDID string, msg *didcomm.PlaintextMessage) error {
    delegates, err := h.entityStore.GetOrgDelegates(ctx, orgDID, 50)
    if err != nil {
        return fmt.Errorf("get delegates: %w", err)
    }
    for _, delegate := range delegates {
        msgCopy := *msg
        msgCopy.To = []string{delegate.DID}
        h.dispatchDIDCommMessage(&msgCopy)
    }
    return nil
}
// First-response-wins enforced via UPDATE approvals SET state='approved' WHERE id=$1 AND state='requested'
// returning 0 rows = already handled by another delegate response
```

### Pattern 5: Flutter Biometric-Gated Ed25519 Signing (MOB-06)

**What:** Use `local_auth` to confirm biometric, then use the Ed25519 private key from `flutter_secure_storage` to sign the JWS approval response. The biometric does not generate the signature — it gates access to the key.

**When to use:** Every approval response (approve/decline).

**Example:**
```dart
// local_auth gates, ed25519_edwards signs
Future<String> signApprovalResponse(String payload) async {
    final localAuth = LocalAuthentication();
    final authenticated = await localAuth.authenticate(
        localizedReason: 'Confirm approval with biometrics',
        options: const AuthenticationOptions(biometricOnly: true),
    );
    if (!authenticated) throw Exception('Biometric authentication failed');

    final keyId = await _storage.getKeyId();
    final privateKey = await _storage.getPrivateKey(keyId!);
    // Build JWS: header.payload.signature (detached payload per RFC 7797)
    return JwsService.signDetached(privateKey!, payload);
}
```

### Anti-Patterns to Avoid

- **Storing encryption keys in the same table as encrypted data:** The `entity_enc_keys` table must be a separate table so a SELECT on credentials does not reveal the key in the same row.
- **Using trustbloc/vc-go's JSON-LD document loader with network calls in tests:** Pass `verifiable.WithDisabledProofCheck()` in tests or use the in-memory loader; the default loader makes network calls to `https://www.w3.org/`.
- **Issuing email VCs before email is verified:** The server must complete the OTP round-trip before calling `issueEmailVC`. Do not issue on `POST /start`.
- **Biometric returning `bool` as proof of signing:** `local_auth.authenticate()` returns only authentication success; the actual signature must be generated from the stored key. The bool is merely a gate.
- **Not deactivating DID Document on crypto-shred:** The spec requires both: (1) deleting the key and (2) marking the DID Doc deactivated. Partial deletion leaves a resolvable DID that points to inaccessible credentials — inconsistent state.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| W3C VC 2.0 JWT serialization and proof verification | Custom VC struct marshaling | `trustbloc/vc-go/verifiable` | VC-JOSE-COSE has many edge cases: context normalization, JWT claim mapping, proof purpose validation |
| SD-JWT issuing and disclosure verification | Custom `_sd` claim handling | `go-sd-jwt` or trustbloc built-in | Salt generation, claim hashing, disclosure serialization are subtle; RFC 9901 compliance required |
| AES-256-GCM encryption | Custom XOR or custom cipher | Go stdlib `crypto/aes` + `crypto/cipher` | Authenticated encryption (prevents tampering); stdlib is FIPS-validated |
| Biometric unlock on Flutter | Native platform code via method channels | `local_auth` plugin | Platform-specific code for FaceID, TouchID, and Android biometric API is complex; plugin handles all variants |

**Key insight:** The VC issuance pipeline has many subtle spec requirements (context ordering, JWT claim mapping, proof format). Attempting to hand-roll even the JWT VC encoding against the spec will produce non-interoperable output. The trustbloc library has been tested against the W3C VC test suite.

---

## Common Pitfalls

### Pitfall 1: trustbloc/vc-go JSON-LD Document Loader

**What goes wrong:** `verifiable.ParseCredential()` or `verifiable.CreateCredential()` makes outbound HTTPS requests to `https://www.w3.org/ns/credentials/v2` to resolve JSON-LD contexts. This will fail in CI, slow down tests, and create a hard dependency on W3C infrastructure.

**Why it happens:** JSON-LD validation requires context loading; the default loader fetches from the network.

**How to avoid:** Use `verifiable.WithDisabledProofCheck()` in tests, or provide an in-memory document loader with pre-loaded context bytes. For production, cache contexts in the binary.

**Warning signs:** Tests pass locally but fail in offline/CI environments with DNS errors.

---

### Pitfall 2: Crypto-Shred Order of Operations

**What goes wrong:** If `DeleteEntity` deletes the entity row before deleting the encryption key row, there is a brief window where credential ciphertext exists without a corresponding key. If the entity row delete fails partway (rare), you have orphaned encrypted data with an orphaned key.

**Why it happens:** No database transaction wraps the two deletes.

**How to avoid:** Wrap the entire crypto-shred operation in a single pgx transaction: (1) DELETE `entity_enc_keys` WHERE entity_id=$1, (2) UPDATE DID Document to deactivated, (3) DELETE entity row, (4) queue `entity/1.0/shredded` DIDComm message.

**Warning signs:** Credential rows exist in DB but `GetEntityEncKey` returns nil (key already gone) — the entity still exists.

---

### Pitfall 3: Flutter inbox_screen.dart References Deleted Signal Model

**What goes wrong:** The existing `InboxScreen` imports `Signal` from `core/models/signal.dart` and `InboxNotifier` that calls the old SSE endpoint. Replacing the model without updating all widget references causes a cascade of compile errors.

**Why it happens:** The mobile app was built for the old signal pipeline (Phase 0 prototype). Phase 4 is a full replacement, not an extension.

**How to avoid:** The plan must treat the mobile app screens as a rewrite. Create new models (`DIDCommMessage`, `Approval`, `Credential`) before touching existing screens. Delete old models only after all references are replaced.

**Warning signs:** `flutter analyze` shows `Signal` undefined errors.

---

### Pitfall 4: Email OTP Race Condition

**What goes wrong:** The server generates an OTP, stores it in Redis with a 10-minute TTL, and sends it by email. If the `POST /v1/credentials/email/start` handler returns success before the Redis write commits, a subsequent verify call finds no OTP.

**Why it happens:** Fire-and-forget pattern on Redis write.

**How to avoid:** Await the Redis SET before returning 200. Use a short TTL (10 min) and a 6-digit random OTP. Rate-limit `start` to 3 calls per hour per entity.

**Warning signs:** `/email/verify` returns 404 "OTP not found" immediately after `/email/start` 200.

---

### Pitfall 5: biometric_signature Uses ECDSA P-256, Not Ed25519

**What goes wrong:** `biometric_signature.createSignature()` uses ECDSA P-256 keys (or RSA) generated by the secure enclave. ATAP requires the JWS `kid` to reference the entity's DID key, which is Ed25519. Using P-256 from `biometric_signature` produces signatures the server cannot verify against the registered Ed25519 key.

**Why it happens:** iOS Secure Enclave only supports ECDSA P-256; it cannot generate Ed25519 keys. The `biometric_signature` package wraps this hardware capability.

**How to avoid:** Use `biometric_signature` only as a UI-level biometric *gate*. Keep the actual signing key as Ed25519 in `flutter_secure_storage` (encrypted at rest by the OS keystore, not in the secure enclave). Use `local_auth` for biometric authentication before unlocking the key. This is slightly less secure than secure-enclave-bound keys but produces spec-compliant JWS.

**Warning signs:** Server verification returns "signature does not match registered public key."

---

### Pitfall 6: First-Response-Wins Race in Org Delegate Fan-Out

**What goes wrong:** Two delegates respond simultaneously. Both responses are processed, both attempt `UPDATE approvals SET state='approved'`. The approval records the second response, overwriting the first.

**Why it happens:** Two goroutines concurrently calling `RespondToApproval`.

**How to avoid:** The `RespondToApproval` store method must use an atomic conditional update: `UPDATE approvals SET state=$1, responded_at=NOW() WHERE id=$2 AND state='requested'`. If `RowsAffected == 0`, the approval was already handled — discard the response silently. This is already the pattern used for one-time approval consumption; apply the same approach here.

**Warning signs:** Approvals end up in `approved` state twice in the same transaction log.

---

## Code Examples

### Bitstring Status List Endpoint Response

```go
// Source: W3C Bitstring Status List v1.0 §5
// GET /v1/credentials/status/{list-id}
// Returns a Verifiable Credential wrapping the encoded bitstring
type StatusListVC struct {
    Context           []string      `json:"@context"`
    ID                string        `json:"id"`
    Type              []string      `json:"type"`
    Issuer            string        `json:"issuer"`
    ValidFrom         time.Time     `json:"validFrom"`
    CredentialSubject StatusListCS  `json:"credentialSubject"`
}

type StatusListCS struct {
    ID          string `json:"id"`
    Type        string `json:"type"`   // "BitstringStatusList"
    StatusPurpose string `json:"statusPurpose"` // "revocation"
    EncodedList string `json:"encodedList"` // base64url(gzip(bits))
}
```

### Migration 012: Credentials Schema

```sql
-- 012_credentials.up.sql
-- Per-entity encryption keys for crypto-shredding (PRV-01, PRV-02)
CREATE TABLE entity_enc_keys (
    entity_id  TEXT PRIMARY KEY REFERENCES entities(id) ON DELETE CASCADE,
    key_bytes  BYTEA NOT NULL,  -- 32-byte AES-256 key
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Verifiable Credentials (CRD-01 through CRD-06)
CREATE TABLE credentials (
    id            TEXT PRIMARY KEY,            -- "crd_" + ULID
    entity_id     TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    type          TEXT NOT NULL,               -- ATAPEmailVerification etc.
    status_index  INT NOT NULL,               -- index in status list
    status_list_id TEXT NOT NULL,
    credential_ct BYTEA NOT NULL,             -- AES-256-GCM encrypted VC JWT
    issued_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
);

CREATE INDEX idx_credentials_entity ON credentials(entity_id);

-- Bitstring Status Lists (CRD-05)
CREATE TABLE credential_status_lists (
    id         TEXT PRIMARY KEY,              -- "csl_" + sequential int
    bits       BYTEA NOT NULL,               -- raw bitstring (16 KB = 131072 slots)
    next_index INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed one status list
INSERT INTO credential_status_lists (id, bits) VALUES ('1', repeat(chr(0), 16384)::bytea);
```

### Flutter DIDComm Message Model

```dart
// core/models/didcomm_message.dart
class DIDCommMessage {
    final String id;
    final String messageType;
    final String senderDID;
    final Map<String, dynamic> body;
    final DateTime createdAt;

    bool get isApprovalRequest =>
        messageType == 'https://atap.dev/protocols/approval/1.0/request' ||
        messageType == 'https://atap.dev/protocols/approval/1.0/cosigned';

    factory DIDCommMessage.fromJson(Map<String, dynamic> json) { ... }
}
```

### Org Delegate Query

```sql
-- store/org_delegates.go: GetOrgDelegates
SELECT id, did FROM entities
WHERE principal_did = $1
  AND type IN ('human', 'agent', 'machine')
LIMIT 50;
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| trustbloc/vc-go VC Data Model 1.0 | VC Data Model 2.0 (W3C Recommendation May 2025) | May 2025 | `validFrom` replaces `issuanceDate`; `@context` URL is `https://www.w3.org/ns/credentials/v2` |
| StatusList2021 | Bitstring Status List v1.0 (W3C Recommendation) | May 2025 | Same mechanism; renamed; `type` is now `BitstringStatusListEntry` |
| SD-JWT draft | RFC 9901 (IETF Standard, Nov 2025) | Nov 2025 | `go-sd-jwt` v1.4.0 targets this RFC |
| Firebase/SSE inbox | DIDComm message queue polling | Phase 1 (this project) | Mobile app must use `GET /v1/messages` not SSE |

**Deprecated/outdated in this codebase:**
- `mobile/lib/core/models/signal.dart` — Signal model; replace with `DIDCommMessage` + `Approval`
- `mobile/lib/providers/inbox_provider.dart` — SSE-based InboxNotifier; rebuild around DIDComm message polling
- `mobile/lib/providers/push_provider.dart` — Firebase push; already out of scope (decision [01-01])
- `mobile/lib/core/api/sse_client.dart` — SSE client; not used in v1.0 architecture
- `mobile/lib/features/inbox/signal_detail_screen.dart` — Signal detail; replace with approval card

---

## Open Questions

1. **Personhood verification provider**
   - What we know: spec says ZK proof of personhood; PRV-04 forbids raw biometrics
   - What's unclear: no external provider integration is required for v1.0; the spec allows a server-issued "ATAPPersonhood" VC after a manual review or stub
   - Recommendation: Implement as a manual admin-issued VC for v1.0 (POST /v1/credentials/personhood accepts a provider assertion token, server issues VC). World ID integration is v2.

2. **trustbloc/vc-go JSON-LD context loading in production**
   - What we know: the library defaults to network-loading JSON-LD contexts
   - What's unclear: whether ATAP's custom `https://atap.dev/ns/v1` context needs to be served during issuance
   - Recommendation: Use `verifiable.WithDisabledProofCheck()` where contexts are not needed; document the custom ATAP context in `static/` and serve it from the platform for completeness.

3. **Recovery passphrase for mobile key backup (spec §5.6)**
   - What we know: spec requires Argon2id (≥256 MiB, ≥3 iterations), ≥77 bits entropy (≥6 Diceware words)
   - What's unclear: whether Phase 4 must implement full recovery UX or can defer passphrase backup to a later screen
   - Recommendation: MOB-01 must generate the keypair and create the DID; passphrase-encrypted backup export can be a separate UI step shown after onboarding (not blocking for phase completion).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go test (`go test ./...`) |
| Config file | none — table-driven tests in `*_test.go` files next to source |
| Quick run command | `cd platform && go test ./internal/credentials/... -v -run .` |
| Full suite command | `cd platform && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CRD-01 | Issue email VC as JWT, parse it back | unit | `go test ./internal/credentials/... -run TestIssueEmailVC` | Wave 0 |
| CRD-02 | All 6 credential types have correct `@context` and `type` fields | unit | `go test ./internal/credentials/... -run TestCredentialTypes` | Wave 0 |
| CRD-03 | Trust level derivation from credential set | unit | `go test ./internal/credentials/... -run TestTrustLevel` | Wave 0 |
| CRD-04 | Effective trust = min(entity, server) | unit | `go test ./internal/credentials/... -run TestEffectiveTrust` | Wave 0 |
| CRD-05 | Bitstring status list encode/revoke/verify | unit | `go test ./internal/credentials/... -run TestStatusList` | Wave 0 |
| CRD-06 | SD-JWT issue and selective disclosure | unit | `go test ./internal/credentials/... -run TestSDJWT` | Wave 0 |
| PRV-01 | Credential encrypted before DB insert, decrypted on read | unit | `go test ./internal/store/... -run TestCredentialEncryption` | Wave 0 |
| PRV-02 | After key deletion, credential decrypt returns error | unit | `go test ./internal/store/... -run TestCryptoShred` | Wave 0 |
| PRV-03 | DeleteEntity deactivates DID Doc, queues shredded message | integration | `go test ./internal/api/... -run TestDeleteEntityCryptoShred` | Wave 0 |
| PRV-04 | Personhood endpoint rejects raw biometric fields | unit | `go test ./internal/api/... -run TestPersonhoodNoBiometric` | Wave 0 |
| API-04 | All 6 credential endpoints respond correctly | integration | `go test ./internal/api/... -run TestCredentialEndpoints` | Wave 0 |
| MSG-06 | Org delegate fan-out dispatches to ≤50 delegates; first-response-wins atomicity | unit | `go test ./internal/store/... -run TestOrgDelegateFanOut` | Wave 0 |
| MOB-01..06 | Flutter widget and integration tests | manual | `cd mobile && flutter test` | Wave 0 |

### Sampling Rate

- **Per task commit:** `cd platform && go test ./internal/credentials/... ./internal/store/...`
- **Per wave merge:** `cd platform && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `platform/internal/credentials/credentials_test.go` — covers CRD-01 through CRD-06
- [ ] `platform/internal/store/credentials_test.go` — covers PRV-01, PRV-02
- [ ] `platform/internal/api/credentials_test.go` — covers API-04, PRV-03, PRV-04
- [ ] `platform/internal/store/org_delegates_test.go` — covers MSG-06
- [ ] `platform/migrations/012_credentials.up.sql` — new schema migration
- [ ] `mobile/test/` — Flutter widget tests for new screens
- [ ] `go get github.com/trustbloc/vc-go/verifiable@v1.3.6` — new dependency

---

## Sources

### Primary (HIGH confidence)

- ATAP spec §5.7, §6, §7.5, §12, §13 (`spec/ATAP-SPEC-v1.0-rc1.md`) — spec is authoritative for all requirement details
- `pkg.go.dev/github.com/trustbloc/vc-go/verifiable` — verified v1.3.6 API, Jan 2026
- `pkg.go.dev/github.com/MichaelFraser99/go-sd-jwt` — verified v1.4.0, Aug 2025
- Go stdlib `crypto/aes`, `crypto/cipher`, `compress/gzip` — standard library, no version concern
- `pub.dev/packages/biometric_signature` v10.2.0 — verified ECDSA only, no Ed25519
- `pub.dev/packages/local_auth` — verified Flutter official plugin
- Existing codebase: `platform/internal/models/models.go`, `store/store.go`, `api/auth.go` — shows exact patterns in use

### Secondary (MEDIUM confidence)

- W3C Bitstring Status List v1.0 spec (`https://www.w3.org/TR/vc-bitstring-status-list/`) — confirmed name change from StatusList2021, GZIP+base64url encoding format
- W3C VC 2.0 family became W3C Recommendation May 2025 — confirmed `validFrom` and v2 context URL
- RFC 9901 SD-JWT became IETF Standard Nov 2025 — confirmed RFC number

### Tertiary (LOW confidence — flagged for validation)

- Personhood provider integration (World ID, etc.) — spec allows server-issued stub; no external provider investigated for v1.0
- trustbloc/vc-go JSON-LD context offline loading — not tested directly; known pattern in Go VC ecosystem

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified on pkg.go.dev and pub.dev as of March 2026
- Architecture: HIGH — patterns derived directly from spec and existing codebase conventions
- Pitfalls: HIGH (biometric ECDSA vs Ed25519 mismatch) — confirmed from biometric_signature docs; MEDIUM (vc-go context loader) — known issue class, not directly confirmed for this version

**Research date:** 2026-03-14
**Valid until:** 2026-04-14 (stable spec, stable libraries; trustbloc/vc-go active monthly)
