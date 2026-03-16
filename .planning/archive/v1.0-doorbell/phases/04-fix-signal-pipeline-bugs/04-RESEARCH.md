# Phase 4: Fix Signal Pipeline Bugs - Research

**Researched:** 2026-03-12
**Domain:** Cross-language bug fixes (Go backend + Flutter mobile)
**Confidence:** HIGH

## Summary

Phase 4 addresses four discrete, well-localized bugs in the signal pipeline. All bugs have known root causes, known file locations, and straightforward fixes. No new features, no schema changes, no architectural changes.

The bugs span two codebases (Go platform, Flutter mobile) but are independent of each other -- they can be fixed in any order. The webhook retry bug requires a minor wiring change (adding SignalStore access to WebhookWorker), while the other three are one-to-three line fixes.

**Primary recommendation:** Fix all four bugs in a single plan with one task per bug, each including a targeted unit test. The fixes are simple enough that a single wave is sufficient.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Flutter `Signal.fromJson` uses the existing `ts` field (already in JSON) instead of `created_at`
- `created_at` stays as `json:"-"` in Go -- no API change needed
- `ts` and `created_at` are effectively duplicate data -- one authoritative timestamp is cleaner
- `pollRetries()` fetches the signal from the signal store using `SignalID` before enqueueing the retry job
- One DB read per retry -- simple, always fresh, small memory footprint
- If signal is expired or deleted by retry time: skip silently, log a warning, mark attempt as failed
- No schema changes to `webhook_delivery_attempts` table
- Handler checks for `ErrClaimNotAvailable` sentinel error and returns 409 Conflict (not 500)
- No structural changes -- just error type checking in the existing handler
- Flutter `loadMore()` changes `?cursor=` to `?after=` to match Go backend's `c.Query("after", "")`
- One-line fix in `inbox_provider.dart`
- One targeted unit test per bug fix (4 Go tests)
- Plus one Dart unit test for the Flutter pagination fix (mock API client, assert `?after=` param)
- No integration tests -- focused unit tests are sufficient for a bug-fix phase

### Claude's Discretion
- Exact test structure and assertions
- Whether to refactor webhook worker to accept a signal store interface or pass it through existing wiring
- Go test helper patterns for claim concurrency test (goroutines vs sequential with pre-redeemed claim)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SIG-04 | Inbox supports cursor-based pagination via `GET /v1/inbox/{entity-id}?after={cursor}&limit=50` | Flutter sends `?cursor=` but Go expects `?after=`; fix is in `inbox_provider.dart:107`. Go side already works correctly. |
| SSE-01 | Entity can open SSE stream and receive signals in real-time | Flutter `Signal.fromJson` parses `created_at` but Go sends `ts`; fixing the Dart model unblocks signal display in inbox/SSE. |
| WHK-03 | Failed webhooks retry with exponential backoff, max 5 attempts | `pollRetries()` enqueues retry jobs without payload; fix requires adding SignalStore to WebhookWorker and fetching signal before enqueue. |
</phase_requirements>

## Standard Stack

### Core (no changes needed)
| Library | Version | Purpose | Already In Use |
|---------|---------|---------|----------------|
| Go stdlib | 1.22+ | Backend language | Yes |
| Fiber v2 | latest | HTTP framework | Yes |
| pgx/v5 | 5.7+ | PostgreSQL driver | Yes |
| zerolog | 1.34+ | Structured logging | Yes |
| Flutter | 3.x | Mobile framework | Yes |
| Dart test | latest | Flutter unit tests | Yes |

No new libraries are needed for this phase. All fixes use existing dependencies.

## Architecture Patterns

### Bug Fix Pattern: Each Fix is Self-Contained

All four bugs are independent. Each fix touches 1-2 files plus a test file.

### Pattern 1: Signal Timestamp Contract
**What:** Go `Signal` struct has `TS time.Time json:"ts"` (line 197) and `CreatedAt time.Time json:"-"` (line 207). The JSON output contains `ts`, not `created_at`.
**Current bug:** Flutter `Signal.fromJson` (line 33) parses `json['created_at']` which is null/missing in the JSON.
**Fix:** Change Flutter to parse `json['ts']` instead of `json['created_at']`.

```dart
// Before (broken):
createdAt: DateTime.parse(json['created_at'] as String),

// After (fixed):
createdAt: DateTime.parse(json['ts'] as String),
```

Also update `toJson()` to emit `ts` for consistency:
```dart
// Before:
'created_at': createdAt.toIso8601String(),
// After:
'ts': createdAt.toIso8601String(),
```

**File:** `mobile/lib/core/models/signal.dart` lines 33, 43

### Pattern 2: Pagination Parameter Alignment
**What:** Go inbox handler uses `c.Query("after", "")` (api.go line 467). Flutter sends `?cursor=`.
**Fix:** One-line change in Flutter `loadMore()`.

```dart
// Before (broken):
'/v1/inbox/$entityId?limit=50&cursor=${state.cursor}',

// After (fixed):
'/v1/inbox/$entityId?limit=50&after=${state.cursor}',
```

**File:** `mobile/lib/providers/inbox_provider.dart` line 107

### Pattern 3: Webhook Retry Payload Fetch
**What:** `WebhookWorker.pollRetries()` creates `WebhookJob` without `Payload` (webhook.go lines 190-196). The `deliver()` method signs `job.Payload` (line 94), so empty payload means empty HTTP body.
**Fix:** Add `SignalStore` to `WebhookWorker`, fetch signal in `pollRetries()`, marshal to JSON for payload.

```go
// WebhookWorker needs a signalStore field
type WebhookWorker struct {
    queue       chan WebhookJob
    httpClient  *http.Client
    store       WebhookStore
    signalStore SignalStore   // NEW
    platformKey ed25519.PrivateKey
    log         zerolog.Logger
}

// In pollRetries:
sig, err := w.signalStore.GetSignal(ctx, a.SignalID)
if err != nil {
    w.log.Warn().Err(err).Str("signal_id", a.SignalID).Msg("signal not found for retry, skipping")
    continue
}
payload, _ := json.Marshal(sig)
w.Enqueue(WebhookJob{
    SignalID:   a.SignalID,
    WebhookURL: a.WebhookURL,
    Attempt:    a.Attempt + 1,
    Payload:    payload,
})
```

**Files:** `platform/internal/api/webhook.go` (struct, constructor, pollRetries), `platform/cmd/server/main.go` line 139 (pass signal store), `platform/test/integration_test.go` line 171 (pass signal store)

### Pattern 4: Claim Redemption Error Handling
**What:** `human.go:70` catches `RedeemClaim` error but always returns 500. The store already returns `store.ErrClaimNotAvailable` sentinel when claim is already redeemed.
**Fix:** Check error type before deciding status code.

```go
if err := h.claimStore.RedeemClaim(c.Context(), req.ClaimCode, entity.ID); err != nil {
    if errors.Is(err, store.ErrClaimNotAvailable) {
        return problem(c, 409, "claim_already_redeemed", "Claim has already been redeemed", "")
    }
    h.log.Error().Err(err).Str("code", req.ClaimCode).Msg("failed to redeem claim")
    return problem(c, 500, "store_error", "Failed to redeem claim", "")
}
```

**File:** `platform/internal/api/human.go` line 70-72

### Anti-Patterns to Avoid
- **Do not add `created_at` to Go JSON output:** The decision is to use `ts` only. Adding `created_at` to JSON would create a redundant field.
- **Do not cache signal payloads in webhook_delivery_attempts:** The decision is to fetch fresh from signal store per retry. Keeps the table schema unchanged.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal JSON serialization | Custom serializer | Go struct tags (`json:"ts"`) | Already works correctly |
| Error type checking | String matching on error messages | `errors.Is(err, sentinel)` | Go idiom, future-proof |
| Test HTTP assertions | Manual response parsing | `httptest` + fiber `app.Test()` | Already used in `api_test.go` |

## Common Pitfalls

### Pitfall 1: Forgetting to update toJson in Flutter Signal model
**What goes wrong:** Fix `fromJson` to read `ts` but leave `toJson` writing `created_at`, creating asymmetric serialization.
**How to avoid:** Update both `fromJson` (line 33) and `toJson` (line 43) in `signal.dart`.

### Pitfall 2: WebhookWorker constructor signature change breaking callers
**What goes wrong:** Adding `SignalStore` parameter to `NewWebhookWorker` breaks `cmd/server/main.go` and `test/integration_test.go`.
**How to avoid:** Update all three call sites: constructor definition, main.go, integration_test.go. Consider setter injection (`SetSignalStore`) to minimize signature changes, but constructor injection is cleaner.

### Pitfall 3: Race condition in claim redemption test
**What goes wrong:** Using goroutines for concurrent claim test creates flaky tests.
**How to avoid:** Simpler approach: pre-redeem the claim in test setup, then attempt a second redemption and assert 409. No concurrency needed to test the error path.

### Pitfall 4: Nil signal in webhook retry
**What goes wrong:** Signal may have been deleted or expired between initial delivery and retry. `GetSignal` returns nil/error.
**How to avoid:** Decision says skip silently, log warning. Must handle this gracefully in `pollRetries`.

## Code Examples

### Existing Test Pattern (from api_test.go)
```go
// fakeStore already implements all store interfaces
// Tests create a fakeStore, set up test data, create Handler, use fiber app.Test()

func TestSomething(t *testing.T) {
    fs := newFakeStore()
    // ... seed data ...
    h := NewHandler(fs, fs, fs, fs, nil, platformPriv, cfg, zerolog.Nop())
    app := fiber.New()
    SetupRoutes(app, h)

    req := httptest.NewRequest(http.MethodPost, "/v1/...", body)
    // ... set auth headers ...
    resp, _ := app.Test(req)
    // assert resp.StatusCode
}
```

### fakeStore ErrClaimNotAvailable (already exists in api_test.go line 281)
```go
func (f *fakeStore) RedeemClaim(_ context.Context, code string, redeemedBy string) error {
    // ... existing logic returns store.ErrClaimNotAvailable when already redeemed
    return store.ErrClaimNotAvailable
}
```

## State of the Art

No version changes or migrations needed. All bugs are in current code.

| Bug | Root Cause | Fix Complexity |
|-----|-----------|----------------|
| Signal timestamp | Flutter reads wrong JSON field | 2-line change |
| Pagination param | Flutter sends wrong query param | 1-line change |
| Webhook retry payload | Missing signal fetch in retry loop | ~15 lines + constructor change |
| Claim 409 | Missing sentinel error check | 4-line change |

## Open Questions

1. **WebhookWorker constructor vs setter injection for SignalStore**
   - What we know: Constructor injection is cleaner but changes the signature at 2 call sites. Setter injection (`SetSignalStore`) matches the existing `SetWebhookWorker` pattern on Handler.
   - Recommendation: Constructor injection -- only 2 call sites to update, and it makes the dependency explicit. Both callers already pass `db` which satisfies `SignalStore`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework (Go) | Go stdlib `testing` + `httptest` |
| Framework (Dart) | `flutter_test` |
| Config file (Go) | None needed -- `go test ./...` |
| Config file (Dart) | `mobile/pubspec.yaml` (already configured) |
| Quick run command (Go) | `cd platform && go test ./internal/api/ -run TestName -x` |
| Quick run command (Dart) | `cd mobile && flutter test test/inbox_provider_test.dart` |
| Full suite command | `cd platform && go test ./... && cd ../mobile && flutter test` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SIG-04 | Flutter sends `?after=` param for pagination | unit (Dart) | `cd mobile && flutter test test/inbox_provider_test.dart` | No -- Wave 0 |
| SSE-01 | Flutter `Signal.fromJson` parses `ts` field | unit (Dart) | `cd mobile && flutter test test/signal_model_test.dart` | No -- Wave 0 |
| WHK-03 | `pollRetries` fetches signal payload before enqueue | unit (Go) | `cd platform && go test ./internal/api/ -run TestPollRetriesFetchesPayload` | No -- Wave 0 |
| WHK-03 | `pollRetries` skips missing/expired signals | unit (Go) | `cd platform && go test ./internal/api/ -run TestPollRetriesSkipsMissing` | No -- Wave 0 |
| (bonus) | Claim redemption returns 409 when already redeemed | unit (Go) | `cd platform && go test ./internal/api/ -run TestClaimRedemption409` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./internal/api/ -count=1`
- **Per wave merge:** Full suite `cd platform && go test ./... && cd ../mobile && flutter test`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `mobile/test/signal_model_test.dart` -- covers SIG-04/SSE-01 (Signal.fromJson parsing)
- [ ] `mobile/test/inbox_provider_test.dart` -- covers SIG-04 (pagination param)
- [ ] Go tests added to existing `platform/internal/api/api_test.go` -- covers WHK-03, claim 409

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection of all bug locations
- `platform/internal/models/models.go:195-208` -- Signal struct JSON tags
- `platform/internal/api/webhook.go:179-202` -- pollRetries implementation
- `platform/internal/api/human.go:55-73` -- claim redemption handler
- `platform/internal/api/api.go:28-78` -- store interface definitions
- `mobile/lib/core/models/signal.dart:22-35` -- Signal.fromJson
- `mobile/lib/providers/inbox_provider.dart:95-124` -- loadMore pagination
- `platform/internal/store/store.go:98-99` -- ErrClaimNotAvailable sentinel

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new libraries, all existing code
- Architecture: HIGH -- all bug locations verified in source, fixes are mechanical
- Pitfalls: HIGH -- bugs are simple enough that pitfalls are limited to overlooking secondary changes

**Research date:** 2026-03-12
**Valid until:** indefinite (bug fix research tied to current code state)
