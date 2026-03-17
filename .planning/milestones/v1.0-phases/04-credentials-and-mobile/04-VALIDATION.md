---
phase: 4
slug: credentials-and-mobile
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-14
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (Go)** | go test |
| **Framework (Flutter)** | flutter test |
| **Quick run (Go)** | `cd platform && go test ./internal/credential/... ./internal/store/... -run "TestCredential\|TestCryptoShred\|TestDelegate" -v -count=1` |
| **Quick run (Flutter)** | `cd mobile && flutter test` |
| **Full suite** | `cd platform && go test ./... -count=1` |
| **Estimated runtime** | ~25 seconds (Go), ~15 seconds (Flutter) |

---

## Sampling Rate

- **After every task commit:** Run quick run command for the relevant stack
- **After every plan wave:** Run full suite for the relevant stack
- **Before `/gsd:verify-work`:** Both full suites must be green
- **Max feedback latency:** 25 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | CRD-01, CRD-02 | unit | `go test ./internal/credential/ -run TestIssue -v` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | CRD-03, CRD-04 | unit | `go test ./internal/credential/ -run TestTrust -v` | ❌ W0 | ⬜ pending |
| 04-02-01 | 02 | 1 | PRV-01, PRV-02 | integration | `go test ./internal/store/ -run TestCryptoShred -v` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 1 | MSG-06 | integration | `go test ./internal/api/ -run TestDelegate -v` | ❌ W0 | ⬜ pending |
| 04-03-01 | 03 | 2 | CRD-05, CRD-06, API-04 | integration | `go test ./internal/api/ -run TestCredential -v` | ❌ W0 | ⬜ pending |
| 04-04-01 | 04 | 3 | MOB-01, MOB-02 | widget | `cd mobile && flutter test test/onboarding_test.dart` | ❌ W0 | ⬜ pending |
| 04-04-02 | 04 | 3 | MOB-03, MOB-04 | widget | `cd mobile && flutter test test/inbox_test.dart` | ❌ W0 | ⬜ pending |
| 04-04-03 | 04 | 3 | MOB-05, MOB-06 | widget | `cd mobile && flutter test test/approval_test.dart` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/credential/credential_test.go` — stubs for CRD-01 through CRD-04
- [ ] `platform/internal/api/credentials_handler_test.go` — stubs for CRD-05, CRD-06, API-04
- [ ] `platform/internal/store/crypto_shred_test.go` — stubs for PRV-01 through PRV-04
- [ ] `mobile/test/onboarding_test.dart` — stubs for MOB-01, MOB-02
- [ ] `mobile/test/inbox_test.dart` — stubs for MOB-03, MOB-04
- [ ] `mobile/test/approval_test.dart` — stubs for MOB-05, MOB-06

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Biometric prompt on iOS/Android | MOB-05 | Requires physical device | Run app on device, attempt approval, verify biometric dialog |
| Secure enclave key generation | MOB-01 | Simulator has limited enclave support | Test on physical device with Secure Enclave |
| Push notification delivery | MOB-04 | Requires FCM/APNs setup | Configure Firebase, send test DIDComm message, verify push |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 25s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
