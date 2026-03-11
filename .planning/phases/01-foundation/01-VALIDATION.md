---
phase: 1
slug: foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib testing (go test) |
| **Config file** | None needed (Go convention) |
| **Quick run command** | `cd platform && go test ./internal/crypto/ -v -count=1` |
| **Full suite command** | `cd platform && go test ./... -v -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./... -v -count=1`
- **After every plan wave:** Run `cd platform && go test ./... -v -count=1 -race`
- **Before `/gsd:verify-work`:** Full suite must be green + `docker compose up -d && curl localhost:8080/v1/health`
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 0 | CRY-01 | unit | `cd platform && go test ./internal/crypto/ -run TestGenerateKeyPair -v` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 0 | CRY-02 | unit | `cd platform && go test ./internal/crypto/ -run TestCanonicalJSON -v` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 0 | CRY-03 | unit | `cd platform && go test ./internal/crypto/ -run TestSignablePayload -v` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 0 | CRY-04 | unit | `cd platform && go test ./internal/crypto/ -run TestNewChannelID -v` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 0 | TST-03 | unit | `cd platform && go test ./internal/crypto/ -v` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 0 | TST-04 | unit | `cd platform && go test ./internal/crypto/ -run TestToken -v` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | REG-01 | HTTP test | `cd platform && go test ./internal/api/ -run TestRegister -v` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 1 | REG-03 | unit | `cd platform && go test ./internal/crypto/ -run TestNewToken -v` | ❌ W0 | ⬜ pending |
| 01-02-03 | 02 | 1 | REG-04 | HTTP test | `cd platform && go test ./internal/api/ -run TestGetEntity -v` | ❌ W0 | ⬜ pending |
| 01-02-04 | 02 | 1 | AUTH-01 | HTTP test | `cd platform && go test ./internal/api/ -run TestAuthRequired -v` | ❌ W0 | ⬜ pending |
| 01-02-05 | 02 | 1 | ERR-01 | HTTP test | `cd platform && go test ./internal/api/ -run TestErrorFormat -v` | ❌ W0 | ⬜ pending |
| 01-02-06 | 02 | 1 | ERR-02 | HTTP test | `cd platform && go test ./internal/api/ -run TestHealth -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/crypto/crypto_test.go` — stubs for CRY-01, CRY-02, CRY-03, CRY-04, TST-03, TST-04
- [ ] `platform/internal/api/api_test.go` — stubs for REG-01, REG-04, AUTH-01, ERR-01, ERR-02
- [ ] `go mod tidy` — resolve dependencies before tests can run
- [ ] `go get github.com/gowebpki/jcs` — RFC 8785 JCS dependency
- [ ] `go get github.com/golang-migrate/migrate/v4` — migration runner dependency

*Wave 0 creates test stubs that initially fail; implementation waves make them pass.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker Compose starts full stack | INF-01 | Requires Docker runtime | `docker compose up -d && curl localhost:8080/v1/health` |
| Multi-stage Alpine Dockerfile builds | INF-02 | Build verification | `docker build -t atap-platform ./platform` |
| zerolog structured logging | INF-04 | Log format inspection | Run server, check JSON log output |
| Graceful shutdown | INF-05 | Signal handling | Send SIGTERM, verify clean exit |
| Dependencies updated | INF-06 | Build verification | `cd platform && go build ./...` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
