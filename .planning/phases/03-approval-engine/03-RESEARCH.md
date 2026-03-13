# Phase 3: Approval Engine - Research

**Researched:** 2026-03-13
**Domain:** Multi-signature approval engine — JWS/JCS signing, approval lifecycle state machine, template fetching, REST API
**Confidence:** HIGH

## Summary

Phase 3 is the protocol's core contribution: a multi-signature approval document where each party (requester, optional mediator, approver) signs independently via JWS Compact Serialization with detached payload, producing a self-contained offline-verifiable proof of consent. The spec (§8 and §11) is fully normative and leaves almost no design freedom — the data model, signing algorithm, state machine, field names, message types, and API endpoints are all specified precisely.

The implementation builds entirely on work already completed in Phases 1 and 2. The cryptographic primitives (Ed25519 via `crypto/ed25519`, JCS via `github.com/gowebpki/jcs`, ULID IDs via `github.com/oklog/ulid/v2`, go-jose for JWS) are already present in `go.mod`. The DIDComm message types (`TypeApprovalRequest`, `TypeApprovalResponse`, `TypeApprovalRevoke`, `TypeApprovalRejected`) are already defined in `platform/internal/didcomm/message.go`. The pattern for HTTP handler → Store interface → pgx is fully established.

The phase has three distinct workstreams: (1) the approval data model + database schema + store layer, (2) the signing/verification logic (JWS detached payload over JCS canonical JSON), and (3) the REST API handlers + DIDComm integration for the full two-party and three-party flows. Template fetching (TPL requirements) is an HTTP-side concern with specific SSRF-prevention requirements.

**Primary recommendation:** Implement in three plans: data model + schema + store, JWS signing + verification engine, API handlers + DIDComm dispatch + template fetching.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| APR-01 | Two-party approvals: `from` signs, sends to `to` who approves/declines (2 signatures) | Spec §8.2, §8.4 two-party sequence |
| APR-02 | Three-party approvals: `from` signs → `via` validates + co-signs → `to` approves/declines (3 signatures) | Spec §8.2, §8.4 three-party sequence |
| APR-03 | Approval format: `atap_approval: "1"`, `apr_` + ULID IDs, ISO 8601 timestamps | Spec §8.5, §8.6 field table |
| APR-04 | Subject contains `type` (reverse-domain), `label`, `reversible` boolean, `payload` (system-specific JSON) | Spec §8.7 subject table |
| APR-05 | JWS Compact Serialization with detached payload (RFC 7515 + RFC 7797) for each signature | Spec §8.8 signatures section |
| APR-06 | Signed payload is UTF-8 of JCS-serialized (RFC 8785) approval excluding `signatures` field | Spec §8.8; `gowebpki/jcs` already in use |
| APR-07 | Full approval lifecycle: requested → approved/declined/expired/rejected → consumed/revoked | Spec §8.3 lifecycle diagram |
| APR-08 | System rejection with `approval/1.0/rejected` message type and standardized reason codes | Spec §8.10; `TypeApprovalRejected` already defined |
| APR-09 | One-time approvals (`valid_until` absent) transition to `consumed` after single use | Spec §8.12 |
| APR-10 | Persistent approvals (`valid_until` set) with receiver-side `max_approval_ttl` enforcement | Spec §8.12, §8.14; `max_approval_ttl` in discovery doc |
| APR-11 | Chained approvals via `parent` field; revoking parent invalidates children | Spec §8.13, §8.15 |
| APR-12 | Approval verification: extract `kid` from JWS header, resolve DID, verify signature for each party | Spec §8.9 verification algorithm |
| TPL-01 | Templates define approval rendering, provided exclusively by `via` system | Spec §11.1 |
| TPL-02 | Templates carry JWS proof signed by `via` entity; client verifies against `via` DID | Spec §11.2, §11.3 |
| TPL-03 | Template fields: brand (name, logo, colors), display (title, fields with types), proof | Spec §11.2 format |
| TPL-04 | Field types: text, currency, date, date_range, list, image, number | Spec §11.4 field types table |
| TPL-05 | Security: HTTPS only, no redirects, IP validation (block RFC 1918/loopback/metadata), 64KB max, 5s timeout | Spec §11.5 security section |
| TPL-06 | Two-party approvals use fallback rendering (label + formatted JSON payload) | Spec §11.1 |
| API-03 | Approval endpoints: POST /v1/approvals, POST /v1/approvals/{id}/respond, GET /v1/approvals/{id}, GET /v1/approvals/{id}/status, GET /v1/approvals, DELETE /v1/approvals/{id} | Spec §13.3 approvals table |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-jose/go-jose/v4` | v4.1.3 (already in go.mod) | JWS Compact Serialization, detached payload signing and verification | RFC 7515/7797 implementation; already an indirect dep from go-dpop, promoted to direct |
| `github.com/gowebpki/jcs` | v1.0.1 (already in go.mod) | RFC 8785 JCS canonical JSON for signing | Already used in `crypto.CanonicalJSON()` |
| `github.com/oklog/ulid/v2` | v2.1.1 (already in go.mod) | Generate `apr_` + ULID IDs | Pattern established in all prior phases |
| `github.com/jackc/pgx/v5` | v5.7.5 (already in go.mod) | PostgreSQL store for approval documents | Pattern established in Phases 1+2 |
| `github.com/gofiber/fiber/v2` | v2.52.12 (already in go.mod) | HTTP handlers for /v1/approvals/* | Pattern established in Phases 1+2 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/http` (stdlib) | Go 1.25 | Template URL fetching (TPL-05 security constraints) | Template fetch in three-party flow |
| `net` (stdlib) | Go 1.25 | IP validation for SSRF prevention (RFC 1918, loopback, cloud metadata) | Template fetch DNS resolution check |
| `time` (stdlib) | Go 1.25 | ISO 8601 / RFC 3339 TTL and expiry enforcement | APR-10 max_approval_ttl |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-jose/v4 JWS | lestrrat-go/jwx/v3 | go-jose already in dep tree (via go-dpop); promoting existing dep is preferable |
| Custom JCS call per sign | Pre-serialized bytes cache | Cache adds complexity; payload is small enough that JCS per operation is fine |

**Installation — no new direct dependencies needed:**
```bash
# go-jose/v4 is already indirect; promote to direct with:
cd platform && go get github.com/go-jose/go-jose/v4
```

---

## Architecture Patterns

### Recommended Project Structure
```
platform/
├── internal/
│   ├── approvals/           # New package: approval domain model + signing
│   │   ├── approval.go      # Approval, Subject, Signature types; ApprovalID() generator
│   │   ├── signer.go        # JWS sign/verify (detached payload over JCS)
│   │   ├── lifecycle.go     # State machine transitions + TTL/max_approval_ttl
│   │   ├── template.go      # Template fetch, SSRF validation, JWS proof verify
│   │   └── *_test.go
│   ├── api/
│   │   └── approvals.go     # HTTP handlers: CreateApproval, RespondApproval, GetApproval,
│   │                        #   GetApprovalStatus, ListApprovals, RevokeApproval
│   └── store/
│       └── approvals.go     # SQL CRUD for approvals table
├── migrations/
│   └── 011_approvals.up.sql # approvals table
```

### Pattern 1: Approval ID Generation
**What:** `apr_` prefix + ULID lowercase, consistent with existing `sig_`, `msg_` pattern.
**When to use:** Every new approval document.
**Example:**
```go
// Source: crypto.NewEntityID() pattern in platform/internal/crypto/crypto.go
func NewApprovalID() string {
    entropy := ulid.Monotonic(rand.Reader, 0)
    id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
    return "apr_" + strings.ToLower(id.String())
}
```

### Pattern 2: JWS Detached Payload Signing (APR-05, APR-06)
**What:** Sign the JCS-canonical approval (excluding `signatures` field) using Ed25519 JWS Compact Serialization with detached payload (RFC 7797). The `..` in the compact token indicates the payload is detached.
**When to use:** Every time a party (`from`, `via`, or `to`) adds their signature.
**Example:**
```go
// Source: RFC 7515 §7.2, RFC 7797 §3; go-jose/v4 API
import "github.com/go-jose/go-jose/v4"

func SignApproval(approval *Approval, privateKey ed25519.PrivateKey, keyID string) (string, error) {
    // 1. Build a copy without signatures field, JCS-serialize it.
    withoutSigs := approvalWithoutSignatures(approval)
    payload, err := crypto.CanonicalJSON(withoutSigs)
    if err != nil {
        return "", fmt.Errorf("canonical JSON: %w", err)
    }

    // 2. Build signer with EdDSA alg + kid header.
    sig, err := jose.NewSigner(
        jose.SigningKey{Algorithm: jose.EdDSA, Key: privateKey},
        (&jose.SignerOptions{}).WithHeader("kid", keyID),
    )
    if err != nil {
        return "", fmt.Errorf("new signer: %w", err)
    }

    // 3. Sign with detached payload.
    jws, err := sig.Sign(payload)
    if err != nil {
        return "", fmt.Errorf("sign: %w", err)
    }

    // 4. Serialize as compact with detached payload: header..signature
    compact, err := jws.CompactSerialize()
    if err != nil {
        return "", fmt.Errorf("compact serialize: %w", err)
    }
    // Detach payload: replace base64(payload) with empty string → "header..sig"
    parts := strings.SplitN(compact, ".", 3)
    return parts[0] + ".." + parts[2], nil
}
```

### Pattern 3: Approval Verification (APR-12)
**What:** For each signature in an approval, parse JWS header, extract `kid`, resolve DID, locate verification method, verify signature.
**When to use:** `GET /v1/approvals/{id}` response includes verification proof; also internal integrity check before `via` co-signs.
**Example:**
```go
// Source: Spec §8.9
func VerifyApprovalSignature(approval *Approval, role string, jwsToken string, resolvedPubKey ed25519.PublicKey) error {
    // 1. Recompute canonical payload without signatures.
    withoutSigs := approvalWithoutSignatures(approval)
    payload, err := crypto.CanonicalJSON(withoutSigs)
    if err != nil {
        return fmt.Errorf("canonical JSON: %w", err)
    }

    // 2. Re-attach payload to compact JWS for parsing.
    parts := strings.SplitN(jwsToken, ".", 3)
    // parts[1] is "" for detached payload; re-attach base64url(payload).
    attached := parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]

    // 3. Parse and verify with go-jose.
    jws, err := jose.ParseSigned(attached, []jose.SignatureAlgorithm{jose.EdDSA})
    if err != nil {
        return fmt.Errorf("parse JWS: %w", err)
    }
    if _, err := jws.Verify(resolvedPubKey); err != nil {
        return fmt.Errorf("signature invalid: %w", err)
    }
    return nil
}
```

### Pattern 4: Store Interface + Handler Pattern
**What:** Define `ApprovalStore` interface in `api/api.go` (consistent with `EntityStore`, `MessageStore`); implement in `store/approvals.go`; inject into Handler.
**When to use:** All approval CRUD operations.
**Example:**
```go
// Add to api/api.go alongside existing store interfaces
type ApprovalStore interface {
    CreateApproval(ctx context.Context, a *models.Approval) error
    GetApproval(ctx context.Context, id string) (*models.Approval, error)
    UpdateApprovalState(ctx context.Context, id, state string) error
    ListApprovals(ctx context.Context, entityDID string, limit, offset int) ([]models.Approval, error)
    GetChildApprovals(ctx context.Context, parentID string) ([]models.Approval, error)
}
```

### Pattern 5: State Machine Enforcement
**What:** The server enforces valid state transitions per spec §8.3. Invalid transitions return HTTP 409.
**When to use:** Before any state mutation.
```
requested → approved       (via /respond with status=approved)
requested → declined       (via /respond with status=declined)
requested → expired        (background job: now > expires_at)
requested → rejected       (three-party only: via system refuses)
approved  → consumed       (one-time: no valid_until, after use)
approved  → revoked        (persistent: DELETE /v1/approvals/{id})
```
Final states (declined, expired, rejected, consumed, revoked) MUST reject further transitions.

### Pattern 6: max_approval_ttl Enforcement (APR-10)
**What:** On `POST /v1/approvals`, if `valid_until` is set, clamp it: `effective = min(requested_valid_until, now + max_approval_ttl)`. `max_approval_ttl` comes from server config (already in discovery doc).
**When to use:** Three-party flow when `via` co-signs; two-party flow when server receives the request.

### Pattern 7: Template SSRF Prevention (TPL-05)
**What:** Custom HTTP client with strict controls when fetching `template_url`.
**When to use:** Three-party flow when `via` resolves the template to inject.
```go
func safeTemplateFetch(ctx context.Context, rawURL string) ([]byte, error) {
    // 1. Reject non-HTTPS.
    // 2. Parse URL, disallow redirects (CheckRedirect: return ErrUseLastResponse).
    // 3. Custom dialer: resolve DNS, check each IP against blocklist.
    //    Block: RFC 1918 (10/8, 172.16/12, 192.168/16), loopback (127/8), link-local (169.254/16).
    //    Block: cloud metadata (169.254.169.254).
    // 4. Enforce 5s total timeout, 64KB response limit.
    // 5. Return raw bytes for JWS proof verification.
}
```

### Anti-Patterns to Avoid
- **Signing the full approval document including `signatures` field:** The spec (§8.8) is explicit: sign the JCS-serialized approval *excluding* the `signatures` field. Including it makes signatures depend on each other, breaking independent verifiability.
- **Using JSON.Marshal instead of JCS for the signing payload:** `json.Marshal` does not sort keys deterministically across Go versions. Always use `gowebpki/jcs` (already wired as `crypto.CanonicalJSON`).
- **Storing JWS as JSON serialization:** The spec requires JWS Compact Serialization. Compact produces the `header..sig` detached form. JSON serialization is a different format.
- **Template fetch without SSRF controls:** `template_url` is attacker-controlled. Without IP validation after DNS resolution, it enables Server-Side Request Forgery to internal services.
- **Storing approval payload as opaque JSONB without indexing `state`:** Queries like "find all active approvals for entity X" require `state` to be an indexed column, not buried in JSONB.
- **Allowing state transitions from final states:** Once `declined`, `expired`, `rejected`, `consumed`, or `revoked`, an approval is permanently terminal. Permitting re-transitions breaks audit integrity.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWS Compact Serialization + detached payload | Custom base64url concat | `go-jose/v4` `Signer` + `ParseSigned` | Correct alg negotiation, header serialization, signature encoding |
| JCS canonical JSON | Custom key-sort marshal | `gowebpki/jcs` (`crypto.CanonicalJSON`) | Already proven in codebase; RFC 8785 float serialization is subtle |
| Ed25519 key operations | Custom signature scheme | `crypto/ed25519` stdlib | Used throughout; private keys already stored in entity records |
| ULID generation | UUID or random hex | `oklog/ulid/v2` | Already the project ID standard |
| ISO 8601 duration parsing (`P90D`) | Custom parser | `time.ParseDuration` + period prefix handling OR hardcode to seconds | `max_approval_ttl` is published as ISO 8601 duration string |

**Key insight:** Every cryptographic primitive and ID generator this phase needs is already a direct or indirect dependency. The primary work is wiring them together correctly per the spec's exact requirements.

---

## Common Pitfalls

### Pitfall 1: Detached Payload Re-attachment for Verification
**What goes wrong:** `go-jose`'s `ParseSigned` expects the payload to be present in the compact token. A detached-payload JWS (`header..sig`) will fail to parse or verify unless the payload is re-attached before calling `ParseSigned`.
**Why it happens:** RFC 7797 detached payload is a transport convention — the payload bytes are transmitted separately. Verifiers must re-attach before invoking JWS verification.
**How to avoid:** Before calling `go-jose`'s `ParseSigned`, reconstruct the compact token: `parts[0] + "." + base64url(recomputedPayload) + "." + parts[2]`.
**Warning signs:** `go-jose` error "square/go-jose: unexpected end of JSON input" or signature verification failures that look like data corruption.

### Pitfall 2: Approval Document Mutation Before All Signatures
**What goes wrong:** The `from` signature is computed over the approval *as of that moment*. If the `via` system mutates any field before co-signing (e.g., normalizes timestamps, trims whitespace), the `from` signature becomes invalid.
**Why it happens:** Each signature is over the JCS of the approval excluding `signatures`. Any field mutation invalidates prior signatures.
**How to avoid:** `via` MUST NOT modify any field of the approval document it received. It only appends its own JWS to `signatures.via`. The `template_url` field, if any, MUST be included in the original `from`-signed document (attacker could swap it otherwise — this is actually a security requirement, not just a pitfall).
**Warning signs:** `from` signature fails to verify after `via` co-signing.

### Pitfall 3: Cross-Test Vectors for JWS Detached Payload
**What goes wrong:** JWS detached payload is implemented in multiple ways across ecosystems. A Go-produced token may fail to verify in a JavaScript client if header serialization or base64url encoding differs.
**Why it happens:** RFC 7797 leaves some encoding details underspecified (e.g., whether the payload is raw bytes or base64url before detachment).
**How to avoid:** Produce and verify test vectors from day one. The STATE.md already flags this: "JWS detached payload `crit` header handling needs cross-platform test vectors from day one."
**Warning signs:** Intra-Go tests pass but cross-platform verification fails.

### Pitfall 4: SSRF via DNS Rebinding in Template Fetch
**What goes wrong:** Checking the IP before making the HTTP request allows DNS rebinding — the DNS record changes between the check and the request.
**Why it happens:** Two separate operations (DNS resolve + HTTP connect) with an attacker-controlled DNS TTL.
**How to avoid:** Use a custom `net.Dialer` that resolves the IP once and validates it, then connects to that resolved IP directly. Do not re-resolve during the HTTP request.
**Warning signs:** Template fetch to cloud metadata endpoint `169.254.169.254` succeeds in tests.

### Pitfall 5: Parent Approval Invalidation Scope
**What goes wrong:** Revoking a parent approval must cascade to invalidate all children, but naive queries only check direct children.
**Why it happens:** Approval chains can be N levels deep via the `parent` field.
**How to avoid:** On revocation, use a recursive CTE (`WITH RECURSIVE`) to find all descendants and mark them `revoked`. Alternatively, on each use, walk the parent chain and verify no ancestor is revoked.
**Warning signs:** Child approval is honored after its grandparent was revoked.

### Pitfall 6: ISO 8601 Duration Parsing for max_approval_ttl
**What goes wrong:** The discovery document publishes `max_approval_ttl` as `"P90D"` (ISO 8601 period), but Go's `time.ParseDuration` only handles Go format (`"2160h"`).
**Why it happens:** Go stdlib does not implement ISO 8601 duration parsing.
**How to avoid:** Write a small ISO 8601 period parser for the common `PnD`, `PnM`, `PnY` cases, or convert the config value to seconds at startup. The config already stores `max_approval_ttl` — just parse it once at load time.
**Warning signs:** Panic or zero TTL when enforcing persistent approval expiry.

---

## Code Examples

### Approval Data Model
```go
// Source: Spec §8.5, §8.6, §8.7 (platform/internal/models/models.go extension)
type Approval struct {
    AtapApproval string          `json:"atap_approval"` // always "1"
    ID           string          `json:"id"`            // "apr_" + ULID
    CreatedAt    time.Time       `json:"created_at"`
    ValidUntil   *time.Time      `json:"valid_until,omitempty"` // nil = one-time

    From   string `json:"from"`             // requester DID
    To     string `json:"to"`               // approver DID
    Via    string `json:"via,omitempty"`     // mediating system DID
    Parent string `json:"parent,omitempty"` // parent approval ID

    Subject     ApprovalSubject       `json:"subject"`
    TemplateURL string                `json:"template_url,omitempty"`
    Signatures  map[string]string     `json:"signatures"` // role → JWS

    // Server-side only (not in signed document)
    State       string    `json:"-"`
    UpdatedAt   time.Time `json:"-"`
}

type ApprovalSubject struct {
    Type       string          `json:"type"`       // reverse-domain
    Label      string          `json:"label"`
    Reversible bool            `json:"reversible"`
    Payload    json.RawMessage `json:"payload"`    // system-specific JSON
}
```

### Approval Response Document
```go
// Source: Spec §8.11
type ApprovalResponse struct {
    AtapApprovalResponse string    `json:"atap_approval_response"` // "1"
    ApprovalID           string    `json:"approval_id"`
    Status               string    `json:"status"`       // "approved" | "declined"
    RespondedAt          time.Time `json:"responded_at"`
    Signature            string    `json:"signature"`    // JWS from `to` entity
}
```

### Database Schema (migration 011)
```sql
-- Source: Spec §8.3 lifecycle, §8.6 fields
CREATE TABLE approvals (
    id           TEXT PRIMARY KEY,          -- "apr_" + ULID
    state        TEXT NOT NULL DEFAULT 'requested',
                                            -- requested|approved|declined|expired
                                            -- rejected|consumed|revoked
    from_did     TEXT NOT NULL,
    to_did       TEXT NOT NULL,
    via_did      TEXT,                      -- NULL for two-party
    parent_id    TEXT REFERENCES approvals(id),
    document     JSONB NOT NULL,            -- full approval JSON (for retrieval)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until  TIMESTAMPTZ,               -- NULL = one-time
    responded_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approvals_from ON approvals(from_did, state);
CREATE INDEX idx_approvals_to   ON approvals(to_did, state);
CREATE INDEX idx_approvals_via  ON approvals(via_did) WHERE via_did IS NOT NULL;
CREATE INDEX idx_approvals_parent ON approvals(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_approvals_expires ON approvals(valid_until)
    WHERE state = 'approved' AND valid_until IS NOT NULL;
```

### Template Model
```go
// Source: Spec §11.2
type Template struct {
    AtapTemplate string         `json:"atap_template"` // "1"
    SubjectType  string         `json:"subject_type"`
    Brand        TemplateBrand  `json:"brand"`
    Display      TemplateDisplay `json:"display"`
    Proof        TemplateProof  `json:"proof"`
}

type TemplateBrand struct {
    Name    string          `json:"name"`
    LogoURL string          `json:"logo_url"`
    Colors  TemplateColors  `json:"colors"`
}

type TemplateColors struct {
    Primary    string `json:"primary"`
    Accent     string `json:"accent"`
    Background string `json:"background"`
}

type TemplateDisplay struct {
    Title  string          `json:"title"`
    Fields []TemplateField `json:"fields"`
}

type TemplateField struct {
    Key   string `json:"key"`
    Label string `json:"label"`
    Type  string `json:"type"` // text|currency|date|date_range|list|image|number
}

type TemplateProof struct {
    KID string `json:"kid"` // did:web:...#key-id
    Alg string `json:"alg"` // "EdDSA"
    Sig string `json:"sig"` // base64url JWS
}
```

### DIDComm Dispatch for Approval Flow
```go
// Source: didcomm/message.go TypeApproval* constants; Spec §8.4
// Three-party step 1: from → via
msg := didcomm.NewMessage(didcomm.TypeApprovalRequest, fromDID, []string{viaDID}, map[string]any{
    "approval_id": approval.ID,
    "approval":    approval, // full document as body or attachment
})
// Three-party step 2: via → to (after co-signing)
msg := didcomm.NewMessage(didcomm.TypeApprovalRequest, viaDID, []string{toDID}, map[string]any{
    "approval_id": approval.ID,
    "approval":    approval,
})
// Step 3: to → from (and via if present)
msg := didcomm.NewMessage(didcomm.TypeApprovalResponse, toDID, recipients, map[string]any{
    "approval_id": response.ApprovalID,
    "status":      response.Status,
    "signature":   response.Signature,
})
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OAuth tokens for consent | JWS multi-sig approval document | ATAP v1 design | Offline verifiability without callback |
| Single-party authorization | Multi-party independent signing | ATAP v1 design | Non-repudiable consent from all parties |
| Mutable approval state | Immutable document + server state column | Phase 3 design | Document integrity preserved; state is server bookkeeping |

**Deprecated/outdated:**
- Custom `atap_` bearer tokens: Replaced by OAuth 2.1 + DPoP (Phase 1 complete)
- Old signal/channel pipeline: Stripped in Phase 1 (INF-01 complete)
- Custom Ed25519 request signing: Replaced by DPoP (Phase 1 complete)

---

## Open Questions

1. **Server as `via` vs. external `via`**
   - What we know: The spec says the ATAP server acts as both DIDComm mediator (untrusted relay) and ATAP system participant (trusted co-signer, MSG-03). In the three-party flow, `via` is a DID of the mediating system.
   - What's unclear: In Phase 3, does the server act as `via` for hosted entities, or is `via` always an external system? The spec says the server "is a trusted co-signer" for three-party, implying the hosted server can be `via`. However, Phase 4 may require more context (credentials, trust levels) before the server performs meaningful `via` validation.
   - Recommendation: Implement the server as a valid `via` participant — it has its own DID (`did:web:{domain}:server:platform`) and Ed25519 key. The `via` validation logic can be minimal in Phase 3 (accept all valid payloads); policy enforcement (trust levels, credential checks) is Phase 4 work.

2. **Approval document in DIDComm body vs. attachment**
   - What we know: The spec shows the approval document as a standalone JSON object. DIDComm messages have both a `body` map and `attachments`.
   - What's unclear: Whether the full approval document goes in `body` or as a `data.json` attachment.
   - Recommendation: Use `attachments` with `data.json` for the full approval document (consistent with DIDComm v2.1 conventions for structured data), with `body` containing just `approval_id` and `status` for lightweight references.

3. **Expiry background job**
   - What we know: The spec says unanswered requests "SHOULD be treated as expired after a system-defined timeout" (§8.16). The DIDComm message queue already has an expiry cleanup job pattern (`CleanupExpiredMessages`).
   - What's unclear: Whether the expiry job runs in the same goroutine pattern as message cleanup or as a separate cron.
   - Recommendation: Follow the established pattern from `store/messages.go` — a `CleanupExpiredApprovals` store method called on a timer from `cmd/server/main.go`.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testcontainers-go` for Postgres/Redis |
| Config file | none — tests use `go test ./...` with testcontainers |
| Quick run command | `cd platform && go test ./internal/approvals/... -run TestSign -v` |
| Full suite command | `cd platform && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| APR-05 | JWS detached payload sign + verify round-trip | unit | `go test ./internal/approvals/... -run TestSignVerify -v` | ❌ Wave 0 |
| APR-06 | JCS canonical payload excludes signatures field | unit | `go test ./internal/approvals/... -run TestCanonicalPayload -v` | ❌ Wave 0 |
| APR-01 | Two-party approval end-to-end: sign → respond → consumed | integration | `go test ./internal/api/... -run TestTwoPartyApproval -v` | ❌ Wave 0 |
| APR-02 | Three-party approval end-to-end: sign → co-sign → respond | integration | `go test ./internal/api/... -run TestThreePartyApproval -v` | ❌ Wave 0 |
| APR-07 | State machine: valid transitions accepted, invalid rejected | unit | `go test ./internal/approvals/... -run TestLifecycle -v` | ❌ Wave 0 |
| APR-09 | One-time approval transitions to consumed after use | integration | `go test ./internal/api/... -run TestConsumedOnUse -v` | ❌ Wave 0 |
| APR-10 | max_approval_ttl clamps valid_until | unit | `go test ./internal/approvals/... -run TestMaxTTL -v` | ❌ Wave 0 |
| APR-11 | Revoking parent invalidates children (recursive) | integration | `go test ./internal/api/... -run TestParentRevoke -v` | ❌ Wave 0 |
| APR-12 | Signature verification: correct kid → DID match | unit | `go test ./internal/approvals/... -run TestVerifyDIDMatch -v` | ❌ Wave 0 |
| TPL-02 | Template JWS proof verification | unit | `go test ./internal/approvals/... -run TestTemplateVerify -v` | ❌ Wave 0 |
| TPL-05 | SSRF: RFC 1918 + loopback + metadata IP blocked | unit | `go test ./internal/approvals/... -run TestSSRFBlock -v` | ❌ Wave 0 |
| API-03 | All 6 approval endpoints respond correctly | integration | `go test ./internal/api/... -run TestApprovalAPI -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./internal/approvals/... -v`
- **Per wave merge:** `cd platform && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `platform/internal/approvals/` package — does not exist yet
- [ ] `platform/internal/approvals/approval_test.go` — covers APR-03, APR-04, APR-05, APR-06
- [ ] `platform/internal/approvals/lifecycle_test.go` — covers APR-07, APR-09, APR-10, APR-11
- [ ] `platform/internal/approvals/template_test.go` — covers TPL-02, TPL-05
- [ ] `platform/internal/api/approvals_test.go` — covers APR-01, APR-02, APR-12, API-03
- [ ] `platform/migrations/011_approvals.up.sql` — required before store tests run

---

## Sources

### Primary (HIGH confidence)
- `spec/ATAP-SPEC-v1.0-rc1.md` §8 (Approvals) — full approval model, fields, lifecycle, signing algorithm, verification, rejection, response, one-time/persistent, chaining, revocation
- `spec/ATAP-SPEC-v1.0-rc1.md` §11 (Templates) — template format, verification, field types, SSRF security requirements
- `spec/ATAP-SPEC-v1.0-rc1.md` §13 (API) — exact endpoint signatures for API-03
- `platform/go.mod` — confirmed existing dependencies (go-jose/v4, gowebpki/jcs, ulid/v2, pgx/v5, fiber/v2)
- `platform/internal/didcomm/message.go` — confirmed TypeApproval* constants already defined (MSG-05)
- `platform/internal/crypto/crypto.go` — confirmed CanonicalJSON (JCS) already wired
- `platform/internal/api/api.go` — confirmed handler/store interface pattern to follow

### Secondary (MEDIUM confidence)
- RFC 7515 (JWS) + RFC 7797 (JWS Detached Payload) — go-jose/v4 implements both; detached payload re-attachment pattern is standard practice
- RFC 8785 (JCS) — gowebpki/jcs implements; `crypto.CanonicalJSON` already in use in Phase 2

### Tertiary (LOW confidence)
- None — all critical claims are sourced from the project spec or codebase directly.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod; patterns established in Phases 1+2
- Architecture: HIGH — spec is fully normative; data model and API are specified precisely
- Pitfalls: HIGH — derived from spec requirements and known cryptographic implementation pitfalls
- Template SSRF: HIGH — spec §11.5 is explicit; SSRF prevention patterns are well-understood

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (stable — spec is frozen at RC1, all libraries are stable releases)
