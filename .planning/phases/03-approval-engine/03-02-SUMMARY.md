---
phase: 03-approval-engine
plan: "02"
subsystem: approval
tags: [jws, eddsa, ssrf, state-machine, template, crypto]
dependency_graph:
  requires: ["03-01"]
  provides: ["03-03"]
  affects: ["platform/internal/approval", "platform/internal/config"]
tech_stack:
  added:
    - "github.com/go-jose/go-jose/v4 (promoted from indirect to direct)"
  patterns:
    - "JWS Compact Serialization with detached payload (header..signature)"
    - "APR-12 kid-driven DID validation before cryptographic verification"
    - "SSRF prevention via custom DialContext with DNS-then-connect pattern"
    - "JCS (RFC 8785) canonical JSON for all signing payloads"
key_files:
  created:
    - platform/internal/approval/signer.go
    - platform/internal/approval/signer_test.go
    - platform/internal/approval/lifecycle.go
    - platform/internal/approval/lifecycle_test.go
    - platform/internal/approval/template.go
    - platform/internal/approval/template_test.go
  modified:
    - platform/internal/config/config.go
decisions:
  - "approvalWithoutSignatures uses JSON marshal/unmarshal round-trip to map and delete key -- avoids struct mutation and handles all json tags naturally"
  - "VerifyApprovalSignature re-attaches payload before calling jose.ParseSigned -- go-jose v4 requires non-detached JWS for parsing"
  - "IsBlockedIP exported for direct testing of SSRF IP validation logic"
  - "SignTemplateProof helper added alongside VerifyTemplateProof to enable test round-trips"
  - "MaxApprovalTTL in Config parsed from MAX_APPROVAL_TTL env var, fallback 2160h (90 days)"
metrics:
  duration_minutes: 8
  completed_date: "2026-03-13"
  tasks_completed: 2
  files_created: 6
  files_modified: 1
---

# Phase 03 Plan 02: Approval Domain Logic Summary

**One-liner:** JWS detached-payload signing with EdDSA+kid, approval lifecycle state machine, and SSRF-safe template fetch with JWS proof verification for the approval engine cryptographic core.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | JWS signing engine + approval lifecycle state machine | d3bf12a | signer.go, signer_test.go, lifecycle.go, lifecycle_test.go |
| 2 | Template fetch with SSRF prevention + JWS proof verification | 5e21cf4 | template.go, template_test.go, config.go |

## What Was Built

### Task 1: JWS Signing Engine + Lifecycle State Machine

**signer.go:**
- `SignApproval(a, privateKey, keyID) string` — produces detached JWS compact (`header..signature`) using EdDSA algorithm with kid header set to the fully-qualified DID URL. Payload is JCS of the approval with signatures field excluded per spec §8.8.
- `VerifyApprovalSignature(a, jwsToken, expectedKID, resolvedPubKey) error` — implements APR-12: extracts kid from JWS protected header, validates against expectedKID before cryptographic verification. Re-attaches detached payload for go-jose parsing.
- `approvalWithoutSignatures(a) map[string]any` — internal helper that round-trips through JSON to produce a map without the signatures field, preserving all json tags without mutating the original struct.

**lifecycle.go:**
- `ValidateTransition(currentState, nextState) error` — enforces spec §8.3 state machine. Allowed: requested→{approved,declined,expired,rejected}, approved→{consumed,revoked}. Terminal states produce error on any transition.
- `IsTerminalState(state) bool` — true for declined, expired, rejected, consumed, revoked.
- `ClampValidUntil(validUntil, maxTTL) *time.Time` — nil passthrough for one-time approvals, otherwise min(validUntil, now+maxTTL).

### Task 2: Template Fetch + SSRF Prevention

**template.go:**
- `IsBlockedIP(ip) bool` — blocks RFC 1918 (via `ip.IsPrivate()`), loopback (via `ip.IsLoopback()`), link-local unicast (via `ip.IsLinkLocalUnicast()`), and explicit 169.254.x.x check. IPv6 loopback (::1) covered by `IsLoopback()`.
- `FetchTemplate(ctx, templateURL) (*Template, error)` — HTTPS-only, no redirects (returns `ErrUseLastResponse`), 5s timeout, 64KB body limit. Custom transport resolves DNS once, validates each IP, connects directly to first non-blocked IP (prevents DNS rebinding).
- `VerifyTemplateProof(tmpl, viaPubKey) error` — verifies JWS proof over JCS-serialized template excluding proof field. Re-attaches detached payload for go-jose.
- `SignTemplateProof(tmpl, privateKey, keyID) error` — signs template and sets Proof.KID/Alg/Sig fields.
- Returns nil, nil for empty URL (two-party approval with fallback rendering).

**config.go:**
- `MaxApprovalTTL time.Duration` field added to Config struct.
- `MAX_APPROVAL_TTL` env var, default `"2160h"` (90 days). Discovery endpoint publishes `"P90D"` as ISO 8601.

## Test Coverage

21 tests, all passing:

**Signer tests (7):** TestSignApproval, TestSignApprovalKID, TestVerifyApprovalSignature, TestVerifyApprovalSignatureKIDMismatch, TestVerifyApprovalSignatureWrongKey, TestVerifyApprovalSignatureMutatedDoc, TestCanonicalPayloadExcludesSignatures

**Lifecycle tests (5):** TestValidTransitions, TestInvalidTransitions, TestIsTerminalState, TestClampValidUntil, TestClampValidUntilNil

**Template tests (9):** TestFetchTemplateRejectsHTTP, TestFetchTemplateRejectsRedirect, TestSSRFBlocksRFC1918, TestSSRFBlocksLoopback, TestSSRFBlocksLinkLocal, TestSSRFBlocksIPv6Loopback, TestVerifyTemplateProof, TestVerifyTemplateProofWrongKey, TestFallbackRendering

## Verification Results

- `go test ./internal/approval/... -v -count=1` — 21/21 PASS
- `go build ./...` — succeeds, no errors
- Detached JWS format verified: parts[1] == "" (empty middle segment)
- kid validation confirmed: `TestVerifyApprovalSignatureKIDMismatch` triggers before crypto check
- All terminal state transitions blocked: `TestInvalidTransitions` (11 cases)
- SSRF: 169.254.169.254 confirmed blocked by `TestSSRFBlocksLinkLocal`

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED
