---
status: complete
phase: 03-approval-engine
source: [03-01-SUMMARY.md, 03-02-SUMMARY.md]
started: 2026-03-16T12:00:00Z
updated: 2026-03-16T12:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/containers. Run `docker compose down -v && docker compose up -d` (or rebuild from scratch). Server boots without errors, migration 012 applies (drops approvals, creates revocations), and a basic API call returns a live response.
result: pass

### 2. Submit a Revocation
expected: `POST /v1/revocations` with Bearer token (atap:revoke scope), body `{"approval_id": "...", "signature": "..."}` returns 201 with a `rev_` prefixed ID and the revocation details. Without atap:revoke scope, returns 403.
result: pass

### 3. List Revocations
expected: `GET /v1/revocations?entity=did:web:...` returns 200 with an array of active (non-expired) revocations for that entity. Missing `entity` query param returns 400.
result: pass

### 4. OAuth Scope Updated
expected: Token creation/validation recognizes `atap:revoke` as a valid scope. The old `atap:approve` scope is rejected (not in validScopes). Requesting a token with `atap:approve` returns an error.
result: pass

### 5. Old Approval Endpoints Removed
expected: `POST /v1/approvals`, `GET /v1/approvals`, `GET /v1/approvals/:id`, etc. all return 404 (not found). No approval routes exist.
result: pass

### 6. DIDComm Message Delivery
expected: Send a DIDComm JWE message to a registered entity via POST /v1/didcomm. Server validates recipient domain, queues message, returns 202 with message ID. (Server-addressed ECDH-1PU revoke path covered by unit tests.)
result: pass

## Summary

total: 6
passed: 6
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
