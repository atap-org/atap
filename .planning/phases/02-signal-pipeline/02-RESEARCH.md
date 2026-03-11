# Phase 2: Signal Pipeline - Research

**Researched:** 2026-03-11
**Domain:** Signal delivery (send/receive/stream), webhook push, inbound channels, Go/Fiber/PostgreSQL/Redis
**Confidence:** HIGH

## Summary

Phase 2 builds the core signal pipeline on top of Phase 1's entity registration and Ed25519 auth infrastructure. The work spans four interconnected subsystems: (1) signal persistence and inbox polling, (2) real-time SSE streaming via Redis pub/sub, (3) outbound webhook delivery with retry, and (4) inbound channels (trusted + open). All subsystems are well-supported by the existing stack -- Fiber v2 has proven SSE streaming via `fasthttp.StreamWriter`, go-redis v9 has stable pub/sub, and pgx/v5 handles the persistence layer.

The existing codebase provides strong foundations: the `EntityStore` interface pattern, auth middleware, RFC 7807 error helpers, and crypto utilities (canonical JSON, Ed25519 signing) are all directly reusable. The Handler struct needs to grow to accept Redis client and new store interfaces. The main server already initializes Redis but does not pass it to handlers yet.

**Primary recommendation:** Extend the existing single-file patterns (models.go, store.go, api.go) with signal/channel/webhook types and handlers. Use a background goroutine pool for webhook delivery. Use testcontainers-go modules for PostgreSQL and Redis in integration tests.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Trust-zone based access stubbed: all authenticated entities can send to any entity (full trust zone enforcement deferred)
- Direct sending uses `POST /v1/inbox/{entity-id}` with entity ID only (not URI)
- Every signal is Ed25519 signed by the sender: canonical(route) + "." + canonical(signal)
- Signals include thread_id and ref fields (stored as metadata, not enforced)
- Priority field stored on signals (normal/high/urgent) but delivery order is insertion-order only
- Trusted channels: bound to a named trustee (ATAP entity ID), full Ed25519 signature verification
- Open channels: secured by 128-bit entropy URL + platform-generated Basic Auth credentials
- One trustee per channel
- No scope constraints in Phase 2
- Signals arriving through open channels flagged as untrusted (source: 'external', trust_level: 0)
- Entity registers webhook URL via `POST /v1/entities/{id}/webhook` (decoupled from registration)
- One webhook URL per entity
- No URL verification (challenge-response)
- SSE and webhook fire independently -- both deliver if both configured
- Webhook payload signed with Ed25519 in `X-ATAP-Signature` header
- Failed webhooks retry: 1s, 5s, 30s, 5m, 30m -- max 5 attempts
- Expired signals excluded from inbox queries and SSE replay but remain in PostgreSQL
- Redis TTL handles pub/sub cleanup
- Default TTL: 7 days if sender doesn't specify
- After max webhook retries: signal marked delivery_status='failed', still in inbox
- Webhook delivery attempt records cleaned up after 24 hours
- Max signal payload size: 64 KB -- reject with HTTP 413
- Idempotency key deduplication within 24-hour window via unique index

### Claude's Discretion
- Signal database schema details (JSONB fields, indexes, partitioning)
- SSE implementation specifics (Fiber streaming, goroutine management)
- Redis pub/sub channel naming and message format
- Webhook delivery worker architecture (goroutine pool, queue, etc.)
- Exponential backoff implementation
- Integration test structure and testcontainers-go setup
- Migration numbering and schema evolution from Phase 1
- Idempotency key implementation (unique index, 24h dedup window)

### Deferred Ideas (OUT OF SCOPE)
- Trust zone enforcement with delegation chains
- Channel scope constraints (signal type filters, rate limits, auto-expiration)
- Multiple webhook URLs per entity
- Webhook URL challenge-response verification
- Priority-based delivery ordering
- Human entity registration and key recovery
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SIG-01 | Authenticated entity can send a signal to any entity's inbox via `POST /v1/inbox/{target-id}` | Extends existing auth middleware + store pattern; new Signal model + SaveSignal store method |
| SIG-02 | Signals follow ATAP format: route/signal/context blocks | Signal model struct maps to BUILD-GUIDE Section 10 format; JSONB for data field |
| SIG-03 | Signals persist in PostgreSQL and survive platform restarts | New signals table migration; write-then-notify pattern ensures persistence before pub/sub |
| SIG-04 | Inbox supports cursor-based pagination via `GET /v1/inbox/{entity-id}?after={cursor}&limit=50` | ULID-based signal IDs are lexicographically sortable, enabling efficient cursor pagination |
| SIG-05 | Expired signals (past TTL) excluded from inbox queries | WHERE clause filters on expires_at; computed from created_at + ttl at insert time |
| SIG-06 | Idempotency key deduplication within 24-hour window | Unique partial index on idempotency_key WHERE idempotency_key IS NOT NULL |
| SSE-01 | Entity can open SSE stream via `GET /v1/inbox/{entity-id}/stream` | Fiber v2 SetBodyStreamWriter + fasthttp.StreamWriter pattern; Redis pub/sub subscription |
| SSE-02 | SSE reconnection replays missed signals using Last-Event-ID | Query signals WHERE id > last_event_id before subscribing to Redis |
| SSE-03 | 30-second heartbeat comments keep connections alive | time.Ticker in select loop writes `: heartbeat\n\n` |
| SSE-04 | PostgreSQL write completes before Redis publish (write-then-notify) | Sequential: store.SaveSignal() then redis.Publish() in send handler |
| WHK-01 | Platform pushes signals to entity's registered webhook URL via HTTP POST | Background goroutine pool with channel-based queue; http.Client with timeout |
| WHK-02 | Webhook payload signed with Ed25519 in X-ATAP-Signature header | Reuse crypto.Sign(); platform needs its own signing key pair |
| WHK-03 | Failed webhooks retry with exponential backoff (1s, 5s, 30s, 5m, 30m) | In-memory retry queue with time.AfterFunc scheduling; delivery_attempts table for tracking |
| WHK-04 | Undeliverable signals marked after max retries | Update signal delivery_status='failed' after 5th attempt fails |
| CHN-01 | Entity can create inbound channels via `POST /v1/entities/{id}/channels` | New channels table; crypto.NewChannelID() already exists for 128-bit entropy IDs |
| CHN-02 | Each channel has unique webhook URL | Constructed from channel ID: `{platform_url}/v1/channels/{channel-id}/signals` |
| CHN-03 | Inbound webhook payloads wrapped into ATAP signals and delivered to inbox | Channel handler wraps raw JSON in Signal struct with source='external' |
| CHN-04 | Entity can list own channels and revoke individual channels | List/delete store methods; revoke sets active=false, revoked_at=now |
| CHN-05 | Channel webhook URL uses 128-bit entropy | crypto.NewChannelID() already generates `chn_` + 32 hex chars (128 bits) |
| TST-01 | Integration tests covering full agent lifecycle: register, send, receive via SSE | testcontainers-go with real PostgreSQL + Redis; test the complete flow end-to-end |
| TST-02 | Integration tests use testcontainers-go for real PostgreSQL and Redis | testcontainers-go/modules/postgres and testcontainers-go/modules/redis |
</phase_requirements>

## Standard Stack

### Core (already in go.mod)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gofiber/fiber/v2 | v2.52.12 | HTTP framework + SSE streaming | Already in use; SetBodyStreamWriter for SSE |
| jackc/pgx/v5 | v5.7.5 | PostgreSQL driver + connection pool | Already in use; signals/channels persistence |
| redis/go-redis/v9 | v9.18.0 | Redis pub/sub for real-time delivery | Already in go.mod, client initialized in main.go |
| oklog/ulid/v2 | v2.1.1 | Signal ID generation (sig_ + ULID) | Already in use for entity IDs |
| gowebpki/jcs | v1.0.1 | Canonical JSON (RFC 8785) for signatures | Already in use in crypto package |
| rs/zerolog | v1.34.0 | Structured logging | Already in use |
| golang-migrate/migrate/v4 | v4.19.1 | SQL migrations | Already in use |

### New Dependencies
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| testcontainers/testcontainers-go | latest | Container-based integration tests | TST-01, TST-02 |
| testcontainers/testcontainers-go/modules/postgres | latest | PostgreSQL test container | Integration tests |
| testcontainers/testcontainers-go/modules/redis | latest | Redis test container | Integration tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory webhook queue | Redis Streams for retry queue | Overkill for Phase 2; CONTEXT.md defers Redis Streams. In-memory + delivery_attempts table is simpler |
| goroutine-per-webhook | Worker pool pattern | Worker pool prevents goroutine explosion under load |

**Installation:**
```bash
cd platform && go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres github.com/testcontainers/testcontainers-go/modules/redis
```

## Architecture Patterns

### Recommended Project Structure
```
platform/
  internal/
    models/models.go       # Add Signal, Channel, WebhookConfig, delivery types
    store/store.go         # Add signal, channel, webhook store methods
    api/api.go             # Add signal, channel, webhook, SSE handlers + routes
    api/api_test.go        # Extend fakeStore with new interfaces
    crypto/crypto.go       # Add NewSignalID(), platform key management
  migrations/
    002_signals.up.sql     # signals table
    002_signals.down.sql
    003_channels.up.sql    # channels + webhook_configs tables
    003_channels.down.sql
    004_webhook_delivery.up.sql   # delivery_attempts table
    004_webhook_delivery.down.sql
  test/
    integration_test.go    # testcontainers-go integration tests
```

### Pattern 1: Write-Then-Notify (SIG-03, SSE-04)
**What:** Always persist to PostgreSQL before publishing to Redis pub/sub.
**When to use:** Every signal send operation.
**Why:** Redis pub/sub is fire-and-forget. If the service crashes between publish and write, signals are lost. Write first guarantees durability.
**Example:**
```go
// In the send handler:
// 1. Validate and build signal
// 2. Store in PostgreSQL (durable)
if err := h.store.SaveSignal(ctx, signal); err != nil {
    return problem(c, 500, "save_failed", "Failed to save signal", "")
}
// 3. Publish to Redis (best-effort real-time notification)
signalJSON, _ := json.Marshal(signal)
h.redis.Publish(ctx, "inbox:"+targetID, signalJSON)
// 4. Queue webhook delivery if entity has webhook configured
```

### Pattern 2: SSE with Redis Pub/Sub (SSE-01, SSE-02, SSE-03)
**What:** Fiber v2 SSE using fasthttp.StreamWriter with Redis subscription.
**When to use:** The `/v1/inbox/{entity-id}/stream` endpoint.
**Example:**
```go
func (h *Handler) InboxStream(c *fiber.Ctx) error {
    entityID := c.Params("entityId")
    lastEventID := c.Get("Last-Event-ID")

    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")

    c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
        // 1. Replay missed signals from PostgreSQL
        if lastEventID != "" {
            missed, _ := h.store.GetSignalsAfter(ctx, entityID, lastEventID)
            for _, sig := range missed {
                fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", sig.ID, sig.JSON())
            }
            w.Flush()
        }

        // 2. Subscribe to Redis
        sub := h.redis.Subscribe(context.Background(), "inbox:"+entityID)
        defer sub.Close()
        ch := sub.Channel()

        // 3. Heartbeat + message loop
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case msg := <-ch:
                fmt.Fprintf(w, "event: signal\nid: %s\ndata: %s\n\n", extractID(msg.Payload), msg.Payload)
                if err := w.Flush(); err != nil {
                    return // client disconnected
                }
            case <-ticker.C:
                fmt.Fprintf(w, ": heartbeat\n\n")
                if err := w.Flush(); err != nil {
                    return // client disconnected
                }
            }
        }
    }))
    return nil
}
```

### Pattern 3: Webhook Delivery Worker (WHK-01 through WHK-04)
**What:** Background goroutine pool that processes webhook deliveries with exponential backoff retry.
**When to use:** Any signal sent to an entity that has a webhook URL configured.
**Example:**
```go
type WebhookWorker struct {
    queue       chan WebhookJob
    httpClient  *http.Client
    store       WebhookStore
    platformKey ed25519.PrivateKey
    log         zerolog.Logger
}

type WebhookJob struct {
    SignalID   string
    WebhookURL string
    Payload    []byte
    Attempt    int
}

var retryDelays = []time.Duration{
    1 * time.Second, 5 * time.Second, 30 * time.Second,
    5 * time.Minute, 30 * time.Minute,
}

func (w *WebhookWorker) Start(ctx context.Context, workers int) {
    for i := 0; i < workers; i++ {
        go w.worker(ctx)
    }
}

func (w *WebhookWorker) worker(ctx context.Context) {
    for {
        select {
        case job := <-w.queue:
            w.deliver(ctx, job)
        case <-ctx.Done():
            return
        }
    }
}
```

### Pattern 4: Store Interface Extension
**What:** Extend the existing EntityStore pattern with SignalStore and ChannelStore interfaces.
**When to use:** All new data access.
**Example:**
```go
type SignalStore interface {
    SaveSignal(ctx context.Context, s *Signal) error
    GetSignal(ctx context.Context, id string) (*Signal, error)
    GetInbox(ctx context.Context, entityID string, after string, limit int) ([]*Signal, error)
    GetSignalsAfter(ctx context.Context, entityID string, afterID string) ([]*Signal, error)
}

type ChannelStore interface {
    CreateChannel(ctx context.Context, ch *Channel) error
    GetChannel(ctx context.Context, id string) (*Channel, error)
    ListChannels(ctx context.Context, entityID string) ([]*Channel, error)
    RevokeChannel(ctx context.Context, id string) error
}

type WebhookStore interface {
    GetWebhookConfig(ctx context.Context, entityID string) (*WebhookConfig, error)
    SetWebhookConfig(ctx context.Context, entityID string, url string) error
    SaveDeliveryAttempt(ctx context.Context, attempt *DeliveryAttempt) error
    UpdateSignalDeliveryStatus(ctx context.Context, signalID string, status string) error
}
```

### Anti-Patterns to Avoid
- **Publishing to Redis before PostgreSQL write:** Violates write-then-notify. A crash between publish and write loses the signal permanently.
- **Blocking the send handler on webhook delivery:** Webhook delivery must be async. The send handler returns 202 immediately after persist + pub/sub.
- **Using `c.Context().Done()` alone for SSE disconnect detection:** In Fiber v2/fasthttp, check `w.Flush()` error as the primary disconnect signal.
- **Creating a goroutine per webhook delivery:** Use a bounded worker pool to prevent goroutine explosion under load.
- **Storing signal payload as TEXT:** Use JSONB for the data field to enable future query capabilities.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Signal IDs | Custom UUID scheme | `sig_` + ULID (oklog/ulid/v2) | ULID provides time-ordering + uniqueness; already in use for entities |
| Channel IDs | Custom random | `crypto.NewChannelID()` | Already implemented with 128-bit entropy |
| Canonical JSON | Custom JSON sorting | `gowebpki/jcs` via existing `crypto.CanonicalJSON()` | RFC 8785 compliant; already in crypto package |
| Integration test containers | Docker scripts / docker-compose in tests | testcontainers-go modules | Automatic lifecycle, port mapping, cleanup |
| SSE event formatting | Custom SSE writer | Standard `fmt.Fprintf(w, "event: ...\nid: ...\ndata: ...\n\n")` | SSE spec is simple enough, but format must be exact |
| Ed25519 webhook signatures | HMAC or custom signing | `crypto.Sign()` with platform key | Reuse existing Ed25519 infrastructure |

**Key insight:** The codebase already has most of the building blocks (Ed25519, canonical JSON, ULID, channel IDs). Phase 2 is primarily about wiring them together with new database tables and API handlers.

## Common Pitfalls

### Pitfall 1: SSE Replay Gap
**What goes wrong:** Signals sent between PostgreSQL query (replay) and Redis subscription start are missed.
**Why it happens:** There's a window between "query all signals after X" and "subscribe to Redis channel."
**How to avoid:** Subscribe to Redis FIRST, buffer incoming messages, THEN replay from PostgreSQL, THEN start delivering buffered + live messages. Or: accept minor duplication by replaying slightly overlapping signals (clients should handle duplicate IDs).
**Warning signs:** Missed signals during reconnection under high throughput.

### Pitfall 2: Fiber v2 StreamWriter Disconnect Detection
**What goes wrong:** SSE goroutine keeps running after client disconnects, leaking goroutines.
**Why it happens:** `c.Context().Done()` may not reliably fire in Fiber v2/fasthttp for streaming connections.
**How to avoid:** Check `w.Flush()` error return as the primary disconnect signal. When Flush returns error, the client is gone.
**Warning signs:** Growing goroutine count over time; Redis subscriptions accumulate.

### Pitfall 3: Webhook Retry Goroutine Leak
**What goes wrong:** Each retry spawns a new goroutine with `time.AfterFunc`, leading to unbounded goroutine growth.
**Why it happens:** High failure rate + long retry delays (up to 30 minutes) means many goroutines waiting.
**How to avoid:** Use a delivery_attempts table in PostgreSQL with `next_retry_at`. A single background ticker goroutine polls for due retries and feeds them into the worker pool.
**Warning signs:** Memory growth proportional to webhook failures.

### Pitfall 4: Signal Payload Size DoS
**What goes wrong:** Large payloads consume memory and database space.
**Why it happens:** No size limit enforced.
**How to avoid:** Check `Content-Length` header and `len(body)` before parsing. Reject with HTTP 413 if > 64 KB. Also set Fiber's `BodyLimit` config.
**Warning signs:** Slow queries, high memory usage.

### Pitfall 5: Idempotency Window Cleanup
**What goes wrong:** The idempotency unique index prevents duplicates but old entries never cleaned up.
**Why it happens:** No TTL mechanism on the index itself.
**How to avoid:** The unique index on `idempotency_key` already handles dedup. Expired signals (past TTL or 7-day default) remain for audit but the unique index prevents reinsertion of same key. For the 24-hour dedup window, add a partial unique index: `WHERE created_at > NOW() - INTERVAL '24 hours'` won't work (not immutable). Instead, use ON CONFLICT with a check on created_at in the insert query.
**Warning signs:** Duplicate signals appearing after 24 hours with same idempotency key.

### Pitfall 6: Open Channel Basic Auth Credential Storage
**What goes wrong:** Storing Basic Auth credentials in plaintext allows credential theft from database breach.
**Why it happens:** Treating credentials as non-sensitive because they're for inbound webhooks.
**How to avoid:** Store Basic Auth password as bcrypt hash (or SHA-256 hash). Return credentials once at channel creation, never again (same pattern as entity registration returning private key once).
**Warning signs:** Credentials visible in database dumps.

## Code Examples

### Signal Model
```go
// Source: BUILD-GUIDE Section 10 + CONTEXT.md decisions
type Signal struct {
    ID      string    `json:"id"`       // sig_ + ULID
    Version string    `json:"v"`        // "1"
    TS      time.Time `json:"ts"`

    Route SignalRoute   `json:"route"`
    Trust SignalTrust   `json:"trust"`
    Signal SignalBody   `json:"signal"`
    Context SignalContext `json:"context"`

    // Server-side fields (not in wire format)
    TargetEntityID  string     `json:"-"`
    DeliveryStatus  string     `json:"-"` // pending, delivered, failed
    ExpiresAt       *time.Time `json:"-"`
    CreatedAt       time.Time  `json:"-"`
}

type SignalRoute struct {
    Origin   string `json:"origin"`
    Target   string `json:"target"`
    ReplyTo  string `json:"reply_to,omitempty"`
    Channel  string `json:"channel,omitempty"`
    Thread   string `json:"thread,omitempty"`
    Ref      string `json:"ref,omitempty"`
}

type SignalBody struct {
    Type      string          `json:"type"`
    Encrypted bool            `json:"encrypted"`
    Data      json.RawMessage `json:"data"`
}

type SignalContext struct {
    Source      string   `json:"source"`            // agent, external, system
    Idempotency string  `json:"idempotency,omitempty"`
    Tags        []string `json:"tags,omitempty"`
    TTL         int      `json:"ttl,omitempty"`     // seconds
    Priority    string   `json:"priority,omitempty"` // normal, high, urgent
}
```

### Channel Model
```go
type Channel struct {
    ID          string     `json:"id"`         // chn_ + 32 hex
    EntityID    string     `json:"entity_id"`
    WebhookURL  string     `json:"webhook_url"`
    Label       string     `json:"label,omitempty"`
    Tags        []string   `json:"tags,omitempty"`
    Type        string     `json:"type"`       // trusted, open
    TrusteeID   string     `json:"trustee_id,omitempty"` // for trusted channels
    Active      bool       `json:"active"`
    SignalCount int64      `json:"signal_count"`
    CreatedAt   time.Time  `json:"created_at"`
    RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}
```

### Cursor-Based Pagination Query
```go
// Source: BUILD-GUIDE Section 9 pagination pattern
func (s *Store) GetInbox(ctx context.Context, entityID, after string, limit int) ([]*Signal, bool, error) {
    query := `
        SELECT id, version, origin, target, reply_to, channel_id, thread_id, ref_id,
               signature, signer_key_id, content_type, data, source_type,
               idempotency_key, tags, ttl, priority, delivery_status, expires_at, created_at
        FROM signals
        WHERE target_entity_id = $1
          AND (expires_at IS NULL OR expires_at > NOW())
    `
    args := []interface{}{entityID}
    if after != "" {
        query += " AND id > $2"
        args = append(args, after)
    }
    query += " ORDER BY id ASC LIMIT $" + strconv.Itoa(len(args)+1)
    args = append(args, limit+1) // fetch one extra to determine has_more

    rows, err := s.pool.Query(ctx, query, args...)
    // ... scan rows, return signals[:limit], len(signals) > limit, nil
}
```

### Platform Signing Key for Webhooks
```go
// The platform needs its own Ed25519 key pair for signing webhook payloads.
// Generated once at startup, persisted in config or environment.
type WebhookSigner struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
    keyID      string
}

func (ws *WebhookSigner) SignPayload(payload []byte) string {
    sig := ed25519.Sign(ws.privateKey, payload)
    return base64.StdEncoding.EncodeToString(sig)
}
```

### Integration Test Setup with testcontainers-go
```go
// Source: testcontainers-go official docs
import (
    "github.com/testcontainers/testcontainers-go"
    tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupTestInfra(t *testing.T) (dbURL, redisURL string) {
    ctx := context.Background()

    // PostgreSQL
    pgContainer, err := tcpostgres.Run(ctx,
        "postgres:16-alpine",
        tcpostgres.WithDatabase("atap_test"),
        tcpostgres.WithUsername("test"),
        tcpostgres.WithPassword("test"),
        tcpostgres.BasicWaitStrategies(),
    )
    require.NoError(t, err)
    t.Cleanup(func() { testcontainers.TerminateContainer(pgContainer) })

    dbURL, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    // Redis
    redisContainer, err := tcredis.Run(ctx, "redis:7")
    require.NoError(t, err)
    t.Cleanup(func() { testcontainers.TerminateContainer(redisContainer) })

    redisURL, err = redisContainer.ConnectionString(ctx)
    require.NoError(t, err)

    return dbURL, redisURL
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bearer tokens | Ed25519 signed requests | Phase 1 (01-02) | Auth middleware already uses signed requests; new endpoints inherit this |
| Token hash storage | Public key lookup by key_id | Phase 1 (01-02) | GetEntityByKeyID already in store |
| go-redis v8 | go-redis v9 | 2023 | Import path is `github.com/redis/go-redis/v9` (already in go.mod) |
| Manual Docker in tests | testcontainers-go modules | 2024-2025 | Postgres and Redis modules provide typed container helpers |

**Deprecated/outdated:**
- Bearer token auth: Removed in Phase 1. All new endpoints use Ed25519 signed request auth.
- `token_hash` column: Removed from entities table.

## Open Questions

1. **Platform signing key management for webhooks (WHK-02)**
   - What we know: Platform needs an Ed25519 key to sign webhook payloads. The crypto package has all primitives.
   - What's unclear: Where to persist the platform key. Options: environment variable, generated at startup and stored in DB, or config file.
   - Recommendation: Generate at startup, store in a `platform_keys` table or environment variable. For Phase 2, environment variable is simplest.

2. **SSE replay gap mitigation (SSE-02)**
   - What we know: There's an inherent race between PostgreSQL replay and Redis subscription.
   - What's unclear: Whether to subscribe-first-then-replay (correct but complex) or accept rare duplicates.
   - Recommendation: Subscribe first, buffer messages, then replay, then drain buffer. Signal IDs are unique so clients can deduplicate.

3. **Background cleanup job for expired signals**
   - What we know: CONTEXT.md says expired signals remain for audit but are excluded from queries. Delivery attempts clean up after 24h.
   - What's unclear: Whether to use a Go background goroutine with ticker or external pg_cron.
   - Recommendation: Go background goroutine with 1-hour ticker. Simpler deployment, no pg_cron dependency.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testcontainers-go v0.35+ |
| Config file | None needed -- Go's built-in test runner |
| Quick run command | `cd platform && go test ./internal/...` |
| Full suite command | `cd platform && go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SIG-01 | Send signal to inbox | unit + integration | `cd platform && go test ./internal/api -run TestSendSignal -count=1` | No -- Wave 0 |
| SIG-02 | Signal format validation | unit | `cd platform && go test ./internal/api -run TestSignalFormat -count=1` | No -- Wave 0 |
| SIG-03 | Signal persistence | integration | `cd platform && go test ./test -run TestSignalPersistence -count=1` | No -- Wave 0 |
| SIG-04 | Cursor-based pagination | unit + integration | `cd platform && go test ./internal/api -run TestInboxPagination -count=1` | No -- Wave 0 |
| SIG-05 | Expired signal exclusion | unit | `cd platform && go test ./internal/api -run TestExpiredExcluded -count=1` | No -- Wave 0 |
| SIG-06 | Idempotency dedup | integration | `cd platform && go test ./test -run TestIdempotency -count=1` | No -- Wave 0 |
| SSE-01 | SSE stream receives signals | integration | `cd platform && go test ./test -run TestSSEStream -count=1` | No -- Wave 0 |
| SSE-02 | SSE reconnection replay | integration | `cd platform && go test ./test -run TestSSEReplay -count=1` | No -- Wave 0 |
| SSE-03 | 30s heartbeat | unit | `cd platform && go test ./internal/api -run TestSSEHeartbeat -count=1` | No -- Wave 0 |
| SSE-04 | Write-then-notify | integration | `cd platform && go test ./test -run TestWriteThenNotify -count=1` | No -- Wave 0 |
| WHK-01 | Webhook push delivery | integration | `cd platform && go test ./test -run TestWebhookDelivery -count=1` | No -- Wave 0 |
| WHK-02 | Webhook Ed25519 signature | unit | `cd platform && go test ./internal/api -run TestWebhookSignature -count=1` | No -- Wave 0 |
| WHK-03 | Webhook retry backoff | unit | `cd platform && go test ./internal/api -run TestWebhookRetry -count=1` | No -- Wave 0 |
| WHK-04 | Undeliverable marking | integration | `cd platform && go test ./test -run TestWebhookMaxRetries -count=1` | No -- Wave 0 |
| CHN-01 | Create channel | unit + integration | `cd platform && go test ./internal/api -run TestCreateChannel -count=1` | No -- Wave 0 |
| CHN-02 | Channel webhook URL | unit | `cd platform && go test ./internal/api -run TestChannelWebhookURL -count=1` | No -- Wave 0 |
| CHN-03 | Inbound webhook wrapping | unit + integration | `cd platform && go test ./internal/api -run TestInboundWebhook -count=1` | No -- Wave 0 |
| CHN-04 | List and revoke channels | unit | `cd platform && go test ./internal/api -run TestChannelListRevoke -count=1` | No -- Wave 0 |
| CHN-05 | 128-bit entropy | unit | `cd platform && go test ./internal/crypto -run TestChannelIDEntropy -count=1` | No -- Wave 0 (crypto test exists but not this specific one) |
| TST-01 | Full lifecycle integration | integration | `cd platform && go test ./test -run TestFullLifecycle -count=1` | No -- Wave 0 |
| TST-02 | testcontainers-go setup | integration | `cd platform && go test ./test -run Test -count=1` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./internal/... -count=1`
- **Per wave merge:** `cd platform && go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `platform/test/integration_test.go` -- testcontainers-go setup + full lifecycle tests (TST-01, TST-02)
- [ ] testcontainers-go dependency: `go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres github.com/testcontainers/testcontainers-go/modules/redis`
- [ ] Extend `fakeStore` in `api_test.go` with SignalStore + ChannelStore methods for unit tests
- [ ] Migrations `002_signals.up.sql`, `003_channels.up.sql`, `004_webhook_delivery.up.sql`

## Sources

### Primary (HIGH confidence)
- Existing codebase: `platform/internal/api/api.go`, `store/store.go`, `models/models.go`, `crypto/crypto.go` -- direct code review
- Existing codebase: `platform/cmd/server/main.go` -- Redis already initialized
- Existing codebase: `platform/migrations/001_entities.up.sql` -- schema pattern
- BUILD-GUIDE Sections 8, 9, 10, 12, 13 -- signal format, API spec, SSE, webhooks

### Secondary (MEDIUM confidence)
- [Fiber SSE Issue #429](https://github.com/gofiber/fiber/issues/429) -- SetBodyStreamWriter pattern verified
- [Fiber SSE Issue #2837](https://github.com/gofiber/fiber/issues/2837) -- Flush error for disconnect detection
- [testcontainers-go Postgres module](https://golang.testcontainers.org/modules/postgres/) -- container setup API
- [testcontainers-go Redis module](https://golang.testcontainers.org/modules/redis/) -- container setup API
- [go-redis pub/sub](https://redis.uptrace.dev/guide/go-redis-pubsub.html) -- Subscribe/Channel pattern

### Tertiary (LOW confidence)
- None -- all findings verified against official sources or codebase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in go.mod or have official module docs verified
- Architecture: HIGH -- patterns derived from existing codebase + official Fiber/Redis docs
- Pitfalls: HIGH -- SSE disconnect detection verified against Fiber GitHub issues; write-then-notify is a well-known pattern
- Testing: HIGH -- testcontainers-go modules have official docs with exact API

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable stack, no fast-moving dependencies)
