---
phase: 03-approval-engine
plan: "02"
subsystem: approval-engine
tags: [template, adaptive-cards, didcomm, revocation, ecdh-1pu]
dependency_graph:
  requires: ["03-01"]
  provides: ["template-adaptive-cards", "didcomm-revoke-handler"]
  affects: ["platform/internal/approval", "platform/internal/api"]
tech_stack:
  added: []
  patterns: ["ECDH-1PU sender lookup via entity store", "server-DID dispatch pattern"]
key_files:
  created: []
  modified:
    - platform/internal/approval/template.go
    - platform/internal/approval/template_test.go
    - platform/internal/api/didcomm_handler.go
    - platform/internal/api/didcomm_handler_test.go
decisions:
  - "SignTemplateProof removed from server package — server never authors templates (spec v1.0-rc1)"
  - "ECDH-1PU decryption for server-addressed JWEs requires sender entity lookup by SKID DID"
  - "dispatchDIDCommMessage recreated inline in didcomm_handler.go (was deleted with approvals.go in 03-01)"
  - "processApprovalRevoke uses approver_did from message body as the revocation's approver_did"
  - "TypeApprovalRevoke forwards to via using QueueMessage with MessageType set (not full ECDH re-encrypt)"
metrics:
  duration_minutes: 7
  completed_date: "2026-03-16"
  tasks_completed: 2
  files_modified: 4
---

# Phase 3 Plan 2: Adaptive Cards Template + DIDComm Revoke Handler Summary

**One-liner:** Removed `SignTemplateProof` from server, updated template tests for Adaptive Cards format, and extended DIDComm handler to decrypt server-addressed JWEs and process `approval/1.0/revoke` messages with revocation storage and optional via forwarding.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Update template library for Adaptive Cards + remove SignTemplateProof | 262ec93 | template.go, template_test.go |
| 2 | Extend DIDComm handler for TypeApprovalRevoke + dispatchDIDCommMessage | 539fd10 | didcomm_handler.go, didcomm_handler_test.go |

## What Was Built

### Task 1: Template Library — Adaptive Cards Format

`platform/internal/approval/template.go`:
- Removed `SignTemplateProof` entirely (server never authors templates per spec v1.0-rc1)
- `FetchTemplate` and `VerifyTemplateProof` unchanged in logic — they work with the new `models.Template{Card: json.RawMessage}` shape from Plan 01
- `templateWithoutProof` correctly handles `json.RawMessage` via marshal/unmarshal round-trip

`platform/internal/approval/template_test.go`:
- Updated `TestVerifyTemplateProof` and `TestVerifyTemplateProofWrongKey` to sign using go-jose directly (no longer calls deleted `SignTemplateProof`)
- Added `TestTemplateMarshalAdaptiveCard`: verifies `card` is a nested JSON object, no legacy fields (`brand`, `display`, `subject_type`)
- Added `TestTemplateFormat`: verifies `atap_template` / `card` / `proof` envelope structure
- Added `TestTemplateJSONRoundTrip`: verifies `json.RawMessage` card survives marshal/unmarshal
- Added `TestVerifyTemplateProofAdaptiveCard`: sign and verify with Adaptive Card content
- All `IsBlockedIP` SSRF tests still pass

### Task 2: DIDComm Revocation Handler

`platform/internal/api/didcomm_handler.go`:
- Added server DID detection after Step 6 entity lookup: `"did:web:{domain}:server:platform"`
- Added `handleServerAddressedMessage`: looks up sender entity by SKID DID, decrypts JWE via `didcomm.Decrypt`, dispatches by message type
- Added `processApprovalRevoke`: extracts `approval_id`, `approver_did`, optional `via` and `valid_until`, stores `Revocation` via `h.revocationStore.CreateRevocation`, forwards to via DID via `QueueMessage` when `via` is present
- Added `dispatchDIDCommMessage`: helper to serialize a `PlaintextMessage` and queue via `messageStore.QueueMessage` (recreated from deleted `approvals.go`)
- All existing passthrough behavior preserved — non-server-addressed messages queue unchanged

`platform/internal/api/didcomm_handler_test.go`:
- Added `buildServerHandlerWithRevocationStore`: creates Handler with real `platformX25519Key` + `revocationStore`
- Added `buildServerAddressedJWE`: builds server-addressed JWE using `didcomm.Encrypt`, returns both JWE and sender X25519 public key
- Tests: `TestDIDCommRevokeStoresRevocation`, `TestDIDCommRevokeDefaultExpiresAt`, `TestDIDCommRevokeWithValidUntil`, `TestDIDCommRevokeSkipsForwardingWithNoVia`, `TestDIDCommRevokeForwardsToVia`
- Regression: `TestDIDCommRejectedPassthrough` — TypeApprovalRejected still queued unchanged (APR-08 preserved)

## Decisions Made

1. **SignTemplateProof removed**: Per spec v1.0-rc1, templates are authored by `via` entities client-side. The server only verifies, never signs. Tests updated to sign using go-jose directly.

2. **ECDH-1PU sender lookup**: The `didcomm.Decrypt` function requires the sender's static X25519 public key. The handler extracts the SKID from the JWE protected header, looks up the sender entity via `GetEntityByDID`, and uses their `X25519PublicKey` for decryption. If the sender is not found, falls through to passthrough queuing.

3. **dispatchDIDCommMessage recreated**: This helper was deleted with `approvals.go` in Plan 01. Recreated in `didcomm_handler.go` where it co-locates with the DIDComm handling code.

4. **Forward via QueueMessage**: Rather than re-encrypting the revoke message, the server queues a `DIDCommMessage` with the plaintext payload serialized to JSON. This is simpler and correct for the server-mediated forwarding case.

5. **approver_did from body**: The `approver_did` for the stored revocation comes from the message body field `approver_did`. If absent, falls back to `senderDID` extracted from the JWE SKID. This is consistent with the HTTP revocation endpoint which uses the auth context entity DID.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

Files exist:
- platform/internal/approval/template.go — modified (SignTemplateProof removed)
- platform/internal/approval/template_test.go — updated with new Adaptive Cards tests
- platform/internal/api/didcomm_handler.go — extended with server-DID processing
- platform/internal/api/didcomm_handler_test.go — new revoke tests added

Commits:
- 262ec93 — feat(03-02): update template library for Adaptive Cards format
- 539fd10 — feat(03-02): extend DIDComm handler for TypeApprovalRevoke processing
