---
phase: 3
slug: mobile-app
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go: `go test` with testcontainers-go; Flutter: `flutter test` |
| **Config file** | Go: existing; Flutter: none yet (Wave 0) |
| **Quick run command** | `cd platform && go test ./internal/push/... -v -count=1` |
| **Full suite command** | `cd platform && go test -tags integration ./... && cd ../mobile && flutter test` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./...`
- **After every plan wave:** Run `cd platform && go test -tags integration ./... && cd ../mobile && flutter test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | MOB-01 | integration (manual) | Manual: run app on device/emulator, tap register | No - Wave 0 | ⬜ pending |
| 03-01-02 | 01 | 1 | MOB-02 | widget test | `cd mobile && flutter test test/inbox_test.dart` | No - Wave 0 | ⬜ pending |
| 03-01-03 | 01 | 1 | MOB-03 | integration | `cd platform && go test -tags integration ./internal/api -run TestPushToken` | No - Wave 0 | ⬜ pending |
| 03-01-04 | 01 | 1 | MOB-04 | unit | `cd platform && go test ./internal/push -run TestSendNotification` | No - Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `mobile/` — Flutter project creation (`flutter create`)
- [ ] `mobile/test/` — Flutter test infrastructure
- [ ] `platform/internal/push/push_test.go` — Push notification unit tests
- [ ] `platform/internal/api/claims_test.go` — Claims endpoint tests
- [ ] Ed25519 cross-language compatibility test (Dart generates, Go verifies)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Flutter app registers agent via API | MOB-01 | Requires device/emulator with UI interaction | 1. Open app 2. Tap register 3. Verify success confirmation |
| Push notification delivery in background | MOB-04 | Requires real device with FCM/APNs | 1. Background the app 2. Send signal to entity inbox 3. Verify push notification appears |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
