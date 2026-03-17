---
phase: 3
slug: approval-engine
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `cd platform && go test ./internal/approval/... ./internal/api/... ./internal/store/... -count=1` |
| **Full suite command** | `cd platform && go test ./... -count=1` |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/approval/... ./internal/api/... ./internal/store/... -count=1`
- **After every plan wave:** Run `cd platform && go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | APR-14 | unit | `go test ./internal/store/... -run TestRevocations` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | AUTH-05 | unit | `go test ./internal/api/... -run TestRevocationScopeRequired` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | REV-01 | unit | `go test ./internal/api/... -run TestHandleDIDCommRevoke` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | REV-02 | unit | `go test ./internal/store/... -run TestCreateRevocation` | ❌ W0 | ⬜ pending |
| 03-02-03 | 02 | 1 | REV-03 | unit | `go test ./internal/store/... -run TestCleanupExpiredRevocations` | ❌ W0 | ⬜ pending |
| 03-02-04 | 02 | 1 | REV-04 | unit | `go test ./internal/api/... -run TestListRevocations` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 2 | TPL-01 | unit | `go test ./internal/approval/... -run TestTemplateMarshalAdaptiveCard` | ❌ W0 | ⬜ pending |
| 03-03-02 | 03 | 2 | TPL-03 | unit | `go test ./internal/approval/... -run TestTemplateFormat` | ❌ W0 | ⬜ pending |
| 03-03-03 | 03 | 2 | TPL-05 | unit | `go test ./internal/approval/... -run TestIsBlockedIP` | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/api/revocations_test.go` — stubs for AUTH-05, REV-04, POST /v1/revocations handler
- [ ] `platform/internal/store/revocations_test.go` — stubs for REV-02, REV-03, APR-14
- [ ] New test cases in `platform/internal/approval/template_test.go` — TPL-01, TPL-03 with Adaptive Card model
- [ ] New test cases in `platform/internal/api/didcomm_handler_test.go` — REV-01 (DIDComm revoke handling)

*Existing tests for deleted files (`api/approvals_test.go`, `store/approvals_test.go`) are deleted as part of the rework.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| DIDComm revoke forwarding reaches via | REV-01 | Requires two-server setup | Send revoke message, verify via's server receives forwarded revocation |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
