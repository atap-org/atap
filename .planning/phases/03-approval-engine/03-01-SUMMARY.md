---
phase: 03-approval-engine
plan: "01"
subsystem: approval-engine
tags: [revocations, oauth-scopes, migration, store, api]
dependency_graph:
  requires: []
  provides: [revocation-api, revocation-store, atap:revoke-scope]
  affects: [platform/internal/api, platform/internal/store, platform/internal/models, platform/migrations]
tech_stack:
  added: []
  patterns: [revocation-list, adaptive-cards-template]
key_files:
  created:
    - platform/internal/api/revocations.go
    - platform/internal/api/revocations_test.go
    - platform/internal/store/revocations.go
    - platform/internal/store/revocations_test.go
    - platform/migrations/012_revocations.up.sql
    - platform/migrations/012_revocations.down.sql
  modified:
    - platform/internal/models/models.go
    - platform/internal/api/api.go
    - platform/internal/api/oauth.go
    - platform/internal/api/oauth_test.go
    - platform/internal/store/oauth_test.go
    - platform/cmd/server/main.go
    - platform/internal/approval/template_test.go
  deleted:
    - platform/internal/api/approvals.go
    - platform/internal/api/approvals_test.go
    - platform/internal/store/approvals.go
    - platform/internal/store/approvals_test.go
decisions:
  - "Server stores revocations (not approvals): approver DID taken from auth context to prevent spoofing"
  - "Template model updated to Adaptive Cards format: removed TemplateBrand/Colors/Display/Field types"
  - "atap:approve scope replaced by atap:revoke everywhere in production code and tests"
  - "RevocationStore replaces ApprovalStore in Handler: NewHandler param count unchanged (5 db params)"
metrics:
  duration_minutes: 8
  completed_date: "2026-03-16"
  tasks_completed: 2
  files_changed: 17
---

# Phase 03 Plan 01: Strip Approval Storage, Add Revocation Infrastructure Summary

**One-liner:** Deleted server-side approval storage (5 CRUD endpoints + DB table) and replaced with a revocation list API (POST/GET /v1/revocations) backed by a new revocations table and atap:revoke OAuth scope.

## What Was Built

### Task 1: Delete approval storage, add Revocation model + migration + store

- Deleted `api/approvals.go`, `api/approvals_test.go`, `store/approvals.go`, `store/approvals_test.go` (2275 lines removed)
- Added `models.Revocation` struct with fields: `id`, `approval_id`, `approver_did`, `revoked_at`, `expires_at`
- Updated `models.Template` to Adaptive Cards format: `atap_template + card (RawMessage) + proof`; removed `TemplateBrand`, `TemplateColors`, `TemplateDisplay`, `TemplateField` types
- Created migration 012: `DROP TABLE IF EXISTS approvals CASCADE` + `CREATE TABLE revocations` with compound index on `(approver_did, expires_at)`
- Implemented `store.CreateRevocation`, `store.ListRevocations` (WHERE expires_at > NOW()), `store.CleanupExpiredRevocations`
- Written 4 contract tests: create/list, expired exclusion, cleanup count, duplicate approval_id

**Commit:** `02891e1`

### Task 2: Revocation API handlers, route wiring, scope change, main.go update

- Replaced `ApprovalStore` interface with `RevocationStore` in `api.go`; updated `Handler` struct and `NewHandler` signature
- Removed all 6 approval routes (`POST /v1/approvals`, `POST /v1/approvals/:id/respond`, `GET /v1/approvals`, `GET /v1/approvals/:id`, `GET /v1/approvals/:id/status`, `DELETE /v1/approvals/:id`)
- Registered: `GET /v1/revocations` (public) and `auth.Post("/revocations", RequireScope("atap:revoke"), SubmitRevocation)`
- Created `api/revocations.go`: `SubmitRevocation` (takes `approval_id`, `valid_until?`, `signature`; derives approver DID from auth context; generates `rev_` + ULID) and `ListRevocations` (requires `entity` query param)
- Changed `atap:approve` → `atap:revoke` in `validScopes`, `allScopes`, error message in `oauth.go`
- Updated all test files: `oauth_test.go`, `store/oauth_test.go`
- Updated `main.go`: approval cleanup goroutine → revocation cleanup goroutine (`CleanupExpiredRevocations`)
- Written 4 API handler tests: success 201, wrong scope 403, list success 200, missing entity 400

**Commit:** `d1d5877`

## Verification

```
go build ./...    → PASS
go test ./...     → PASS (all packages)
grep approvalStore/ApprovalStore/atap:approve/"/approvals" in production code → 0 matches
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed approval/template_test.go to use Adaptive Cards Template format**
- **Found during:** Task 2 full test run (`go test ./...`)
- **Issue:** `makeTestTemplate()` in `platform/internal/approval/template_test.go` referenced deleted types `models.TemplateBrand`, `models.TemplateColors`, `models.TemplateDisplay`, `models.TemplateField` and the removed `SubjectType` field
- **Fix:** Updated `makeTestTemplate()` to construct a valid Adaptive Cards template with `json.RawMessage` card body
- **Files modified:** `platform/internal/approval/template_test.go`
- **Commit:** `d1d5877`

## Self-Check: PASSED

| Item | Result |
|------|--------|
| `platform/internal/api/revocations.go` | FOUND |
| `platform/internal/store/revocations.go` | FOUND |
| `platform/migrations/012_revocations.up.sql` | FOUND |
| `platform/internal/api/approvals.go` | CONFIRMED DELETED |
| Commit `02891e1` | FOUND |
| Commit `d1d5877` | FOUND |
| `go build ./...` | PASS |
| `go test ./...` | PASS (7 packages) |
