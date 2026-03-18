---
phase: 5
slug: api-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) |
| **Config file** | None — Go tests are file-adjacent (`*_test.go`) |
| **Quick run command** | `cd platform && go test ./internal/api/ -run TestRateLimit -v` |
| **Full suite command** | `cd platform && go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/api/ -run TestRateLimit -v`
- **After every plan wave:** Run `cd platform && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_PublicExceeded -v` | ❌ W0 | ⬜ pending |
| 05-01-02 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_AuthExceeded -v` | ❌ W0 | ⬜ pending |
| 05-01-03 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_WithinLimit -v` | ❌ W0 | ⬜ pending |
| 05-01-04 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_Headers -v` | ❌ W0 | ⬜ pending |
| 05-01-05 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_Allowlist -v` | ❌ W0 | ⬜ pending |
| 05-01-06 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_HealthExempt -v` | ❌ W0 | ⬜ pending |
| 05-01-07 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_WellKnownExempt -v` | ❌ W0 | ⬜ pending |
| 05-01-08 | 01 | 1 | API-07 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_RedisDown -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/api/ratelimit_test.go` — stubs for all API-07 test cases
- [ ] `platform/migrations/015_rate_limit_config.up.sql` — schema and seed data
- [ ] `platform/migrations/015_rate_limit_config.down.sql` — teardown

*Existing Go `testing` infrastructure covers all phase requirements.*

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
