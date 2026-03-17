---
phase: 03-approval-engine
verified: 2026-03-16T00:00:00Z
status: passed
score: 7/7 must-haves verified
re_verification:
  previous_status: passed
  previous_score: 5/5
  note: >
    Previous VERIFICATION.md was written for the original approval-CRUD implementation
    (Phase 3 prior to spec v1.0-rc1 rework). Plans 03-01 and 03-02 completely replaced
    that implementation. This is a fresh verification against the new plans.
  gaps_closed:
    - "Server approval storage removed (was not a gap in old report but was a correctness issue)"
    - "RevocationStore replaces ApprovalStore"
    - "Template model updated to Adaptive Cards format"
    - "DIDComm handler extended for TypeApprovalRevoke"
  gaps_remaining: []
  regressions: []
gaps: []
human_verification:
  - test: "Approver sends TypeApprovalRevoke DIDComm JWE to server; server stores revocation; via DID receives forwarded message in inbox"
    expected: "Revocation stored in DB with correct approver_did and expires_at; via entity's inbox contains the forwarded revoke message"
    why_human: "Requires live ECDH-1PU JWE construction with real keypairs and a running server + database; cannot be exercised in the static analysis environment"
  - test: "GET /v1/revocations?entity={did} returns only non-expired entries after an entry expires"
    expected: "Entries with expires_at in the past are absent; entries with future expires_at are present; checked_at field is within 1 second of current time"
    why_human: "Time-sensitive expiry behavior requires real-time clock and a live database"
---

# Phase 3: Approval Engine (Rework) Verification Report

**Phase Goal:** Revocation infrastructure — server strips approval storage, adds revocation list API (POST/GET /v1/revocations), updates OAuth scopes, converts templates to Adaptive Cards format, and handles DIDComm `approval/1.0/revoke` messages with storage and via forwarding.
**Verified:** 2026-03-16T00:00:00Z
**Status:** passed
**Re-verification:** Yes — prior VERIFICATION.md was for superseded approval-CRUD implementation; this verifies the spec v1.0-rc1 rework (Plans 03-01 and 03-02).

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                      | Status     | Evidence                                                                                     |
|----|-------------------------------------------------------------------------------------------|------------|----------------------------------------------------------------------------------------------|
| 1  | Server does not store approvals — no approvals table, no approval CRUD endpoints          | ✓ VERIFIED | `api/approvals.go` and `store/approvals.go` deleted; migration 012 drops approvals table; no `/approvals` routes in `api.go` |
| 2  | POST /v1/revocations creates a revocation entry and returns 201                           | ✓ VERIFIED | `SubmitRevocation` in `api/revocations.go`; derives approver_did from auth context; calls `CreateRevocation`; returns 201 JSON with id/approval_id/approver_did/revoked_at/expires_at |
| 3  | GET /v1/revocations?entity={did} returns active revocations for that approver DID         | ✓ VERIFIED | `ListRevocations` handler queries `revocationStore.ListRevocations`; requires `entity` param; returns `{ entity, revocations, checked_at }` |
| 4  | OAuth scope atap:approve is replaced by atap:revoke everywhere in production code         | ✓ VERIFIED | `validScopes` and `allScopes` in `oauth.go` use `atap:revoke`; error message updated; zero matches for `atap:approve` in platform source |
| 5  | POST /v1/revocations requires atap:revoke scope; GET /v1/revocations is public            | ✓ VERIFIED | `auth.Post("/revocations", h.RequireScope("atap:revoke"), h.SubmitRevocation)` in `api.go:142`; `v1.Get("/revocations", h.ListRevocations)` at line 131 (before auth group) |
| 6  | Expired revocations are cleaned up by background goroutine                                | ✓ VERIFIED | `main.go` goroutine at line 130 calls `db.CleanupExpiredRevocations` on 5-minute ticker; no approval cleanup goroutine remains |
| 7  | Server compiles and all tests pass after rework                                           | ✓ VERIFIED | `go build ./...` exits 0; `go test ./...` passes all 7 packages (api, approval, crypto, didcomm, store, models, config) |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact                                          | Expected                                          | Status     | Details                                                                                            |
|---------------------------------------------------|---------------------------------------------------|------------|----------------------------------------------------------------------------------------------------|
| `platform/internal/api/revocations.go`            | SubmitRevocation and ListRevocations handlers     | ✓ VERIFIED | 109 lines; SubmitRevocation derives approver_did from auth context, generates rev_ ULID, creates DB entry; ListRevocations enforces entity param |
| `platform/internal/store/revocations.go`          | CreateRevocation, ListRevocations, CleanupExpiredRevocations | ✓ VERIFIED | 60 lines; ListRevocations filters `expires_at > NOW()`; CleanupExpiredRevocations returns rows affected |
| `platform/migrations/012_revocations.up.sql`      | DROP approvals table, CREATE revocations table    | ✓ VERIFIED | `DROP TABLE IF EXISTS approvals CASCADE`; creates revocations with UNIQUE approval_id, compound index on (approver_did, expires_at), sparse expires index |
| `platform/internal/models/models.go`              | Revocation struct and Adaptive Cards Template struct | ✓ VERIFIED | `type Revocation struct` at line 250; `type Template struct` uses `json.RawMessage Card` at line 264; old TemplateBrand/Colors/Display/Field types absent |
| `platform/internal/approval/template.go`          | FetchTemplate, VerifyTemplateProof, IsBlockedIP — no SignTemplateProof | ✓ VERIFIED | 199 lines; SignTemplateProof absent; FetchTemplate and VerifyTemplateProof work with new Template model; IsBlockedIP exported |
| `platform/internal/api/didcomm_handler.go`        | handleServerAddressedMessage, processApprovalRevoke, dispatchDIDCommMessage | ✓ VERIFIED | handleServerAddressedMessage detects server DID, decrypts JWE with ECDH-1PU; processApprovalRevoke stores revocation + forwards to via; dispatchDIDCommMessage recreated |
| `platform/internal/api/api.go`                    | RevocationStore interface, no ApprovalStore, correct routes | ✓ VERIFIED | RevocationStore interface at line 44; Handler.revocationStore field at line 67; NewHandler takes rs RevocationStore param; routes wired at lines 131 and 142 |

### Key Link Verification

| From                                      | To                                            | Via                                            | Status     | Details                                                                                        |
|-------------------------------------------|-----------------------------------------------|------------------------------------------------|------------|------------------------------------------------------------------------------------------------|
| `api/revocations.go`                      | `store/revocations.go`                        | `h.revocationStore.CreateRevocation / ListRevocations` | ✓ WIRED | Both calls present at revocations.go:63 and :91; revocationStore field wired in NewHandler |
| `api/api.go`                              | `api/revocations.go`                          | `SetupRoutes` registers /v1/revocations        | ✓ WIRED    | `h.ListRevocations` at api.go:131; `h.SubmitRevocation` at api.go:142                        |
| `cmd/server/main.go`                      | `api/api.go`                                  | `NewHandler` takes db as RevocationStore (5th param) | ✓ WIRED | main.go:116 `api.NewHandler(db, db, db, db, db, ...)` — 5th db satisfies RevocationStore    |
| `api/didcomm_handler.go`                  | `store/revocations.go`                        | `h.revocationStore.CreateRevocation` on TypeApprovalRevoke | ✓ WIRED | didcomm_handler.go:236 calls CreateRevocation; revocationStore injected via Handler struct  |
| `api/didcomm_handler.go`                  | `api/didcomm_handler.go`                      | `dispatchDIDCommMessage` / `QueueMessage` for via forwarding | ✓ WIRED | processApprovalRevoke calls `h.messageStore.QueueMessage` at line 257 for via forwarding    |
| `approval/template.go`                    | `models/models.go`                            | Uses `models.Template` with `json.RawMessage Card` | ✓ WIRED | `var tmpl models.Template` at template.go:141; Template struct has Card json.RawMessage field |
| `cmd/server/main.go`                      | `store/revocations.go`                        | CleanupExpiredRevocations goroutine            | ✓ WIRED    | main.go:135 `db.CleanupExpiredRevocations(context.Background())`                              |

### Requirements Coverage

| Requirement | Source Plan | Description                                                                                               | Status       | Evidence                                                                                                         |
|-------------|------------|-----------------------------------------------------------------------------------------------------------|--------------|------------------------------------------------------------------------------------------------------------------|
| APR-01      | 03-01      | Two-party approvals: server is transport only, no storage                                                 | ✓ SATISFIED  | approvals.go deleted; server has no approval storage; approval documents are DIDComm-transported between parties |
| APR-02      | 03-01      | Three-party approvals: via is external machine, not ATAP server                                           | ✓ SATISFIED  | No server co-signing code; server only mediates DIDComm; via forwarding in processApprovalRevoke is relay-only   |
| APR-03      | 03-01      | Approval format with atap_approval, apr_ IDs, ISO 8601; approvals stored by parties not server           | ✓ SATISFIED  | Approval struct retained for library use (signing/verification); server does not persist approvals               |
| APR-04      | 03-01      | Subject contains type, label, reversible, payload                                                         | ✓ SATISFIED  | ApprovalSubject struct retained in models.go for approval document construction by external parties              |
| APR-05      | 03-01      | JWS Compact Serialization with detached payload (RFC 7515 + RFC 7797)                                    | ✓ SATISFIED  | approval/signer.go unchanged; SignApproval and VerifyApprovalSignature retained; tests pass                      |
| APR-06      | 03-01      | Signed payload is JCS-serialized approval excluding signatures field                                      | ✓ SATISFIED  | approval/signer.go unchanged; canonicalization pattern preserved; signer_test.go passes                          |
| APR-07      | 03-01      | Full lifecycle: requested/approved/declined/expired/rejected/consumed/revoked; tracked by via not server  | ✓ SATISFIED  | Lifecycle constants and state machine in approval/lifecycle.go; server no longer stores state; lifecycle_test passes |
| APR-08      | 03-01      | TypeApprovalRejected message type passes through server unchanged                                         | ✓ SATISFIED  | TestDIDCommRejectedPassthrough: TypeApprovalRejected addressed to entity DID is queued unchanged via passthrough path |
| APR-09      | 03-01      | Default TTL 60 minutes for approvals without valid_until; tracked by via not server                       | ✓ SATISFIED  | Server revocation default: `revokedAt.Add(60 * time.Minute)` at revocations.go:47 and didcomm_handler.go:224    |
| APR-10      | 03-01      | Standing Approvals with valid_until; max_approval_ttl enforcement                                         | ✓ SATISFIED  | max_approval_ttl: 86400 published in discovery.go:22; valid_until respected in SubmitRevocation expires_at logic |
| APR-11      | 03-01      | Chained approvals; revoking parent invalidates children                                                   | ✓ SATISFIED  | approval/lifecycle.go and approval package retained; parent field and chaining logic preserved for external use   |
| APR-12      | 03-01      | Extract kid from JWS header, resolve DID, verify signature                                                | ✓ SATISFIED  | approval/signer.go VerifyApprovalSignature retained; signer_test.go passes                                       |
| APR-13      | 03-01      | Standing Approval enforcement: via MUST verify signatures, expiry, revocation list, DID liveness, etc.   | ✓ SATISFIED  | Server provides GET /v1/revocations for revocation list queries (spec §8.14 obligation is on via system, not server) |
| APR-14      | 03-01      | Server does not store approvals; stores only entity records, credentials, revocation lists                | ✓ SATISFIED  | api/approvals.go and store/approvals.go deleted; migration 012 drops approvals table; only revocations table added |
| AUTH-05     | 03-01      | Token scopes: atap:inbox, atap:send, atap:revoke, atap:manage (approve replaced by revoke)               | ✓ SATISFIED  | oauth.go validScopes and allScopes use atap:revoke; zero instances of atap:approve in production code             |
| MSG-03      | 03-01      | Server is DIDComm mediator only; via role belongs to external systems                                     | ✓ SATISFIED  | No server co-signing in any handler; server queues/relays messages only; processApprovalRevoke does relay forwarding |
| REV-01      | 03-01/03-02 | Revocation via DIDComm TypeApprovalRevoke; server stores + forwards to via                               | ✓ SATISFIED  | processApprovalRevoke in didcomm_handler.go stores revocation, forwards to via DID via QueueMessage; 5 tests pass  |
| REV-02      | 03-01      | Server stores revoked approval IDs indexed by approver DID                                                | ✓ SATISFIED  | revocations table with idx_revocations_approver on (approver_did, expires_at); ListRevocations queries by approver_did |
| REV-03      | 03-01      | Self-cleaning revocation lists with expires_at; servers remove expired entries                            | ✓ SATISFIED  | CleanupExpiredRevocations deletes where expires_at < NOW(); background goroutine in main.go runs every 5 minutes   |
| REV-04      | 03-01      | GET /v1/revocations?entity={approver-did} returns active revoked approval IDs                             | ✓ SATISFIED  | ListRevocations handler at revocations.go:84; returns entity, revocations[], checked_at; requires entity param    |
| REV-05      | 03-02      | via system checks local cache first then queries approver's ATAP server                                   | ✓ SATISFIED  | Server provides public GET /v1/revocations endpoint for this query path (obligation is on via client, not server) |
| TPL-01      | 03-02      | Templates use Adaptive Cards format; provided exclusively by via system (external machine)                | ✓ SATISFIED  | Template struct has Card json.RawMessage field; SignTemplateProof removed; FetchTemplate fetches external URL     |
| TPL-02      | 03-02      | Templates carry JWS proof signed by via; client verifies against via DID                                  | ✓ SATISFIED  | VerifyTemplateProof retained in template.go; template_test.go TestVerifyTemplateProofAdaptiveCard passes          |
| TPL-03      | 03-02      | Template wraps Adaptive Card in atap_template envelope with card and proof fields                         | ✓ SATISFIED  | models.Template: AtapTemplate string + Card json.RawMessage + Proof TemplateProof; TestTemplateFormat passes      |
| TPL-04      | 03-02      | Data binding via Adaptive Card Templating syntax; context: subject/payload/brand/from/to/via              | ✓ SATISFIED  | Card field is opaque json.RawMessage; data binding is client-side per spec; server stores/ferries card verbatim    |
| TPL-05      | 03-02      | Security: HTTPS only, no redirects, IP validation, 64KB max, 5s timeout                                  | ✓ SATISFIED  | FetchTemplate: https:// prefix check; CheckRedirect returns ErrUseLastResponse; ssrfSafeTransport; LimitReader 64KB; Timeout 5s; 9 SSRF tests pass |
| TPL-06      | 03-02      | Two-party approvals use fallback rendering (label + formatted JSON payload)                               | ✓ SATISFIED  | FetchTemplate returns nil, nil for empty URL; TestFallbackRendering passes                                        |
| API-03      | 03-01      | Revocation endpoints: POST /v1/revocations, GET /v1/revocations; no approval CRUD endpoints               | ✓ SATISFIED  | Both routes registered in SetupRoutes; all 6 old approval routes removed; TestSubmitRevocation_* and TestListRevocations_* pass |

All 27 requirements verified. No orphaned requirements — all IDs claimed in plans map directly to implementations.

**Note on requirements not covered by this phase's new plans but listed in the phase requirement IDs:**
APR-04 through APR-06, APR-11, APR-12 are served by the retained `approval/signer.go` and `approval/lifecycle.go` packages which were unchanged in this rework. Their tests still pass as confirmed by `go test ./internal/approval/...`.

### Anti-Patterns Found

No blockers or warnings detected. Scan of all phase-modified files returned clean:

- No TODO/FIXME/PLACEHOLDER comments in phase 3 artifacts
- No empty return stubs
- No handler-only-prevents-default patterns
- One informational item: `// TODO Phase 4: add IP-based rate limiting` at `api.go:127` — correctly deferred, out of scope for this phase

### Human Verification Required

**1. End-to-End Revocation via DIDComm**

**Test:** Construct a TypeApprovalRevoke JWE encrypted to the server's X25519 public key with ECDH-1PU (sender key known to server). POST to `/v1/didcomm`. Verify: (a) revocation entry appears in database with correct approver_did and expires_at; (b) if `via` field present in message body, the via entity's inbox contains a forwarded revoke message.
**Expected:** Revocation stored; via entity receives forwarded message; server returns 202 Accepted.
**Why human:** Requires live ECDH-1PU JWE construction with real keypairs and a running server + PostgreSQL instance.

**2. Revocation List Expiry Behaviour**

**Test:** Create a revocation with `expires_at` 2 minutes in the future. Immediately query GET /v1/revocations — entry should be present. Wait 2 minutes. Query again — entry should be absent (either cleaned up by goroutine or filtered by `expires_at > NOW()` in the SELECT).
**Expected:** Entry absent after expiry without any explicit deletion request.
**Why human:** Time-sensitive; requires real database and real-time clock progression.

### Gaps Summary

No gaps. All automated checks pass. Phase 3 goal is achieved.

The implementation correctly strips all server-side approval storage, implements the revocation list API with proper scope enforcement, updates OAuth scopes to `atap:revoke`, converts the Template model to Adaptive Cards format, removes `SignTemplateProof` from the server package, and extends the DIDComm handler to decrypt and process `approval/1.0/revoke` messages with revocation storage and optional via forwarding. All 7 test packages compile and pass. All 27 requirements are satisfied.

---

_Verified: 2026-03-16T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
