# Phase 4: Fix Signal Pipeline Bugs - Context

**Gathered:** 2026-03-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix 4 cross-language integration bugs that break signal display, inbox pagination, webhook retry delivery, and concurrent claim redemption. All bugs have known root causes and specific file locations. No new features — pure bug fixes with targeted tests.

</domain>

<decisions>
## Implementation Decisions

### Signal JSON contract
- Flutter `Signal.fromJson` uses the existing `ts` field (already in JSON) instead of `created_at`
- `created_at` stays as `json:"-"` in Go — no API change needed
- `ts` and `created_at` are effectively duplicate data (both set server-side at creation time) — one authoritative timestamp is cleaner
- Update roadmap success criterion #1 to reference `ts` instead of `created_at`

### Webhook retry payload
- `pollRetries()` fetches the signal from the signal store using `SignalID` before enqueueing the retry job
- One DB read per retry — simple, always fresh, small memory footprint
- If signal is expired or deleted by retry time: skip silently, log a warning, mark attempt as failed
- No schema changes to `webhook_delivery_attempts` table

### Claim concurrency
- Handler checks for `ErrClaimNotAvailable` sentinel error and returns 409 Conflict (not 500)
- No structural changes — just error type checking in the existing handler

### Pagination param
- Flutter `loadMore()` changes `?cursor=` to `?after=` to match Go backend's `c.Query("after", "")`
- One-line fix in `inbox_provider.dart`

### Testing approach
- One targeted unit test per bug fix (4 Go tests)
- Plus one Dart unit test for the Flutter pagination fix (mock API client, assert `?after=` param)
- No integration tests — focused unit tests are sufficient for a bug-fix phase

### Claude's Discretion
- Exact test structure and assertions
- Whether to refactor webhook worker to accept a signal store interface or pass it through existing wiring
- Go test helper patterns for claim concurrency test (goroutines vs sequential with pre-redeemed claim)

</decisions>

<specifics>
## Specific Ideas

- `ts` vs `created_at` discussion revealed they're duplicate data — keep API surface minimal with just `ts`
- Webhook retry should gracefully handle expired signals rather than failing loudly

</specifics>

<code_context>
## Existing Code Insights

### Bug Locations
- `platform/internal/models/models.go:207` — `CreatedAt` tagged `json:"-"` (no change needed now)
- `mobile/lib/providers/inbox_provider.dart:107` — sends `?cursor=` instead of `?after=`
- `platform/internal/api/webhook.go:190` — `pollRetries()` enqueues without payload
- `platform/internal/api/human.go:70` — doesn't check `ErrClaimNotAvailable`

### Reusable Assets
- `store.ErrClaimNotAvailable` sentinel error already exists (Phase 3)
- `store.GetSignal()` or equivalent needed for webhook retry payload fetch
- Existing `WebhookJob` struct has `Payload` field — just needs to be populated
- Flutter `Signal.fromJson` already parses timestamps — just wrong field name

### Established Patterns
- `errors.Is(err, store.ErrClaimNotAvailable)` for sentinel error checking (Go convention)
- `EntityStore` interface pattern for test mocking — webhook worker may need `SignalStore` interface access
- Phase 2 testcontainers-go for integration tests (not needed here — unit tests sufficient)

### Integration Points
- Webhook worker needs signal store access to fetch payloads for retry
- Flutter inbox provider connects to Go `GET /v1/inbox/{id}?after=&limit=50`

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-fix-signal-pipeline-bugs*
*Context gathered: 2026-03-12*
