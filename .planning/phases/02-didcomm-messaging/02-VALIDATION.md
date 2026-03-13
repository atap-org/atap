---
phase: 2
slug: didcomm-messaging
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — standard Go testing |
| **Quick run command** | `cd platform && go test ./internal/didcomm/...` |
| **Full suite command** | `cd platform && go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/didcomm/...`
- **After every plan wave:** Run `cd platform && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | MSG-01 | unit | `go test ./internal/didcomm/ -run TestEnvelope` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | MSG-04 | unit | `go test ./internal/didcomm/ -run TestAuthcrypt` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | MSG-02 | integration | `go test ./internal/api/ -run TestDIDCommSend` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | MSG-03 | integration | `go test ./internal/store/ -run TestMessageQueue` | ❌ W0 | ⬜ pending |
| 02-02-03 | 02 | 2 | MSG-05 | integration | `go test ./internal/api/ -run TestProtocolRouting` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/didcomm/didcomm_test.go` — stubs for MSG-01, MSG-04 (envelope, authcrypt)
- [ ] `platform/internal/didcomm/protocol_test.go` — stubs for MSG-05 (protocol types)
- [ ] `platform/internal/api/didcomm_handler_test.go` — stubs for MSG-02, API-05 (HTTP endpoint)
- [ ] `platform/internal/store/message_store_test.go` — stubs for MSG-03 (queue/offline delivery)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cross-server DIDComm delivery | MSG-02 (federation) | Requires two server instances | Deploy two local instances, send message between them |

*Note: Federation is Phase 2 scope only if cross-server is required; otherwise automated tests suffice.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
