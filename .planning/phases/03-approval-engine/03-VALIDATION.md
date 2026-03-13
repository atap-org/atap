---
phase: 3
slug: approval-engine
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `cd platform && go test ./internal/approval/... ./internal/api/... -run TestApproval -v -count=1` |
| **Full suite command** | `cd platform && go test ./... -count=1` |
| **Estimated runtime** | ~20 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/approval/... -v -count=1`
- **After every plan wave:** Run `cd platform && go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | APR-01, APR-02 | unit | `go test ./internal/approval/ -run TestApproval -v` | ❌ W0 | ⬜ pending |
| 03-01-02 | 01 | 1 | APR-03, APR-04 | unit | `go test ./internal/approval/ -run TestSign -v` | ❌ W0 | ⬜ pending |
| 03-02-01 | 02 | 1 | APR-05, APR-06 | unit | `go test ./internal/approval/ -run TestLifecycle -v` | ❌ W0 | ⬜ pending |
| 03-02-02 | 02 | 1 | TPL-01, TPL-02 | unit | `go test ./internal/approval/ -run TestTemplate -v` | ❌ W0 | ⬜ pending |
| 03-03-01 | 03 | 2 | API-03 | integration | `go test ./internal/api/ -run TestApproval -v` | ❌ W0 | ⬜ pending |
| 03-03-02 | 03 | 2 | APR-07, APR-08 | integration | `go test ./internal/api/ -run TestApprovalFlow -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/approval/approval_test.go` — stubs for APR-01 through APR-04 (model, signing, verification)
- [ ] `platform/internal/approval/lifecycle_test.go` — stubs for APR-05 through APR-08 (state machine, TTL, revocation)
- [ ] `platform/internal/approval/template_test.go` — stubs for TPL-01 through TPL-06 (template fetch, verify, SSRF)
- [ ] `platform/internal/api/approval_handler_test.go` — stubs for API-03 (HTTP endpoints)
- [ ] `platform/internal/store/approvals_test.go` — stubs for APR-09 through APR-12 (persistence, query)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Template rendering with brand colors | TPL-03 | Visual rendering is client-side | Verify template JSON produces correct field mapping |
| Cross-server approval via DIDComm | APR-07 | Requires two server instances | Deploy two local instances, run full approval flow |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 20s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
