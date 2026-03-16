---
status: testing
phase: 03-approval-engine
source: [03-01-SUMMARY.md, 03-02-SUMMARY.md]
started: 2026-03-16T12:00:00Z
updated: 2026-03-16T12:00:00Z
---

## Current Test

number: 1
name: Cold Start Smoke Test
expected: |
  Kill any running server/containers. Run `docker compose down -v && docker compose up -d` (or rebuild from scratch). Server boots without errors, migration 012 applies (drops approvals table, creates revocations table), and `GET /v1/health` or any basic API call returns a live response.
awaiting: user response

## Tests

### 1. Cold Start Smoke Test
expected: Kill any running server/containers. Run `docker compose down -v && docker compose up -d` (or rebuild from scratch). Server boots without errors, migration 012 applies (drops approvals, creates revocations), and a basic API call returns a live response.
result: [pending]

### 2. Submit a Revocation
expected: `POST /v1/revocations` with Bearer token (atap:revoke scope), body `{"approval_id": "...", "signature": "..."}` returns 201 with a `rev_` prefixed ID and the revocation details. Without atap:revoke scope, returns 403.
result: [pending]

### 3. List Revocations
expected: `GET /v1/revocations?entity=did:web:...` returns 200 with an array of active (non-expired) revocations for that entity. Missing `entity` query param returns 400.
result: [pending]

### 4. OAuth Scope Updated
expected: Token creation/validation recognizes `atap:revoke` as a valid scope. The old `atap:approve` scope is rejected (not in validScopes). Requesting a token with `atap:approve` returns an error.
result: [pending]

### 5. Old Approval Endpoints Removed
expected: `POST /v1/approvals`, `GET /v1/approvals`, `GET /v1/approvals/:id`, etc. all return 404 (not found). No approval routes exist.
result: [pending]

### 6. DIDComm Revocation Processing
expected: Send a DIDComm message of type `approval/1.0/revoke` addressed to the server DID (`did:web:{domain}:server:platform`). The server decrypts the JWE, stores a revocation record, and if `via` is present in the body, forwards the message to the target entity.
result: [pending]

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0

## Gaps

[none yet]
