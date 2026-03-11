# Phase 2: Signal Pipeline - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Agents can send signals to any entity's inbox and receive them in real-time via SSE, webhook push, or polling — with durable persistence and no signal loss. Includes: signal sending/receiving, inbox with cursor-based pagination, SSE streaming with reconnection replay, webhook push delivery with retry, inbound channels (trusted + open), and integration tests with testcontainers-go. Does NOT include: delegations, trust chain enforcement, human entities, attestations, or mobile app.

</domain>

<decisions>
## Implementation Decisions

### Signal access model
- Trust-zone based access: entities within a trust relationship can send directly to each other's inbox
- Phase 2 stubs trust — all authenticated entities can send to any entity (full trust zone enforcement deferred until delegation chains exist)
- Direct sending uses `POST /v1/inbox/{entity-id}` with entity ID only (not URI)
- Every signal is Ed25519 signed by the sender: canonical(route) + "." + canonical(signal) — cryptographic proof of origin
- Signals include thread_id and ref fields (stored as metadata, not enforced)
- Priority field stored on signals (normal/high/urgent) but delivery order is insertion-order only — no queue jumping in Phase 2

### Channel types — trusted and open
- **Trusted channels**: bound to a named trustee (ATAP entity ID). Platform verifies entity exists at creation. Only that entity can POST. Full Ed25519 signature verification on every request (auth middleware + trustee ID match)
- **Open channels**: secured by 128-bit entropy URL + platform-generated Basic Auth credentials. Credentials returned once at creation. For external/untrusted services (Stripe, GitHub, etc.)
- One trustee per channel — want to authorize 3 agents? Create 3 channels. Simple, per-relationship revocation
- No scope constraints in Phase 2 (no signal type filters, rate limits, or auto-expiration on channels)
- Signals arriving through open channels are flagged as untrusted (source: 'external', trust_level: 0). Receiving entity can filter/handle differently

### Webhook delivery
- Entity registers a webhook URL via separate endpoint: `POST /v1/entities/{id}/webhook` (decoupled from registration)
- One webhook URL per entity — entity handles its own routing
- No URL verification (challenge-response) — accept any URL, retry logic handles failures
- SSE and webhook fire independently — both deliver if both are configured. Belt and suspenders
- Webhook payload signed with Ed25519 in `X-ATAP-Signature` header
- Failed webhooks retry with exponential backoff: 1s, 5s, 30s, 5m, 30m — max 5 attempts

### Signal lifecycle
- Expired signals (past TTL) excluded from inbox queries and SSE replay but remain in PostgreSQL for audit
- Redis TTL handles pub/sub side cleanup automatically
- Default TTL: 7 days if sender doesn't specify. Sender can override with shorter or longer
- After max webhook retries: signal marked delivery_status='failed', still in inbox for SSE/polling pickup
- Webhook delivery attempt records cleaned up after 24 hours to manage data growth
- Max signal payload size: 64 KB — reject larger with HTTP 413
- Background cleanup job for expired signals (implementation details at Claude's discretion)

### Claude's Discretion
- Signal database schema details (JSONB fields, indexes, partitioning)
- SSE implementation specifics (Fiber streaming, goroutine management)
- Redis pub/sub channel naming and message format
- Webhook delivery worker architecture (goroutine pool, queue, etc.)
- Exponential backoff implementation
- Integration test structure and testcontainers-go setup
- Migration numbering and schema evolution from Phase 1
- Idempotency key implementation (unique index, 24h dedup window)

</decisions>

<specifics>
## Specific Ideas

- "Entities can send messages within their trust zone — agent to its human, agent to related agents, humans to their agents — but external callers only POST to endpoints the agent/human created for a specific reason"
- Channels as consent tokens: anti-spam by design — no anonymous channel usage
- Open channels with Basic Auth fill the gap for "dumb" webhook sources (Stripe, GitHub, Zapier) that can't do Ed25519
- Both SSE and webhook fire for the same signal — SSE for real-time, webhook for guaranteed delivery
- Data growth awareness: delivery attempts clean up after 24h, signals after 7d default TTL

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/crypto/crypto.go`: Ed25519 key generation, canonical JSON (JCS), signature verification, key encoding — all needed for signal signing and webhook signatures
- `internal/crypto/crypto.go` `ParseSignatureHeader` / `VerifyRequest`: reuse for channel trustee verification
- `internal/models/models.go`: Entity model, ProblemDetail — extend with Signal, Channel, WebhookConfig models
- `internal/store/store.go`: pgx/v5 pool pattern, entity CRUD — extend with signal/channel/webhook stores
- `internal/api/api.go`: Handler struct with EntityStore interface, auth middleware — extend with SignalStore, ChannelStore interfaces

### Established Patterns
- EntityStore interface pattern enables fake-store testing without PostgreSQL — replicate for SignalStore, ChannelStore
- Auth middleware extracts entity from signed request and sets `c.Locals("entity")` — all new endpoints use this
- RFC 7807 `problem()` helper for all error responses
- Fiber v2 route groups with middleware chaining
- zerolog structured JSON logging with entity context

### Integration Points
- `api.SetupRoutes()` — add signal, channel, webhook, SSE routes under authenticated group
- `cmd/server/main.go` — Redis client already initialized (currently unused), needed for pub/sub
- `migrations/` — add 002_signals.up.sql, 003_channels.up.sql, etc.
- `store.Store` — add signal and channel methods alongside entity methods

</code_context>

<deferred>
## Deferred Ideas

- Trust zone enforcement with delegation chains — requires delegation infrastructure (Phase 2 of BUILD-GUIDE / v2 requirements)
- Channel scope constraints (signal type filters, rate limits, auto-expiration) — future phase
- Multiple webhook URLs per entity (primary + fallback) — future phase if needed
- Webhook URL challenge-response verification — future phase
- Priority-based delivery ordering — future phase
- Human entity registration and key recovery — Phase 2 of BUILD-GUIDE

</deferred>

---

*Phase: 02-signal-pipeline*
*Context gathered: 2026-03-11*
