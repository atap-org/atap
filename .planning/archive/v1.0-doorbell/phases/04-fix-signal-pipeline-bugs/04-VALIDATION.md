---
phase: 4
slug: fix-signal-pipeline-bugs
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-12
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (Go)** | Go stdlib `testing` + `httptest` |
| **Framework (Dart)** | `flutter_test` |
| **Config file (Go)** | None — `go test ./...` |
| **Config file (Dart)** | `mobile/pubspec.yaml` (already configured) |
| **Quick run command (Go)** | `cd platform && go test ./internal/api/ -count=1` |
| **Quick run command (Dart)** | `cd mobile && flutter test test/signal_model_test.dart` |
| **Full suite command** | `cd platform && go test ./... && cd ../mobile && flutter test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/api/ -count=1`
- **After every plan wave:** Run `cd platform && go test ./... && cd ../mobile && flutter test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | SSE-01 | unit (Dart) | `cd mobile && flutter test test/signal_model_test.dart` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | SIG-04 | unit (Dart) | `cd mobile && flutter test test/inbox_provider_test.dart` | ❌ W0 | ⬜ pending |
| 04-01-03 | 01 | 1 | WHK-03 | unit (Go) | `cd platform && go test ./internal/api/ -run TestPollRetries` | ❌ W0 | ⬜ pending |
| 04-01-04 | 01 | 1 | WHK-03 | unit (Go) | `cd platform && go test ./internal/api/ -run TestClaimRedemption409` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `mobile/test/signal_model_test.dart` — Signal.fromJson parses `ts` field
- [ ] `mobile/test/inbox_provider_test.dart` — loadMore sends `?after=` param
- [ ] Go tests in `platform/internal/api/api_test.go` — webhook retry payload + claim 409

*All test files are new — Wave 0 creates them alongside bug fixes.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
