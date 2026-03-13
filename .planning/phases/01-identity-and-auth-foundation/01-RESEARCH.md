# Phase 1: Identity and Auth Foundation - Research

**Researched:** 2026-03-13
**Domain:** DID:web identity, OAuth 2.1 + DPoP, Go backend (Fiber v2, pgx/v5)
**Confidence:** HIGH (primary via official RFCs and verified Go library docs)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INF-01 | Strip old signal pipeline code (signals, channels, webhooks, custom auth, SSE) | Identified all files to delete; entities table survives, 6 other migrations drop |
| INF-02 | Database migration from signal-based schema to DID/approval/VC-based schema | New schema designed: entities (extended), oauth_clients, oauth_tokens, oauth_auth_codes, key_versions |
| INF-03 | Docker Compose updated for new service configuration | Existing Docker Compose keeps Postgres + Redis; Redis role changes to OAuth state/nonce storage |
| DID-01 | Every entity identified by `did:web` DID with path `{server}:{type}:{id}` | did:web path format verified against W3C spec; DID string construction pattern confirmed |
| DID-02 | DID Documents hosted at standard did:web HTTPS path with Ed25519 verification keys | Resolution path: `/{type}/{id}/did.json`; Ed25519VerificationKey2020 format confirmed |
| DID-03 | DID Documents include ATAP properties in `https://atap.dev/ns/v1` context | JSON-LD multi-context array pattern confirmed; custom context layered over DID core v1 |
| DID-04 | Four entity types: human, agent, machine, org | Existing `type` column and constants survive; DID path prefix derives from type |
| DID-05 | Human IDs derived from public key: `lowercase(base32(sha256(pubkey))[:16])` | `DeriveHumanID()` already implemented correctly in `crypto/crypto.go` |
| DID-06 | Agent DID Documents MUST include `atap:principal` referencing controlling entity | JSON field in DID Document; stored as FK in entities table (`principal_did` column) |
| DID-07 | Key rotation via DID Document update; previous key versions retained | Requires new `key_versions` table; current entity has single key |
| DID-08 | DID resolution uses HTTPS with valid TLS certificate | Handled by infrastructure (TLS termination at reverse proxy); no app code change |
| AUTH-01 | OAuth 2.1 with DPoP (RFC 9449) for API access | go-jose/v4 (already in go.mod as indirect) + AxisCommunications/go-dpop confirmed |
| AUTH-02 | Agent entities use Client Credentials grant | Token endpoint: POST /v1/oauth/token; client_credentials grant type |
| AUTH-03 | Human entities use Authorization Code + PKCE + biometric | Auth endpoint: GET /v1/oauth/authorize; code + code_challenge pattern |
| AUTH-04 | All tokens DPoP-bound with proof JWT on each request | DPoP middleware validates proof on every authenticated request |
| AUTH-05 | Token scopes: `atap:inbox`, `atap:send`, `atap:approve`, `atap:manage` | Scope strings stored as TEXT[] in token table |
| AUTH-06 | Default access token 1 hour, refresh up to 90 days | Token expiry columns in oauth_tokens table |
| SRV-01 | Server discovery via `/.well-known/atap.json` | Static handler; JSON structure defined below |
| SRV-02 | Server trust levels (L0-L3) | Published in discovery doc; no runtime enforcement in Phase 1 |
| SRV-03 | `max_approval_ttl` in discovery doc and enforced | Published in discovery doc; enforcement deferred to approval phases |
| API-01 | Entity endpoints: POST /v1/entities, GET /v1/entities/{id}, DELETE /v1/entities/{id} | New entity registration flow (replaces old `/v1/register`) |
| API-02 | DID resolution: GET /{type}/{id}/did.json | New route outside /v1/ prefix; returns JSON-LD DID Document |
| API-06 | All errors follow RFC 7807 with `https://atap.dev/errors/{type}` URIs | `problem()` helper already correct; update error type strings |
</phase_requirements>

---

## Summary

Phase 1 rebuilds the ATAP platform's identity and authentication layer from a custom protocol to standards-based infrastructure. The existing Go backend (Fiber v2, pgx/v5, zerolog) is kept — only the protocol layer changes. The core work is three independent subsystems: (1) strip the old signal pipeline and rewrite the entity model around `did:web` DIDs with JSON-LD DID Documents, (2) build an OAuth 2.1 authorization server with DPoP token binding using `go-jose/v4` and `go-dpop`, and (3) add the server discovery endpoint.

The key architectural shift: authentication moves from custom Ed25519 signed requests (the `Signature keyId=...` Authorization header) to OAuth 2.1 Bearer tokens that are DPoP-bound. Entity identity shifts from custom `agent://` URIs to W3C `did:web` DIDs with publicly resolvable DID Documents. Both systems can be built without external auth providers — the platform IS the authorization server.

**Primary recommendation:** Build the OAuth 2.1 authorization server from scratch using `go-jose/v4` (already in go.mod) and `github.com/AxisCommunications/go-dpop` for DPoP proof validation. Do not use Fosite (DPoP not natively supported) or external providers. The existing Fiber + pgx infrastructure handles routing and persistence cleanly.

---

## Standard Stack

### Core (Already in go.mod)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/go-jose/go-jose/v4 | v4.1.3 | JWT signing/verification, JWK thumbprints | Official JOSE implementation, Ed25519 support confirmed |
| github.com/gowebpki/jcs | v1.0.1 | RFC 8785 JCS canonical JSON | Already used for signal signing |
| github.com/oklog/ulid/v2 | v2.1.1 | ULID generation for entity IDs | Already in use |
| github.com/jackc/pgx/v5 | v5.7.5 | PostgreSQL driver | Already in use |
| github.com/gofiber/fiber/v2 | v2.52.12 | HTTP routing | Already in use |

### New Dependencies to Add
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/AxisCommunications/go-dpop | latest | RFC 9449 DPoP proof generation + validation | Ed25519 support confirmed, both client and server sides |
| github.com/google/uuid | v1.6.0 | RFC 4122 UUID v4 for DPoP `jti` nonces | Already in go.mod as indirect; promote to direct |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-dpop | pquerna/dpop | pquerna is less maintained; go-dpop has explicit Ed25519 support and active maintenance |
| Custom OAuth server | Fosite/ory | Fosite has no native DPoP support (open issue #641); adds significant complexity for minimal benefit |
| Custom OAuth server | External IdP (Keycloak, Auth0) | Spec mandates platform IS the authorization server; external IdP not feasible |

**Installation:**
```bash
cd platform && go get github.com/AxisCommunications/go-dpop
```

---

## Architecture Patterns

### Recommended Project Structure

New files and changes within `platform/`:

```
platform/
  cmd/server/
    main.go               — update: remove old pipeline wiring, add OAuth server
  internal/
    api/
      api.go              — replace: remove signal/channel/webhook handlers; add DID + OAuth + discovery routes
      auth.go             — new: DPoP-bound Bearer token middleware (replaces Ed25519 auth middleware)
      did.go              — new: DID Document handler (GET /{type}/{id}/did.json)
      discovery.go        — new: /.well-known/atap.json handler
      oauth.go            — new: OAuth 2.1 token + authorize endpoints
      entities.go         — new: POST /v1/entities, GET /v1/entities/{id}, DELETE /v1/entities/{id}
    models/
      models.go           — replace: strip Signal/Channel/Webhook types; add DIDDocument, OAuthToken, OAuthClient, OAuthAuthCode
    store/
      store.go            — replace: strip signal/channel/webhook/claim/delegation stores; add entity, oauth, key_version stores
    crypto/
      crypto.go           — extend: add JWK thumbprint helper; keep DeriveHumanID, CanonicalJSON
  migrations/
    008_strip_old_pipeline.down.sql
    008_strip_old_pipeline.up.sql  — DROP TABLE signals, channels, webhook_delivery, claims, delegations
    009_did_and_oauth.up.sql       — new entity columns (did, principal_did), oauth_clients, oauth_tokens, oauth_auth_codes, key_versions
    009_did_and_oauth.down.sql
```

### Pattern 1: DID Construction and Document Structure

**What:** Build a `did:web` DID from entity fields, then serve its JSON-LD DID Document at the standard HTTPS path.

**DID format:** `did:web:{domain}:{type}:{entity_id}`
Example: `did:web:atap.app:agent:01jqkm9f3p0000000000000000`

**Resolution path:** The colon-separated path after the domain maps to URL slashes: `GET /agent/01jqkm9f3p0000000000000000/did.json`

**DID Document structure:**
```json
{
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1",
    "https://atap.dev/ns/v1"
  ],
  "id": "did:web:atap.app:agent:01jqkm9f3p0000000000000000",
  "verificationMethod": [{
    "id": "did:web:atap.app:agent:01jqkm9f3p0000000000000000#key-1",
    "type": "Ed25519VerificationKey2020",
    "controller": "did:web:atap.app:agent:01jqkm9f3p0000000000000000",
    "publicKeyMultibase": "z6Mkf..."
  }],
  "authentication": ["did:web:atap.app:agent:01jqkm9f3p0000000000000000#key-1"],
  "assertionMethod": ["did:web:atap.app:agent:01jqkm9f3p0000000000000000#key-1"],
  "atap:type": "agent",
  "atap:principal": "did:web:atap.app:human:abcdef1234567890"
}
```

**Key DID-specific note:** `publicKeyMultibase` uses multibase encoding: prefix `z` (base58btc) + base58-encoded raw public key bytes. This is NOT base64. Requires `encoding/base58` or manual multibase encoding.

**Source:** W3C did:web Method Specification, W3C DID Core, Ed25519VerificationKey2020 suite spec.

### Pattern 2: OAuth 2.1 Token Endpoint with DPoP

**What:** A self-contained authorization server handling two grant types with DPoP token binding.

**Token endpoint:** `POST /v1/oauth/token`

**Client Credentials flow (agents/machines):**
```
Request:
  POST /v1/oauth/token
  DPoP: <proof_jwt>
  Content-Type: application/x-www-form-urlencoded

  grant_type=client_credentials
  &client_id=did:web:atap.app:agent:01jqkm...
  &client_secret=<secret_registered_at_entity_creation>
  &scope=atap:inbox atap:send

Response:
  {
    "access_token": "<opaque_or_jwt>",
    "token_type": "DPoP",
    "expires_in": 3600,
    "scope": "atap:inbox atap:send"
  }
```

**Authorization Code + PKCE flow (humans):**
```
Step 1: GET /v1/oauth/authorize
  ?response_type=code
  &client_id=did:web:atap.app:human:abcdef...
  &redirect_uri=atap://callback
  &scope=atap:approve atap:manage
  &code_challenge=<S256_challenge>
  &code_challenge_method=S256
  &state=<random>

Step 2: POST /v1/oauth/token
  DPoP: <proof_jwt>
  grant_type=authorization_code
  &code=<auth_code>
  &redirect_uri=atap://callback
  &code_verifier=<raw_verifier>
```

### Pattern 3: DPoP Proof Validation Middleware

**What:** Every authenticated API request carries both `Authorization: DPoP <token>` and `DPoP: <proof_jwt>`. The middleware validates both.

**Validation sequence:**
1. Extract `DPoP` header → parse proof JWT using `go-dpop`
2. Verify proof `htm` matches request method, `htu` matches request URL
3. Verify proof `iat` within 60-second window (anti-replay)
4. Check proof `jti` not seen before (nonce replay cache in Redis with TTL=5min)
5. Extract `Authorization: DPoP <token>` → look up token in DB
6. Verify token `cnf.jkt` thumbprint matches `jwk` thumbprint in proof header
7. Set entity in `c.Locals("entity")`

**DPoP nonce replay cache key:** `dpop:nonce:{jti}` in Redis, TTL 5 minutes.

### Pattern 4: Database Schema for OAuth 2.1

**New tables required:**

```sql
-- Extend entities with DID fields
ALTER TABLE entities ADD COLUMN did TEXT UNIQUE;          -- did:web:... string
ALTER TABLE entities ADD COLUMN principal_did TEXT;       -- for agents: their controlling human/org DID
ALTER TABLE entities ADD COLUMN client_secret_hash TEXT;  -- bcrypt hash; agents only

-- Key versions for DID-07 (rotation)
CREATE TABLE key_versions (
    id          TEXT PRIMARY KEY,    -- key_... identifier
    entity_id   TEXT NOT NULL REFERENCES entities(id),
    public_key  BYTEA NOT NULL,
    key_index   INTEGER NOT NULL DEFAULT 1,  -- monotonic, for #key-N fragment
    valid_from  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,         -- NULL = currently active
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth 2.1 auth codes (for Authorization Code + PKCE)
CREATE TABLE oauth_auth_codes (
    code              TEXT PRIMARY KEY,
    entity_id         TEXT NOT NULL REFERENCES entities(id),
    redirect_uri      TEXT NOT NULL,
    scope             TEXT[] NOT NULL,
    code_challenge    TEXT NOT NULL,        -- S256 challenge
    dpop_jkt          TEXT NOT NULL,        -- JWK thumbprint from initial DPoP proof
    expires_at        TIMESTAMPTZ NOT NULL,
    used_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OAuth 2.1 access + refresh tokens
CREATE TABLE oauth_tokens (
    id              TEXT PRIMARY KEY,       -- opaque random token value (stored; hash if paranoid)
    entity_id       TEXT NOT NULL REFERENCES entities(id),
    token_type      TEXT NOT NULL DEFAULT 'access',  -- 'access' | 'refresh'
    scope           TEXT[] NOT NULL,
    dpop_jkt        TEXT NOT NULL,          -- JWK thumbprint binding
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_oauth_tokens_entity ON oauth_tokens(entity_id, token_type);
CREATE INDEX idx_oauth_tokens_expires ON oauth_tokens(expires_at);
```

### Anti-Patterns to Avoid

- **Don't verify DPoP jwk against stored public key at token endpoint:** At the token endpoint, the DPoP public key in the proof IS the key being registered — it doesn't need to match an existing stored key. Only at resource server endpoints does the proof key need to match the stored `cnf.jkt`.
- **Don't store token hashes for DPoP `ath`:** `ath` in DPoP proofs is the base64url(SHA-256(access_token_value)). Store the raw token value in the DB; compute SHA-256 at validation time.
- **Don't use JSON-LD `@context` as a string when extension contexts are needed:** The `@context` must be an array when ATAP adds its custom context. `"@context": ["https://www.w3.org/ns/did/v1", "https://atap.dev/ns/v1"]` not `"@context": "https://www.w3.org/ns/did/v1"`.
- **Don't reuse entity ULID as the human ID:** Human IDs are derived from the public key hash (`DeriveHumanID()`). Agent/machine/org IDs use ULID. The existing `DeriveHumanID()` function is already correct.
- **Don't use `Authorization: Bearer` with DPoP:** The token type for DPoP-bound tokens is `DPoP`. Middleware must reject `Bearer` scheme for DPoP-protected endpoints.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT signing/verification | Custom JWS impl | `go-jose/v4` | JWS has subtle serialization edge cases; go-jose is battle-tested |
| DPoP proof validation | Manual JWT header parsing + thumbprint | `github.com/AxisCommunications/go-dpop` | RFC 9449 has precise timing + replay rules; off-by-one errors cause security holes |
| PKCE code verifier/challenge | Custom SHA-256 + base64url | `crypto/sha256` + `encoding/base64.RawURLEncoding` from stdlib | No external dep needed; pattern is 4 lines |
| JWK SHA-256 thumbprint | Custom JWK serialization | `go-jose/v4 JSONWebKey.Thumbprint(crypto.SHA256)` | JWK thumbprint spec (RFC 7638) has precise field ordering |
| Multibase encoding for Ed25519 public key in DID doc | Custom base58 | Add `github.com/mr-tron/base58` | Multibase `z` prefix = base58btc encoding; stdlib has no base58 |

**Key insight:** OAuth 2.1 has no "small" implementation. PKCE, DPoP, token binding, replay prevention, and scope enforcement each have security-critical edge cases. Use library primitives for crypto; only the orchestration logic is custom.

---

## Common Pitfalls

### Pitfall 1: DID Resolution Path Construction

**What goes wrong:** The `did:web` method encodes colons as path separators. `did:web:atap.app:agent:abc123` resolves to `https://atap.app/agent/abc123/did.json`. A common mistake is also appending `/.well-known` (which only applies when there is NO path component).

**Why it happens:** The `/.well-known/did.json` fallback only applies to bare-domain DIDs like `did:web:example.com`. Path-based DIDs (all ATAP entity DIDs have `:{type}:{id}`) use direct path mapping.

**How to avoid:** Route registration: `app.Get("/:type/:id/did.json", h.ResolveDID)`. Validate that `type` is one of the four entity types.

### Pitfall 2: Ed25519VerificationKey2020 Public Key Encoding

**What goes wrong:** The existing `crypto.EncodePublicKey()` returns base64-standard (not multibase). DID Documents using `Ed25519VerificationKey2020` require `publicKeyMultibase` with `z` (base58btc) prefix, NOT `publicKeyBase64` or raw base64.

**Why it happens:** The existing codebase uses base64 for everything; DID Document spec requires multibase.

**How to avoid:** Add `encodeMultibase(pubkey []byte) string` in the crypto package: base58-encode the raw 32 bytes, prepend `z`. This requires adding `github.com/mr-tron/base58`.

### Pitfall 3: DPoP Replay Attack Window

**What goes wrong:** If the `jti` nonce cache in Redis expires too quickly or isn't checked, DPoP proofs can be replayed within their valid time window (up to 60 seconds per spec).

**Why it happens:** Redis TTL set to match `iat` skew tolerance but not accounting for network latency.

**How to avoid:** Set Redis nonce TTL to at least 5 minutes (much larger than the 60-second proof window). Key: `dpop:nonce:{jti}`, value: `1`, TTL: 300s.

### Pitfall 4: OAuth Authorization Code MUST Be Single-Use

**What goes wrong:** An authorization code used twice returns a token on both calls if the `used_at` column check isn't atomic.

**Why it happens:** Race condition: two requests arrive simultaneously with the same code.

**How to avoid:** Use PostgreSQL `UPDATE oauth_auth_codes SET used_at = NOW() WHERE code = $1 AND used_at IS NULL RETURNING *`. If 0 rows affected, the code was already used.

### Pitfall 5: Old Auth Middleware Breaking New Endpoints

**What goes wrong:** The existing `AuthMiddleware()` uses the custom `Signature keyId=...` header scheme. If both old and new middleware are registered simultaneously, old clients get through and new DPoP checks are skipped.

**Why it happens:** Gradual migration leaves both middleware active on shared routes.

**How to avoid:** This is a clean-break migration. Delete the old `AuthMiddleware()` in INF-01. The new DPoP middleware is the only auth path. No backward compatibility.

### Pitfall 6: PKCE code_challenge_method Must Be S256

**What goes wrong:** OAuth 2.1 (unlike 2.0) mandates `S256` for PKCE. Accepting `plain` method is a security regression.

**Why it happens:** Legacy code or tutorial examples use `plain` for simplicity.

**How to avoid:** Reject any `code_challenge_method` that is not exactly `"S256"` with HTTP 400.

### Pitfall 7: Missing `Content-Type: application/did+ld+json` on DID Documents

**What goes wrong:** Returning DID Documents as `application/json` instead of `application/did+ld+json` breaks standards-compliant DID resolvers.

**Why it happens:** Fiber's `c.JSON()` defaults to `application/json`.

**How to avoid:** Set `c.Set("Content-Type", "application/did+ld+json")` before writing the response in the DID resolution handler.

---

## Code Examples

### DID Construction
```go
// Source: W3C did:web Method Specification (w3c-ccg.github.io/did-method-web)
// DID format: did:web:{domain}:{type}:{id}
func BuildDID(domain, entityType, entityID string) string {
    return fmt.Sprintf("did:web:%s:%s:%s", domain, entityType, entityID)
}

// DID Document URL from DID (used for routing)
func DIDToPath(entityType, entityID string) string {
    // did:web:domain:agent:abc → /agent/abc/did.json
    return fmt.Sprintf("/%s/%s/did.json", entityType, entityID)
}
```

### Multibase Ed25519 Public Key Encoding
```go
// Source: Ed25519VerificationKey2020 suite spec
// 'z' prefix = base58btc in multibase spec
import "github.com/mr-tron/base58"

func EncodePublicKeyMultibase(pub ed25519.PublicKey) string {
    return "z" + base58.Encode(pub)
}
```

### JWK Thumbprint Computation (for DPoP cnf.jkt)
```go
// Source: go-jose/v4 docs (pkg.go.dev/github.com/go-jose/go-jose/v4)
import (
    "crypto"
    "github.com/go-jose/go-jose/v4"
)

func JWKThumbprint(pub ed25519.PublicKey) (string, error) {
    jwk := jose.JSONWebKey{Key: pub, Algorithm: string(jose.EdDSA)}
    tb, err := jwk.Thumbprint(crypto.SHA256)
    if err != nil {
        return "", err
    }
    return base64.RawURLEncoding.EncodeToString(tb), nil
}
```

### JWT Access Token Issuance (go-jose/v4)
```go
// Source: go-jose/v4/jwt docs (pkg.go.dev/github.com/go-jose/go-jose/v4/jwt)
import (
    "github.com/go-jose/go-jose/v4"
    "github.com/go-jose/go-jose/v4/jwt"
)

func IssueAccessToken(platformPrivKey ed25519.PrivateKey, sub, issuer, jkt string, scopes []string, ttl time.Duration) (string, error) {
    sig, err := jose.NewSigner(
        jose.SigningKey{Algorithm: jose.EdDSA, Key: platformPrivKey},
        (&jose.SignerOptions{}).WithType("JWT"),
    )
    if err != nil {
        return "", err
    }

    now := time.Now()
    claims := jwt.Claims{
        Subject:   sub,          // entity DID
        Issuer:    issuer,       // platform domain
        IssuedAt:  jwt.NewNumericDate(now),
        Expiry:    jwt.NewNumericDate(now.Add(ttl)),
        ID:        uuid.NewString(), // jti for revocation
    }
    // cnf claim with jkt for DPoP binding
    cnfClaims := struct {
        CNF struct {
            JKT string `json:"jkt"`
        } `json:"cnf"`
        Scope string `json:"scope"`
    }{
        Scope: strings.Join(scopes, " "),
    }
    cnfClaims.CNF.JKT = jkt

    return jwt.Signed(sig).Claims(claims).Claims(cnfClaims).Serialize()
}
```

### DPoP Proof Validation (go-dpop)
```go
// Source: github.com/AxisCommunications/go-dpop docs
import "github.com/AxisCommunications/go-dpop"

func ValidateDPoPProof(proofHeader, method, rawURL, accessToken string) (string, error) {
    u, _ := url.Parse(rawURL)
    window := 60 * time.Second
    proof, err := dpop.Parse(proofHeader, dpop.Method(method), u, dpop.ParseOptions{
        TimeWindow: &window,
    })
    if err != nil {
        return "", fmt.Errorf("invalid DPoP proof: %w", err)
    }
    // Returns JWK thumbprint string for binding verification
    return proof.PublicKey(), nil
}
```

### PKCE S256 Challenge Verification
```go
// Source: RFC 7636 §4.6 (standard library crypto/sha256)
import (
    "crypto/sha256"
    "encoding/base64"
)

func VerifyPKCE(storedChallenge, codeVerifier string) bool {
    h := sha256.Sum256([]byte(codeVerifier))
    computed := base64.RawURLEncoding.EncodeToString(h[:])
    return storedChallenge == computed
}
```

### Server Discovery Document
```go
// Source: ATAP Spec §9 (SRV-01, SRV-02, SRV-03)
func (h *Handler) Discovery(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "domain":            h.config.PlatformDomain,
        "api_base":          fmt.Sprintf("https://%s/v1", h.config.PlatformDomain),
        "didcomm_endpoint":  fmt.Sprintf("https://%s/v1/didcomm", h.config.PlatformDomain),
        "claim_types":       []string{},   // populated in Phase 2+
        "max_approval_ttl":  86400,        // 24 hours in seconds; Phase 3 enforcement
        "trust_level":       1,            // L1: DV TLS
        "oauth": fiber.Map{
            "token_endpoint":     fmt.Sprintf("https://%s/v1/oauth/token", h.config.PlatformDomain),
            "authorize_endpoint": fmt.Sprintf("https://%s/v1/oauth/authorize", h.config.PlatformDomain),
            "grant_types":        []string{"client_credentials", "authorization_code"},
            "dpop_required":      true,
        },
    })
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom `agent://` URIs | W3C `did:web` DIDs | ATAP v1.0-rc1 (2026) | Full decentralized resolution, no custom resolver needed |
| Ed25519 signed request headers (`Signature keyId=...`) | OAuth 2.1 Bearer tokens bound via DPoP | ATAP v1.0-rc1 (2026) | Standard tooling, token scopes, refresh flows |
| Server-generated keypairs returned to client | Client generates keypair, registers public key | ATAP v1.0-rc1 (2026) | Private keys never leave device |
| Custom claim codes (`ATAP-XXXX`) | W3C VCs | ATAP v1.0-rc1 (2026) | Standardized, verifiable, portable |
| Custom signal inbox | DIDComm v2.1 | ATAP v1.0-rc1 (2026) | Encrypted, routable, federable |
| `Ed25519VerificationKey2018` in DID docs | `Ed25519VerificationKey2020` | 2020-2021 | More precise encoding spec using multibase |

**Deprecated/outdated:**
- `signals`, `channels`, `webhook_delivery`, `claims`, `delegations` tables: drop entirely in migration 008
- `RegisterAgent` handler (POST /v1/register): replace with `POST /v1/entities`
- `AuthMiddleware()` using Signature header scheme: delete entirely
- Entity `URI` field (`agent://...`): replaced by `DID` field (`did:web:...`)
- `RegisterResponse` returning `PrivateKey`: SECURITY VIOLATION in new model — server never generates or stores private keys

---

## Open Questions

1. **Client registration for Authorization Code flow**
   - What we know: Client Credentials flow is self-contained (entity's DID is the client_id). Authorization Code flow needs a registered `redirect_uri`.
   - What's unclear: Does the mobile app register its redirect URI at entity creation time, or is it a separate endpoint? The spec (§13) doesn't define a client registration endpoint.
   - Recommendation: Store `redirect_uris` as a JSONB column on the entities table. Set at entity registration for human/org entities; default to the platform's mobile app scheme (`atap://callback`).

2. **Human entity registration flow with keypair**
   - What we know: Old flow had server generating the keypair. New model: client generates keypair, sends public key. Human ID is derived from public key.
   - What's unclear: Does `POST /v1/entities` for a human type accept `public_key` in the request body? Or does human registration happen via Authorization Code flow only?
   - Recommendation: `POST /v1/entities` with `{"type": "human", "public_key": "<multibase>"}` creates the entity and returns the DID. The human then uses that DID as `client_id` in OAuth flows.

3. **DPoP nonce server challenge (RFC 9449 §8)**
   - What we know: RFC 9449 allows servers to mandate use of server-provided nonces via `DPoP-Nonce` response header + `use_dpop_nonce` error.
   - What's unclear: The spec doesn't mandate this, and it adds a round-trip.
   - Recommendation: Skip server-issued nonces for Phase 1. Client-generated `jti` with Redis replay cache is sufficient. Add server nonces in a later security hardening phase.

4. **Authorization Code redirect for mobile (custom URI scheme)**
   - What we know: Mobile app uses `atap://callback` as redirect URI. PKCE handles CSRF without state validation by the server.
   - What's unclear: How does the server deliver the auth code to the mobile client? In-app browser with redirect, or direct response?
   - Recommendation: Support both `atap://callback` (mobile deep link) and HTTP redirects. The server issues a `302 Location: atap://callback?code=...&state=...` response.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package + testcontainers-go (already in go.mod) |
| Config file | none — table-driven tests in `*_test.go` files next to source |
| Quick run command | `cd platform && go test ./internal/... -run TestDID -v` |
| Full suite command | `cd platform && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DID-01 | DID string construction from entity fields | unit | `go test ./internal/crypto -run TestBuildDID -v` | ❌ Wave 0 |
| DID-02 | DID Document returned at correct path with correct fields | unit | `go test ./internal/api -run TestResolveDID -v` | ❌ Wave 0 |
| DID-03 | DID Document contains ATAP context and `atap:type` | unit | `go test ./internal/api -run TestResolveDIDContext -v` | ❌ Wave 0 |
| DID-05 | Human ID derived correctly from public key | unit | `go test ./internal/crypto -run TestDeriveHumanID -v` | ✅ exists (crypto_test.go) |
| DID-07 | Key rotation stores previous key version with validity period | unit | `go test ./internal/store -run TestKeyRotation -v` | ❌ Wave 0 |
| AUTH-01/04 | DPoP proof validation rejects bad `htm`, expired `iat`, replayed `jti` | unit | `go test ./internal/api -run TestDPoP -v` | ❌ Wave 0 |
| AUTH-02 | Client credentials grant issues DPoP-bound access token | integration | `go test ./internal/api -run TestClientCredentials -v` | ❌ Wave 0 |
| AUTH-03 | Authorization code + PKCE issues token; rejects bad verifier | integration | `go test ./internal/api -run TestAuthCode -v` | ❌ Wave 0 |
| AUTH-05 | Token scopes enforced on API endpoints | unit | `go test ./internal/api -run TestScopeEnforcement -v` | ❌ Wave 0 |
| AUTH-06 | Access token expires after 1 hour; refresh token after 90 days | unit | `go test ./internal/api -run TestTokenExpiry -v` | ❌ Wave 0 |
| SRV-01 | Discovery doc returns required fields | unit | `go test ./internal/api -run TestDiscovery -v` | ❌ Wave 0 |
| API-01 | POST /v1/entities creates entity and returns DID | unit | `go test ./internal/api -run TestCreateEntity -v` | ❌ Wave 0 |
| API-06 | All error responses have RFC 7807 type URI format | unit | `go test ./internal/api -run TestProblemDetail -v` | ❌ Wave 0 (existing `problem()` helper) |
| INF-01 | Old signal/channel/webhook routes return 404 | unit | `go test ./internal/api -run TestOldRoutesGone -v` | ❌ Wave 0 |
| INF-02 | New schema migrations apply without error | integration | `go test ./internal/store -run TestMigrations -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./internal/... -run <relevant_prefix> -v -count=1`
- **Per wave merge:** `cd platform && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `platform/internal/crypto/did_test.go` — covers DID-01: DID construction, multibase encoding
- [ ] `platform/internal/api/did_test.go` — covers DID-02, DID-03: DID Document structure and routing
- [ ] `platform/internal/api/oauth_test.go` — covers AUTH-01 through AUTH-06: token flows
- [ ] `platform/internal/api/discovery_test.go` — covers SRV-01: discovery document
- [ ] `platform/internal/api/entities_test.go` — covers API-01: entity CRUD
- [ ] `platform/internal/store/migrations_test.go` — covers INF-02: migration correctness
- [ ] No framework install needed — `go test` is built-in; testcontainers-go already in go.mod

---

## Sources

### Primary (HIGH confidence)
- W3C did:web Method Specification (w3c-ccg.github.io/did-method-web) — resolution algorithm, DID Document path format
- RFC 9449 (datatracker.ietf.org/doc/html/rfc9449) — DPoP proof structure, cnf/jkt binding, token endpoint requirements
- pkg.go.dev/github.com/go-jose/go-jose/v4 — Ed25519 support confirmed, JWT signing API, JWK Thumbprint method
- pkg.go.dev/github.com/AxisCommunications/go-dpop — Ed25519 support confirmed, Parse/Validate API
- Existing codebase (platform/go.mod) — go-jose/v4 already an indirect dependency; can promote to direct

### Secondary (MEDIUM confidence)
- W3C DID Core JSON-LD context (@context array pattern for extension contexts) — verified via W3C DID WG issue discussion
- Ed25519VerificationKey2020 specification (w3id.org/security/suites/ed25519-2020/v1) — publicKeyMultibase encoding requirement; multibase `z` = base58btc
- Fosite DPoP issue #641 (github.com/ory/fosite) — confirmed DPoP not natively supported; custom implementation required

### Tertiary (LOW confidence)
- Base58 encoding library recommendation (`github.com/mr-tron/base58`) — widely used in Go DID tooling but not independently verified against multibase spec; validate against multibase RFC before committing

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — go-jose/v4 and go-dpop verified via official Go package docs; both confirm Ed25519 and RFC 9449 compliance
- Architecture: HIGH — did:web resolution algorithm verified against W3C spec; OAuth 2.1 token flows verified against RFC 9449
- Pitfalls: HIGH — old auth middleware conflict, DPoP replay, multibase encoding all verified against source specs
- Schema design: MEDIUM — derived from spec requirements; specific column types may need adjustment during implementation

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (30 days — stable specs, but go-dpop library maintenance should be re-checked)
