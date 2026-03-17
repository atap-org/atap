# Phase 03: Approval Engine — Research

**Researched:** 2026-03-16
**Domain:** ATAP approval engine rework — DIDComm-only transport, revocation API, Adaptive Cards templates
**Confidence:** HIGH (all findings from spec v1.0-rc1 and direct codebase inspection)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Server Role**
- ATAP server is pure DIDComm transport (mediator) for approvals — it routes messages but does not inspect, store, or validate approval content
- Server stores only: entity records, credentials, and revocation lists
- All approval CRUD endpoints (POST/GET/DELETE /v1/approvals) must be removed
- Server does NOT co-sign approvals — it is never `via`

**Via Semantics**
- `via` is an external machine entity (e.g., online shop, travel agent, bank) — NOT the ATAP server
- The `via` system validates business rules, co-signs the approval, and provides the branded Adaptive Card template
- Flow: agent (`from`) → via online shop (`via`) validates + co-signs → human (`to`) approves/declines
- The `via` knows the payload (it's their transaction), provides the template (their brand), and co-signs (vouching for business context)

**Approval / Standing Approval Terminology**
- **Approval** (was "one-time"): `valid_until` absent/null. Default TTL 60 minutes (RECOMMENDED, system-configurable). Transitions to `consumed` after use or `expired` after TTL.
- **Standing Approval** (was "persistent"): `valid_until` set as ISO 8601 datetime. Valid for repeated use until expiry, subject to `max_approval_ttl`.

**Templates**
- Templates use Microsoft Adaptive Cards format — not custom JSON
- Template provided exclusively by `via` system, signed with JWS proof
- Client fetches template directly from `via`'s `template_url` — server never touches templates
- Mandatory JWS verification against `via` DID — if verification fails, fall back to generic rendering
- Fall back to generic rendering if template URL unreachable; notify `via` about template failure via DIDComm signal
- Two-party approvals use fallback rendering: `subject.label` as title + `subject.payload` as formatted JSON
- Fallback rendering needs a separate spec for what data is shown vs hidden — deferred
- Full spec security rules apply on mobile client: HTTPS only, 64KB max, 5s timeout, IP blocking, `$schema` NOT a fetch target
- Adaptive Card `Action.Submit` and `Action.OpenUrl` MUST be disabled

**Revocation**
- Revocation via DIDComm message (`approval/1.0/revoke`) sent by approver to their ATAP server
- Server stores revocation entries (negative attestation model) — not approvals
- Server forwards revocation to `via` system via DIDComm so `via` can cache locally
- Self-cleaning revocation lists: each entry carries `expires_at` (from `valid_until` or `revoked_at` + 60min). Servers SHOULD remove expired entries.
- Revocation check: `via` checks local cache first (fast path), then queries approver's ATAP server (authoritative)
- Revocation list API: `GET /v1/revocations?entity={approver-did}` returns active revocations with `revoked_at` and `expires_at`

**API Changes**
- Removed: POST /v1/approvals, POST /v1/approvals/{id}/respond, GET /v1/approvals/{id}, GET /v1/approvals/{id}/status, GET /v1/approvals, DELETE /v1/approvals/{id}
- Added: POST /v1/revocations (submit signed revocation, `atap:revoke` scope), GET /v1/revocations (query by entity DID, public)
- OAuth scope changed: `atap:approve` -> `atap:revoke`

### Claude's Discretion
- How to refactor existing approval code (keep reusable parts like JWS signing, JCS canonicalization, lifecycle state machine)
- Database migration strategy (remove approvals table, add revocations table)
- How to structure the approval model code now that server doesn't store approvals (library vs handler)

### Deferred Ideas (OUT OF SCOPE)
- Agent-provided templates for two-party approvals — future phase, no priority
- Fallback rendering specification (what data is shown/hidden in detail view) — separate spec work
- Template failure notification signal to `via` — implement when mobile client is built (Phase 4)
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| APR-01 | Two-party approvals: `from` signs, sends to `to` via DIDComm who approves/declines (2 signatures). Server is transport only. | Server is pure DIDComm relay; no storage. Existing `approval/signer.go` and `lifecycle.go` remain valid. DIDComm dispatch via `messageStore.QueueMessage` already works. |
| APR-02 | Three-party approvals: `from` signs → `via` (external machine) validates + co-signs → `to` approves/declines (3 signatures). `via` is NOT the ATAP server. | `via` is an external DID. ATAP server only routes `approval/1.0/request` to `via`'s DIDComm endpoint. Server never co-signs. |
| APR-03 | Approval format with `atap_approval: "1"`, `apr_` + ULID IDs. Approvals are portable documents stored by parties, not by the server. | Approval model struct is correct per spec §8.5. Must remove server-side storage. Model stays as library type only. |
| APR-04 | Subject contains `type` (reverse-domain), `label`, `reversible` boolean, `payload` (system-specific JSON). | Already implemented correctly in `models.ApprovalSubject`. No change needed. |
| APR-05 | JWS Compact Serialization with detached payload (RFC 7515 + RFC 7797) for each signature. | Already implemented correctly in `approval/signer.go`. No change needed. |
| APR-06 | Signed payload is UTF-8 of JCS-serialized (RFC 8785) approval excluding `signatures` field. | Already implemented in `approval/signer.go` via `approvalWithoutSignatures` + `crypto.CanonicalJSON`. No change needed. |
| APR-07 | Full approval lifecycle: requested → approved/declined/expired/rejected → consumed/revoked. Approval TTL 60min. Standing Approval via `valid_until`. | Lifecycle state machine in `approval/lifecycle.go` is correct. Server does NOT enforce TTL — `via` system does. State constants need no change. |
| APR-08 | System rejection by `via` (external machine) with `approval/1.0/rejected` message type. | `TypeApprovalRejected` DIDComm message type already defined. Server only passes `rejected` messages through DIDComm. No server-side rejection logic. |
| APR-09 | Approvals (`valid_until` absent) have default TTL of 60 minutes. Tracked by `via` system, not ATAP server. | Server does not track TTL. `ClampValidUntil` in lifecycle.go can stay as library function but server does not call it for approvals it doesn't store. |
| APR-10 | Standing Approvals (`valid_until` set) valid for repeated use, subject to receiver-side `max_approval_ttl`. | `max_approval_ttl` published in discovery. Enforcement is done by `via` system, not server. |
| APR-11 | Chained approvals via `parent` field; revoking parent invalidates children. | Server no longer stores approvals so parent chain enforcement moves to `via`. Revocation API only records the revoked ID. |
| APR-12 | Approval verification: extract `kid` from JWS header, resolve DID, verify signature. | Already implemented in `approval/signer.go:VerifyApprovalSignature`. No change needed. |
| APR-13 | Standing Approval enforcement by `via` system: verify sigs, expiry, revocation list, DID liveness, principal claim, parent validity, payload rules. | Server-side: only implement revocation list API (REV-04). All other checks are `via` system responsibility. |
| APR-14 | Server does not store approvals — stores only entity records, credentials, and revocation lists. | Requires: drop `approvals` table in migration 012, remove `store/approvals.go` and `api/approvals.go`, remove `ApprovalStore` interface from `api/api.go`. |
| AUTH-05 | Token scopes: `atap:inbox`, `atap:send`, `atap:revoke`, `atap:manage`. | Change `atap:approve` to `atap:revoke` in `RequireScope` call in `SetupRoutes`. OAuth token scope list must include `atap:revoke`. |
| MSG-03 | Server is DIDComm mediator only — `via` role belongs to external systems (machines), not the ATAP server. | Remove all server co-signing code from `api/approvals.go`. Server only routes DIDComm messages via existing `HandleDIDComm`. |
| TPL-01 | Templates use Microsoft Adaptive Cards format, provided exclusively by `via` system. | Rework `models.Template` struct: replace `SubjectType`/`Brand`/`Display`/`TemplateField` with Adaptive Card `card` field (raw JSON) + `proof`. |
| TPL-02 | Templates carry JWS proof signed by `via`; client fetches from `template_url` and verifies against `via` DID. | Template fetch + JWS verification stays in `approval/template.go`. Server-side: keep `FetchTemplate` and `VerifyTemplateProof` as library functions for future client use. |
| TPL-03 | Template wraps Adaptive Card in `atap_template` envelope with `card` (standard Adaptive Card) and `proof` (JWS). | Update `models.Template`: replace current struct with `AtapTemplate`, `Card json.RawMessage`, `Proof TemplateProof`. |
| TPL-04 | Data binding via Adaptive Card Templating syntax with context: subject, payload, brand, from, to, via. | Client-side concern. Server-side: template model must preserve `card` as `json.RawMessage` to pass through untouched. |
| TPL-05 | Security: HTTPS only, no redirects, IP validation, 64KB max, 5s timeout. Adaptive Card Action.Submit and Action.OpenUrl MUST be disabled. `$schema` NOT a fetch target. | SSRF logic in `approval/template.go:ssrfSafeTransport` and `IsBlockedIP` is correct and reusable. |
| TPL-06 | Two-party approvals use fallback rendering. | Server does not render templates. Fallback rendering is client-side only. |
| REV-01 | Revocation via DIDComm message (`approval/1.0/revoke`) sent by approver to ATAP server. Server forwards to `via` via DIDComm. | New: `HandleDIDComm` must handle `TypeApprovalRevoke` message type — store in revocations table, dispatch forward to `via`. |
| REV-02 | Server stores revoked approval IDs in revocation list indexed by approver DID (negative attestation). | New: `revocations` table with `approval_id`, `approver_did`, `revoked_at`, `expires_at`. New `store/revocations.go`. |
| REV-03 | Self-cleaning: each entry carries `expires_at`. Servers SHOULD remove expired entries. | `CleanupExpiredRevocations` function in store. Can run on a background goroutine (same pattern as `CleanupExpiredMessages`). |
| REV-04 | GET /v1/revocations?entity={approver-did} — public, returns active revoked approval IDs. | New public endpoint. No auth required. Returns `{ entity, revocations: [{approval_id, revoked_at, expires_at}], checked_at }`. |
| REV-05 | `via` checks local cache first, then queries approver's ATAP server. | Not a server-side requirement — `via` is an external system. ATAP server only provides GET /v1/revocations (REV-04). |
| API-03 | Revocation endpoints: POST /v1/revocations (submit signed revocation), GET /v1/revocations (query). Server does NOT expose approval CRUD. | New revocation API handler. Remove all 6 approval endpoints from `SetupRoutes`. |
</phase_requirements>

---

## Summary

Phase 3 is a significant **rework** of the previously completed approval engine. The spec v1.0-rc1 inverted the server's role: it is now a pure DIDComm mediator with no approval storage, not a storage engine that also routes messages. The `via` party is an external machine (e.g., online shop), not the ATAP server itself.

The rework has three distinct concerns: (1) **delete** the server-side approval storage and CRUD API (6 endpoints, 1 table, 1 store file), (2) **add** a new revocation list API (2 endpoints, 1 table, 1 store file, DIDComm handler extension), and (3) **update** the template model to match the Adaptive Cards spec format. A large portion of the existing codebase is still correct and reusable: the JWS signer, the lifecycle state machine, the SSRF-safe template fetcher, the DIDComm message types, and the dispatch infrastructure.

The key risk is the DIDComm handler extension for REV-01: the existing `HandleDIDComm` endpoint needs to recognize `approval/1.0/revoke` messages, parse the `approval_id` from the body, write a revocation entry, and forward a copy to the `via` entity — all without storing the full approval document. The DIDComm handler currently processes encrypted JWE envelopes and routes plaintext bodies; it must now extract the approval_id from the decrypted body and act on it.

**Primary recommendation:** Two plans. Plan 1: delete approval CRUD (endpoints, store, interface, migration) and add revocation table + REST API + scope change. Plan 2: rework template model to Adaptive Cards format and extend DIDComm handler for revoke message type.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-jose/go-jose/v4` | v4 (existing) | JWS signing/verification with EdDSA | Already used for approval signing and DIDComm envelope; consistent with existing code |
| `github.com/gofiber/fiber/v2` | v2 (existing) | HTTP handlers for revocation REST API | Established pattern throughout codebase |
| `github.com/jackc/pgx/v5` | v5 (existing) | PostgreSQL for revocations table | Consistent with all other store files |
| `encoding/json` | stdlib | `json.RawMessage` for Adaptive Card passthrough | No external dep needed — cards are opaque JSON |

### No New Dependencies Required
This phase requires no new Go dependencies. All functionality is achieved by reworking existing code. The `approval/` package's signing, lifecycle, and template fetch code needs structural changes but no new imports.

---

## Architecture Patterns

### Recommended Project Structure After Rework

```
platform/internal/
  api/
    api.go              -- remove ApprovalStore interface and field, remove approval routes, add revocation routes
    approvals.go        -- DELETE this file entirely
    revocations.go      -- NEW: POST /v1/revocations, GET /v1/revocations handlers
    approvals_test.go   -- DELETE this file (tests deleted handler)
    didcomm_handler.go  -- EXTEND: handle TypeApprovalRevoke message type
  approval/
    signer.go           -- KEEP as-is (SignApproval, VerifyApprovalSignature)
    lifecycle.go        -- KEEP as-is (ValidateTransition, IsTerminalState, ClampValidUntil)
    template.go         -- KEEP SSRF logic; remove SignTemplateProof (server no longer signs templates)
    template_test.go    -- KEEP existing SSRF tests; add model shape tests
  models/
    models.go           -- UPDATE Template struct for Adaptive Cards; ADD Revocation struct; keep Approval struct
  store/
    approvals.go        -- DELETE this file entirely
    approvals_test.go   -- DELETE this file
    revocations.go      -- NEW: CreateRevocation, ListRevocations, CleanupExpiredRevocations
    revocations_test.go -- NEW: unit tests for revocation store
    store.go            -- remove approvals migration references if any
  migrations/
    012_revocations.up.sql   -- NEW: drop approvals table, create revocations table
    012_revocations.down.sql -- NEW: reverse migration
```

### Pattern 1: Revocation Record Model

```go
// Source: spec §8.15
// Add to platform/internal/models/models.go

type Revocation struct {
    ID          string    `json:"id"`           // "rev_" + ULID
    ApprovalID  string    `json:"approval_id"`  // the revoked approval ID ("apr_" + ULID)
    ApproverDID string    `json:"approver_did"` // indexed for GET /v1/revocations?entity=
    RevokedAt   time.Time `json:"revoked_at"`
    ExpiresAt   time.Time `json:"expires_at"`   // = valid_until of original, or revoked_at + 60min
}
```

### Pattern 2: Revocation API Response

```go
// Source: spec §8.15 Revocation List API
// GET /v1/revocations?entity={approver-did}
// Response shape:
{
    "entity":      "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
    "revocations": [
        {
            "approval_id": "apr_4d7e9f1a",
            "revoked_at":  "2026-06-01T10:00:00Z",
            "expires_at":  "2026-09-12T00:00:00Z"
        }
    ],
    "checked_at": "2026-08-01T12:00:00Z"
}
```

### Pattern 3: DIDComm Revoke Handler Extension

```go
// Source: spec §8.15 + existing didcomm_handler.go dispatch pattern
// Extend HandleDIDComm's message type switch:
case didcomm.TypeApprovalRevoke:
    approvalID, _ := msg.Body["approval_id"].(string)
    viaDID, _     := msg.Body["via"].(string)
    validUntilStr, _ := msg.Body["valid_until"].(string) // may be absent
    // 1. Compute expires_at: if valid_until present -> parse it, else -> time.Now() + 60min
    // 2. h.revocationStore.CreateRevocation(ctx, &models.Revocation{...})
    // 3. If viaDID != "": forward TypeApprovalRevoke to viaDID via messageStore.QueueMessage
```

### Pattern 4: Adaptive Cards Template Model

```go
// Source: spec §11.3
// Update platform/internal/models/models.go

// Template is the ATAP template envelope wrapping a standard Adaptive Card.
type Template struct {
    AtapTemplate string          `json:"atap_template"` // always "1"
    Card         json.RawMessage `json:"card"`          // standard Adaptive Card JSON (opaque)
    Proof        TemplateProof   `json:"proof"`
}

// TemplateProof is unchanged:
type TemplateProof struct {
    KID string `json:"kid"` // did:web:...#key-id
    Alg string `json:"alg"` // "EdDSA"
    Sig string `json:"sig"` // base64url detached JWS compact
}

// Remove: TemplateBrand, TemplateColors, TemplateDisplay, TemplateField
```

### Pattern 5: Revocation Store Interface

```go
// Source: pattern from existing store interfaces in api/api.go
// Replace ApprovalStore interface with RevocationStore:
type RevocationStore interface {
    CreateRevocation(ctx context.Context, r *models.Revocation) error
    ListRevocations(ctx context.Context, approverDID string) ([]models.Revocation, error)
    CleanupExpiredRevocations(ctx context.Context) (int64, error)
}
```

### Anti-Patterns to Avoid

- **Server storing approvals as fallback:** The spec is explicit — server does not store approvals. Do not add any approval caching, temp storage, or "pending approval" table.
- **Server acting as `via` for any flow:** Remove the `if req.Via != serverDID { reject }` check. The server is never `via`.
- **Parsing Adaptive Card internals on the server:** The `card` field is an opaque JSON blob. Use `json.RawMessage`. Do not define structs for Adaptive Card elements.
- **Approval TTL enforcement on the server:** Server does not track Approval TTL. That is the `via` system's responsibility.
- **Keeping `serverDID()` and `serverKeyID()` helpers in `api/approvals.go`:** These functions exist only to support the old co-signing logic. Remove them when the file is deleted.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SSRF prevention for template URLs | Custom IP blocker | `approval/template.go:IsBlockedIP` + `ssrfSafeTransport` | Already correct and tested; handles DNS rebinding |
| JWS detached payload signing | Custom base64/signing | `approval/signer.go:SignApproval` + `VerifyApprovalSignature` | Handles re-attachment, kid validation, go-jose v4 quirks |
| JCS canonical JSON | Custom serializer | `crypto.CanonicalJSON` | RFC 8785 compliance; already tested |
| DIDComm message dispatch | Direct HTTP calls to `via` | `messageStore.QueueMessage` | Handles offline delivery, message IDs, persistence |
| ULID generation for revocation IDs | Custom ID generator | `ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)` | Already used throughout codebase |
| RFC 7807 errors | Custom error format | `problem(c, status, errType, title, detail)` helper | Already used by all handlers |

**Key insight:** The approval engine's cryptographic and transport primitives are all correct and tested. The rework is about changing what the server stores and which HTTP endpoints it exposes — not about changing the cryptographic logic.

---

## Common Pitfalls

### Pitfall 1: ApprovalStore Interface Removal Breaks Compilation
**What goes wrong:** `api/api.go` defines `ApprovalStore` interface and `Handler` has `approvalStore` field. Deleting `api/approvals.go` without updating `api/api.go` causes compile errors.
**Why it happens:** `NewHandler` takes `as ApprovalStore` as a parameter; `cmd/server/main.go` passes `store.Store` implementing it.
**How to avoid:** Remove `ApprovalStore` interface from `api/api.go`, remove `approvalStore` field from `Handler`, remove parameter from `NewHandler`. Also update `cmd/server/main.go` call site.
**Warning signs:** Compile error on `h.approvalStore` references after deleting `api/approvals.go`.

### Pitfall 2: Approval Route Registrations Still in SetupRoutes
**What goes wrong:** `SetupRoutes` still registers `/v1/approvals` routes after handlers are removed.
**Why it happens:** `api/api.go:SetupRoutes` has 5 approval route registrations and 1 public status route that reference deleted handler methods.
**How to avoid:** Remove all 6 approval route lines from `SetupRoutes`. Add 2 revocation route lines (POST authenticated, GET public).
**Warning signs:** Compile errors referencing `h.CreateApproval`, `h.RespondApproval`, etc. after file deletion.

### Pitfall 3: Scope String `atap:approve` Not Fully Replaced
**What goes wrong:** POST /v1/revocations protected with old `atap:approve` scope; or new tokens issued with old scope.
**Why it happens:** Scope string must be changed in two places: `oauth.go` allowed scope list AND `SetupRoutes` `RequireScope` call.
**How to avoid:** Grep for `atap:approve` across the entire `platform/` tree and replace all occurrences.
**Warning signs:** 403 errors when calling POST /v1/revocations with a fresh token, or no error when token has old scope.

### Pitfall 4: DIDComm `approval/1.0/revoke` Body Not Matched Correctly
**What goes wrong:** `approval_id` not found in DIDComm body because of type assertion on wrong key.
**Why it happens:** `msg.Body` is `map[string]any`; type assertions must be on correct string key per spec body format.
**How to avoid:** Use `approvalID, _ := msg.Body["approval_id"].(string)`. Log and silently skip if empty — do not reject the DIDComm message.
**Warning signs:** Revocations not being stored despite correct DIDComm delivery.

### Pitfall 5: Revocation `expires_at` Calculation Error
**What goes wrong:** For plain Approvals (without `valid_until`), `expires_at` should be `revoked_at + 60min`. Using a nil-deref on `valid_until` causes panic or zero-time.
**Why it happens:** Spec says: `expires_at` = `valid_until` if Standing Approval; OR `revoked_at + 60min` if plain Approval.
**How to avoid:** In `CreateRevocation` logic, compute: if `validUntil == nil` → `revokedAt.Add(60 * time.Minute)`, else → `*validUntil`.
**Warning signs:** Revocation entries with zero `expires_at` for non-Standing Approvals.

### Pitfall 6: Template `templateWithoutProof` After Model Restructure
**What goes wrong:** After changing `Template.Card` to `json.RawMessage` and removing old fields, existing tests for `VerifyTemplateProof` may fail if they construct `Template` structs using old fields.
**Why it happens:** Old template model tests in `approval/template_test.go` likely construct `Template` with `Brand`, `Display`, etc.
**How to avoid:** Update `approval/template_test.go` tests to use new `{ AtapTemplate: "1", Card: json.RawMessage(...), Proof: ... }` model. Remove tests for `SignTemplateProof` (function is deleted).
**Warning signs:** Compile errors in `template_test.go` referencing removed fields.

### Pitfall 7: `store.Store` No Longer Implements `ApprovalStore` After Deletion
**What goes wrong:** `store.Store` satisfies `api.ApprovalStore` interface by having all approval methods. Removing `store/approvals.go` means `store.Store` loses those methods, but if `api.ApprovalStore` interface is also removed, the compiler is satisfied.
**Why it happens:** Go's implicit interface satisfaction means removing both sides in concert is required.
**How to avoid:** Remove `store/approvals.go`, remove `api.ApprovalStore` interface, and remove the `approvalStore` Handler field in the same plan. Do not remove one without the other.

---

## Code Examples

### Migration 012 — Drop approvals, create revocations
```sql
-- Source: spec §8.15, pattern from existing migrations
-- 012_revocations.up.sql

DROP TABLE IF EXISTS approvals CASCADE;

CREATE TABLE revocations (
    id           TEXT PRIMARY KEY,         -- "rev_" + ULID
    approval_id  TEXT NOT NULL,            -- the revoked approval ID
    approver_did TEXT NOT NULL,            -- indexed for entity-based query
    revoked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL      -- self-cleaning TTL
);

CREATE INDEX idx_revocations_approver ON revocations(approver_did, expires_at);
CREATE INDEX idx_revocations_expires  ON revocations(expires_at);
```

### POST /v1/revocations Request Body
```json
{
  "approval_id": "apr_8f3a9b2c",
  "approver_did": "did:web:provider.example:human:x7k9m2w4p3n8j5q2",
  "valid_until": null,
  "signature": "<JWS-detached-compact>"
}
```

### DIDComm revocation forward to `via`
```go
// Source: existing dispatchDIDCommMessage pattern in api/approvals.go
// Used when handling TypeApprovalRevoke in didcomm_handler.go
fwdMsg := didcomm.NewMessage(
    didcomm.TypeApprovalRevoke,
    approverDID,
    []string{viaDID},
    map[string]any{
        "approval_id": approvalID,
        "revoked_at":  revokedAt.Format(time.RFC3339),
    },
)
h.dispatchDIDCommMessage(fwdMsg)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Server stores approvals in `approvals` table | Server stores only revocations; approvals held by parties | Spec v1.0-rc1 (2026-03) | Drop `approvals` table, remove 6 endpoints, remove store file |
| Server is `via` (co-signs approvals) | `via` is always an external machine entity | Spec v1.0-rc1 (2026-03) | Remove all `h.serverDID()` / `h.serverKeyID()` co-signing logic |
| Custom template JSON format | Microsoft Adaptive Cards with opaque `card` field | Spec v1.0-rc1 (2026-03) | Replace `models.Template` struct; keep signing/verification logic |
| Revocation via DELETE /v1/approvals/{id} | Revocation via DIDComm `approval/1.0/revoke` + POST /v1/revocations | Spec v1.0-rc1 (2026-03) | New revocation table and API; DIDComm handler extension |
| OAuth scope `atap:approve` | OAuth scope `atap:revoke` | Spec v1.0-rc1 (2026-03) | Rename in `oauth.go` and `SetupRoutes` |

**Deprecated/outdated (must remove in this phase):**
- `platform/internal/api/approvals.go`: entire file deleted
- `platform/internal/api/approvals_test.go`: entire file deleted
- `platform/internal/store/approvals.go`: entire file deleted
- `platform/internal/store/approvals_test.go`: entire file deleted
- `models.TemplateBrand`, `models.TemplateColors`, `models.TemplateDisplay`, `models.TemplateField`: removed
- `approval.SignTemplateProof`: removed (server never authors templates)
- `ApprovalStore` interface in `api/api.go`: removed
- `approvalStore` field on `Handler`: removed
- 6 approval routes in `SetupRoutes`: removed
- `approvals` table: dropped in migration 012
- `serverDID()` and `serverKeyID()` helpers in `api/approvals.go`: removed with file

---

## Open Questions

1. **POST /v1/revocations — what is signed?**
   - What we know: Spec §8.15 says "submit signed approval revocation" and requires `atap:revoke` scope.
   - What's unclear: The spec does not specify the exact signing scope for the REST revocation body.
   - Recommendation: Sign over JCS-canonical `{ approval_id, approver_did, revoked_at }` using the approver's key. Verify using the same `VerifyApprovalSignature` pattern from `approval/signer.go`. The `revoked_at` timestamp is included in the signed payload to prevent replay.

2. **How does the server know `valid_until` to compute `expires_at`?**
   - What we know: `expires_at` is derived from the original approval's `valid_until`. The server no longer stores approvals.
   - What's unclear: The client must include `valid_until` in the revocation request body, but the server cannot verify this matches the original.
   - Recommendation: Include `valid_until` (nullable) in the POST /v1/revocations request body and in the signed payload. If null → `expires_at = revoked_at + 60min`. If set → `expires_at = valid_until`. The signature covers this field to prevent downgrade attacks.

3. **DIDComm revocation forwarding — two-party case**
   - What we know: Server forwards revocation to `via` via DIDComm. Two-party approvals have no `via`.
   - What's unclear: The revocation DIDComm body may not include a `via` field for two-party approvals.
   - Recommendation: If `via` is absent or empty string in the revocation body, skip forwarding. No error. Only forward when `via` is a non-empty DID.

4. **DIDComm `approval/1.0/revoke` sender authentication**
   - What we know: DIDComm messages are authenticated via ECDH-1PU — server knows sender DID from JWE.
   - What's unclear: Should the handler verify the ECDH-1PU sender DID matches `approver_did` in the body?
   - Recommendation: Yes — verify sender DID from JWE header matches `approver_did` in the body. Reject mismatches with a DIDComm problem-report.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package |
| Config file | none — standard `go test ./...` from `platform/` directory |
| Quick run command | `cd platform && go test ./internal/approval/... ./internal/api/... ./internal/store/... -count=1` |
| Full suite command | `cd platform && go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| APR-14 | No approval stored after any flow | unit | `go test ./internal/store/... -run TestRevocations` | Wave 0 — new file |
| AUTH-05 | POST /v1/revocations 403 with wrong scope | unit | `go test ./internal/api/... -run TestRevocationScopeRequired` | Wave 0 — new file |
| REV-01 | DIDComm revoke message handled and forwarded to `via` | unit | `go test ./internal/api/... -run TestHandleDIDCommRevoke` | Wave 0 — extend `didcomm_handler_test.go` |
| REV-02 | Revocation record stored by approver DID | unit | `go test ./internal/store/... -run TestCreateRevocation` | Wave 0 — new file |
| REV-03 | Expired revocations removed by CleanupExpiredRevocations | unit | `go test ./internal/store/... -run TestCleanupExpiredRevocations` | Wave 0 — new file |
| REV-04 | GET /v1/revocations returns only active entries for entity | unit | `go test ./internal/api/... -run TestListRevocations` | Wave 0 — new file |
| TPL-01 | Template model accepts `card` as json.RawMessage | unit | `go test ./internal/approval/... -run TestTemplateMarshalAdaptiveCard` | Wave 0 — new test in `template_test.go` |
| TPL-03 | Template envelope serialized as `atap_template`/`card`/`proof` | unit | `go test ./internal/approval/... -run TestTemplateFormat` | Wave 0 — new test in `template_test.go` |
| TPL-05 | SSRF blocked IPs still blocked after model change | unit | `go test ./internal/approval/... -run TestIsBlockedIP` | `approval/template_test.go` (existing) |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./internal/approval/... ./internal/api/... ./internal/store/... -count=1`
- **Per wave merge:** `cd platform && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `platform/internal/api/revocations_test.go` — covers AUTH-05, REV-04, POST /v1/revocations handler
- [ ] `platform/internal/store/revocations_test.go` — covers REV-02, REV-03, APR-14
- [ ] New test cases in `platform/internal/approval/template_test.go` — covers TPL-01, TPL-03 with new model shape
- [ ] New test cases in `platform/internal/api/didcomm_handler_test.go` — covers REV-01 (DIDComm revoke handling)

*(Existing tests for deleted files — `api/approvals_test.go` and `store/approvals_test.go` — are deleted as part of Plan 1.)*

---

## Sources

### Primary (HIGH confidence)
- `spec/ATAP-SPEC-v1.0-rc1.md` §8 (Approvals), §8.15 (Revocation), §11 (Templates), §13 (API) — authoritative spec
- `.planning/phases/03-approval-engine/03-CONTEXT.md` — locked user decisions
- `.planning/REQUIREMENTS.md` — requirement definitions with phase assignments
- Direct codebase inspection: `platform/internal/api/approvals.go`, `platform/internal/store/approvals.go`, `platform/internal/approval/signer.go`, `platform/internal/approval/lifecycle.go`, `platform/internal/approval/template.go`, `platform/internal/models/models.go`, `platform/internal/api/api.go`, `platform/migrations/011_approvals.up.sql`, `platform/internal/didcomm/message.go`

### Secondary (MEDIUM confidence)
- Microsoft Adaptive Cards spec v1.5 — `card` field in template format (referenced in spec §11.2/11.3; content matches spec examples at adaptivecards.io)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all existing libraries verified in codebase
- Architecture: HIGH — spec §8.1, §8.15, §13 are explicit; deletion/addition scope is fully enumerated
- Pitfalls: HIGH — all from direct code inspection of files that must be changed
- Test map: HIGH — based on existing test patterns in the repo and new functionality boundaries

**Research date:** 2026-03-16
**Valid until:** 2026-04-16 (spec v1.0-rc1 is release candidate; stable until v1.0 final)
