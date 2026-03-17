---
phase: 1
slug: identity-and-auth-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package + testcontainers-go |
| **Config file** | none — table-driven tests in `*_test.go` files next to source |
| **Quick run command** | `cd platform && go test ./internal/... -run TestDID -v` |
| **Full suite command** | `cd platform && go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd platform && go test ./internal/... -run <relevant_prefix> -v -count=1`
- **After every plan wave:** Run `cd platform && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | INF-01 | unit | `go test ./internal/api -run TestOldRoutesGone -v` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | INF-02 | integration | `go test ./internal/store -run TestMigrations -v` | ❌ W0 | ⬜ pending |
| 1-02-01 | 02 | 1 | DID-01 | unit | `go test ./internal/crypto -run TestBuildDID -v` | ❌ W0 | ⬜ pending |
| 1-02-02 | 02 | 1 | DID-02 | unit | `go test ./internal/api -run TestResolveDID -v` | ❌ W0 | ⬜ pending |
| 1-02-03 | 02 | 1 | DID-03 | unit | `go test ./internal/api -run TestResolveDIDContext -v` | ❌ W0 | ⬜ pending |
| 1-02-04 | 02 | 1 | DID-05 | unit | `go test ./internal/crypto -run TestDeriveHumanID -v` | ✅ | ⬜ pending |
| 1-02-05 | 02 | 2 | DID-07 | unit | `go test ./internal/store -run TestKeyRotation -v` | ❌ W0 | ⬜ pending |
| 1-03-01 | 03 | 2 | AUTH-01, AUTH-04 | unit | `go test ./internal/api -run TestDPoP -v` | ❌ W0 | ⬜ pending |
| 1-03-02 | 03 | 2 | AUTH-02 | integration | `go test ./internal/api -run TestClientCredentials -v` | ❌ W0 | ⬜ pending |
| 1-03-03 | 03 | 2 | AUTH-03 | integration | `go test ./internal/api -run TestAuthCode -v` | ❌ W0 | ⬜ pending |
| 1-03-04 | 03 | 2 | AUTH-05 | unit | `go test ./internal/api -run TestScopeEnforcement -v` | ❌ W0 | ⬜ pending |
| 1-03-05 | 03 | 2 | AUTH-06 | unit | `go test ./internal/api -run TestTokenExpiry -v` | ❌ W0 | ⬜ pending |
| 1-04-01 | 01 | 1 | SRV-01 | unit | `go test ./internal/api -run TestDiscovery -v` | ❌ W0 | ⬜ pending |
| 1-04-02 | 01 | 1 | API-01 | unit | `go test ./internal/api -run TestCreateEntity -v` | ❌ W0 | ⬜ pending |
| 1-04-03 | 01 | 1 | API-06 | unit | `go test ./internal/api -run TestProblemDetail -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `platform/internal/crypto/did_test.go` — stubs for DID-01: DID construction, multibase encoding
- [ ] `platform/internal/api/did_test.go` — stubs for DID-02, DID-03: DID Document structure and routing
- [ ] `platform/internal/api/oauth_test.go` — stubs for AUTH-01 through AUTH-06: token flows
- [ ] `platform/internal/api/discovery_test.go` — stubs for SRV-01: discovery document
- [ ] `platform/internal/api/entities_test.go` — stubs for API-01: entity CRUD
- [ ] `platform/internal/store/migrations_test.go` — stubs for INF-02: migration correctness

*No framework install needed — `go test` is built-in; testcontainers-go already in go.mod*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| DID Document resolvable by external client | DID-02 | Requires real HTTPS endpoint | Deploy to staging, resolve with `curl` or universal DID resolver |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
