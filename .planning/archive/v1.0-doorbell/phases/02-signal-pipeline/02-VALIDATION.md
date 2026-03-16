---
phase: 2
slug: signal-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testcontainers-go v0.35+ |
| **Config file** | None — Go's built-in test runner |
| **Quick run command** | `cd platform && go test ./internal/... -count=1` |
| **Full suite command** | `cd platform && go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds (unit), ~120 seconds (with testcontainers) |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/... -count=1`
- **After every plan wave:** Run `cd platform && go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | SIG-01 | unit + integration | `cd platform && go test ./internal/api -run TestSendSignal -count=1` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | SIG-02 | unit | `cd platform && go test ./internal/api -run TestSignalFormat -count=1` | ❌ W0 | ⬜ pending |
| 02-01-03 | 01 | 1 | SIG-03 | integration | `cd platform && go test ./test -run TestSignalPersistence -count=1` | ❌ W0 | ⬜ pending |
| 02-01-04 | 01 | 1 | SIG-04 | unit + integration | `cd platform && go test ./internal/api -run TestInboxPagination -count=1` | ❌ W0 | ⬜ pending |
| 02-01-05 | 01 | 1 | SIG-05 | unit | `cd platform && go test ./internal/api -run TestExpiredExcluded -count=1` | ❌ W0 | ⬜ pending |
| 02-01-06 | 01 | 1 | SIG-06 | integration | `cd platform && go test ./test -run TestIdempotency -count=1` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | SSE-01 | integration | `cd platform && go test ./test -run TestSSEStream -count=1` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | SSE-02 | integration | `cd platform && go test ./test -run TestSSEReplay -count=1` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 2 | SSE-03 | unit | `cd platform && go test ./internal/api -run TestSSEHeartbeat -count=1` | ❌ W0 | ⬜ pending |
| 02-02-04 | 02 | 2 | SSE-04 | integration | `cd platform && go test ./test -run TestWriteThenNotify -count=1` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 3 | WHK-01 | integration | `cd platform && go test ./test -run TestWebhookDelivery -count=1` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 3 | WHK-02 | unit | `cd platform && go test ./internal/api -run TestWebhookSignature -count=1` | ❌ W0 | ⬜ pending |
| 02-03-03 | 03 | 3 | WHK-03 | unit | `cd platform && go test ./internal/api -run TestWebhookRetry -count=1` | ❌ W0 | ⬜ pending |
| 02-03-04 | 03 | 3 | WHK-04 | integration | `cd platform && go test ./test -run TestWebhookMaxRetries -count=1` | ❌ W0 | ⬜ pending |
| 02-04-01 | 04 | 3 | CHN-01 | unit + integration | `cd platform && go test ./internal/api -run TestCreateChannel -count=1` | ❌ W0 | ⬜ pending |
| 02-04-02 | 04 | 3 | CHN-02 | unit | `cd platform && go test ./internal/api -run TestChannelWebhookURL -count=1` | ❌ W0 | ⬜ pending |
| 02-04-03 | 04 | 3 | CHN-03 | unit + integration | `cd platform && go test ./internal/api -run TestInboundWebhook -count=1` | ❌ W0 | ⬜ pending |
| 02-04-04 | 04 | 3 | CHN-04 | unit | `cd platform && go test ./internal/api -run TestChannelListRevoke -count=1` | ❌ W0 | ⬜ pending |
| 02-04-05 | 04 | 3 | CHN-05 | unit | `cd platform && go test ./internal/crypto -run TestChannelIDEntropy -count=1` | ❌ W0 | ⬜ pending |
| 02-05-01 | 05 | 4 | TST-01 | integration | `cd platform && go test ./test -run TestFullLifecycle -count=1` | ❌ W0 | ⬜ pending |
| 02-05-02 | 05 | 4 | TST-02 | integration | `cd platform && go test ./test -run Test -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/test/integration_test.go` — testcontainers-go setup + shared fixtures (TST-01, TST-02)
- [ ] testcontainers-go dependency: `go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres github.com/testcontainers/testcontainers-go/modules/redis`
- [ ] Extend `fakeStore` in `api_test.go` with SignalStore + ChannelStore methods
- [ ] Migrations `002_signals.up.sql`, `003_channels.up.sql`, `004_webhook_delivery.up.sql`

*All phase requirements need Wave 0 test stubs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SSE heartbeat timing | SSE-03 | Exact 30s interval hard to assert deterministically | Connect to SSE stream, observe heartbeat comments arrive ~30s apart |

*All other behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
