---
phase: 5
slug: mobile-push-notifications
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-12
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | flutter_test (built-in) |
| **Config file** | none (uses pubspec.yaml test section) |
| **Quick run command** | `cd mobile && flutter test test/push_provider_test.dart` |
| **Full suite command** | `cd mobile && flutter test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd mobile && flutter test`
- **After every plan wave:** Run `cd mobile && flutter test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | MOB-03 | unit | `cd mobile && flutter test test/push_provider_test.dart` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | MOB-03 | unit | `cd mobile && flutter test test/push_provider_test.dart` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | MOB-02 | widget | `cd mobile && flutter test test/inbox_widget_test.dart` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `mobile/test/push_provider_test.dart` — covers MOB-03 (token registration call, platform detection, permission denied state)
- Note: Firebase initialization cannot be unit tested without Firebase project — test PushNotifier state transitions with mocked dependencies

*Existing `mobile/test/inbox_widget_test.dart` covers MOB-02 (inbox display with pull-to-refresh).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Push notification arrives on device in background | MOB-03 | Requires real FCM project + physical device | 1. Send signal via API 2. Verify notification appears on device 3. Tap notification 4. Verify signal detail screen opens |
| Badge count clears on inbox open | MOB-03 | Requires real device badge API | 1. Send signal while app is closed 2. Check app badge shows count 3. Open app to inbox 4. Verify badge clears |
| Cold start navigation from notification tap | MOB-03 | Requires app termination + real push | 1. Force quit app 2. Tap pending notification 3. Verify app opens to signal detail screen |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
