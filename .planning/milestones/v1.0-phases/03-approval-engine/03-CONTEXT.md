# Phase 3: Approval Engine - Context

**Gathered:** 2026-03-15
**Status:** Ready for replanning (previous implementation invalidated by spec v1.0-rc1 changes)

<domain>
## Phase Boundary

Two-party and three-party approval flows with JWS multi-signature, Adaptive Card templates, approval lifecycle, and revocation lists. The ATAP server is transport-only (DIDComm mediator) — it does NOT store approvals. The `via` role belongs to external systems (machines like online shops, travel agents). Revocation uses a negative attestation model on the approver's ATAP server.

This is a **rework** of the previously completed Phase 3. The spec v1.0-rc1 fundamentally changed the server's role, the `via` semantics, template format, approval terminology, and the API surface.

</domain>

<decisions>
## Implementation Decisions

### Server Role
- ATAP server is **pure DIDComm transport** (mediator) for approvals — it routes messages but does not inspect, store, or validate approval content
- Server stores only: entity records, credentials, and revocation lists
- All approval CRUD endpoints (POST/GET/DELETE /v1/approvals) must be **removed**
- Server does NOT co-sign approvals — it is never `via`

### Via Semantics
- `via` is an **external machine** entity (e.g., online shop, travel agent, bank) — NOT the ATAP server
- The `via` system validates business rules, co-signs the approval, and provides the branded Adaptive Card template
- Flow: agent (`from`) → via online shop (`via`) validates + co-signs → human (`to`) approves/declines
- The `via` knows the payload (it's their transaction), provides the template (their brand), and co-signs (vouching for business context)

### Approval / Standing Approval Terminology
- **Approval** (was "one-time"): `valid_until` absent/null. Default TTL 60 minutes (RECOMMENDED, system-configurable). Transitions to `consumed` after use or `expired` after TTL.
- **Standing Approval** (was "persistent"): `valid_until` set as ISO 8601 datetime. Valid for repeated use until expiry, subject to `max_approval_ttl`.

### Templates
- Templates use **Microsoft Adaptive Cards** format — not custom JSON
- Template provided exclusively by `via` system, signed with JWS proof
- **Client fetches template directly** from `via`'s `template_url` — server never touches templates
- Mandatory JWS verification against `via` DID — if verification fails, fall back to generic rendering
- Fall back to generic rendering if template URL unreachable; notify `via` about template failure via DIDComm signal
- Two-party approvals use fallback rendering: `subject.label` as title + `subject.payload` as formatted JSON
- Fallback rendering needs a separate spec for what data is shown vs hidden (dates, subject, message, payload as JSON in details view) — deferred
- Full spec security rules apply on mobile client: HTTPS only, 64KB max, 5s timeout, IP blocking, `$schema` NOT a fetch target
- Adaptive Card `Action.Submit` and `Action.OpenUrl` MUST be disabled

### Revocation
- Revocation via DIDComm message (`approval/1.0/revoke`) sent by approver to their ATAP server
- Server stores revocation entries (negative attestation model) — not approvals
- Server **forwards revocation to `via`** system via DIDComm so `via` can cache locally
- Self-cleaning revocation lists: each entry carries `expires_at` (from `valid_until` or `revoked_at` + 60min). Servers SHOULD remove expired entries.
- Revocation check: `via` checks **local cache first** (fast path), then queries approver's ATAP server (authoritative)
- Revocation list API: `GET /v1/revocations?entity={approver-did}` returns active revocations with `revoked_at` and `expires_at`

### API Changes
- **Removed**: POST /v1/approvals, POST /v1/approvals/{id}/respond, GET /v1/approvals/{id}, GET /v1/approvals/{id}/status, GET /v1/approvals, DELETE /v1/approvals/{id}
- **Added**: POST /v1/revocations (submit signed revocation, `atap:revoke` scope), GET /v1/revocations (query by entity DID, public)
- OAuth scope changed: `atap:approve` → `atap:revoke`

### Claude's Discretion
- How to refactor existing approval code (keep reusable parts like JWS signing, JCS canonicalization, lifecycle state machine)
- Database migration strategy (remove approvals table, add revocations table)
- How to structure the approval model code now that server doesn't store approvals (library vs handler)

</decisions>

<specifics>
## Specific Ideas

- "The ATAP server does NOT co-sign anything, it is pure transport. Identities can be checked by the receiving party."
- "Via is the author of the message — it's called via because agent VIA onlineshop TO human. The online shop knows the payload, can sign it, and provides the template."
- "Fallback rendering needs specification for what data is rendered (dates, subject, message) and what payload data is hidden or shown as JSON in a details view."
- "It is in the machine's risk zone to follow standing approvals; it is wise to check revocation lists before acting."
- Agent-provided templates for two-party approvals: noted for later, no priority now.

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `platform/internal/approval/signer.go`: JWS signing and verification (SignApproval, VerifyApprovalSignature) — reusable as-is, signing logic is correct
- `platform/internal/approval/lifecycle.go`: State machine (ValidateTransition, IsTerminalState) — reusable, may need updates for Approval/Standing Approval terminology
- `platform/internal/approval/template.go`: Template fetch with SSRF prevention — needs rework for Adaptive Cards format, but SSRF logic reusable
- `platform/internal/crypto/`: Ed25519 key gen, JCS canonicalization, ID generation — fully reusable
- `platform/internal/didcomm/`: DIDComm message types and dispatch — reusable, message types already defined

### Established Patterns
- JCS canonicalization for signing scope (RFC 8785) — keep as-is
- DIDComm message dispatch via messageStore.QueueMessage — keep for revocation forwarding
- RFC 7807 Problem Details for errors — keep

### Integration Points
- `platform/internal/api/approvals.go`: Must be gutted — remove all 6 approval endpoints, replace with revocation endpoints
- `platform/internal/store/approvals.go`: Must be reworked — remove approval CRUD, add revocation list storage
- `platform/internal/api/api.go` (SetupRoutes): Remove approval route group, add revocation routes
- `platform/internal/models/`: Approval model stays as a library type (for signing/verification), but is no longer stored server-side
- OAuth middleware: scope `atap:approve` → `atap:revoke`

</code_context>

<deferred>
## Deferred Ideas

- Agent-provided templates for two-party approvals — future phase, no priority
- Fallback rendering specification (what data is shown/hidden in detail view) — separate spec work
- Template failure notification signal to `via` — implement when mobile client is built (Phase 4)

</deferred>

---

*Phase: 03-approval-engine*
*Context gathered: 2026-03-15*
