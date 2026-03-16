# Architecture Patterns

**Domain:** Agent identity and trust protocol platform with real-time signal delivery
**Researched:** 2026-03-11
**Confidence:** HIGH (analysis based on existing codebase + build guide + established patterns for SSE/webhook systems)

## Recommended Architecture

ATAP follows a **hub-and-spoke signal broker** pattern: a central platform receives, persists, and fans out signals to entities through multiple delivery channels. This is the correct pattern for the domain. It mirrors how push notification services (FCM, APNs), message brokers (RabbitMQ), and identity-aware relay systems (Matrix homeservers) work.

```
                        External Services
                              |
                     [Inbound Channels]
                              |
                              v
Agents/SDKs ---[REST API]--> ATAP Platform ---[SSE]--------> Agents/SDKs
                              |    |    |
                              |    |    +--[Webhook Push]--> Agent Endpoints
                              |    |
                              |    +-------[FCM/APNs]------> Mobile App
                              |
                         [PostgreSQL]  [Redis Pub/Sub]
                         (durability)  (real-time fan-out)
```

### System Topology (Phase 1)

```
+-------------------+       +-------------------+       +-------------------+
|   Flutter App     |       |   Agent (SDK)     |       | External Service  |
|   (Human client)  |       |   (Python/JS/Go)  |       | (GitHub, Stripe)  |
+--------+----------+       +--------+----------+       +--------+----------+
         |                           |                            |
         | HTTPS + SSE               | HTTPS + SSE                | HTTPS POST
         | FCM/APNs push             |                            | (to channel URL)
         |                           |                            |
+--------v-----------v---------------v----------------------------v----------+
|                           ATAP Platform (Go/Fiber)                         |
|                                                                            |
|  +-------------+  +----------------+  +----------------+  +-----------+   |
|  | Auth        |  | Signal Router  |  | Channel        |  | Entity    |   |
|  | Middleware   |  | (send/persist/ |  | Manager        |  | Registry  |   |
|  | (token hash) |  |  fan-out)     |  | (inbound URLs) |  | (CRUD)    |   |
|  +------+------+  +-------+--------+  +-------+--------+  +-----+-----+   |
|         |                 |                    |                  |         |
|  +------v-----------------v--------------------v------------------v------+  |
|  |                     Store Layer (pgx pool)                           |  |
|  +----------------------------------+-----------------------------------+  |
|                                     |                                      |
+-------------------------------------+--------------------------------------+
                                      |
              +-----------------------+------------------------+
              |                                                |
    +---------v----------+                          +----------v---------+
    |   PostgreSQL 16    |                          |     Redis 7        |
    |                    |                          |                    |
    | entities           |                          | pub/sub channels:  |
    | signals            |                          |   inbox:{entityId} |
    | channels           |                          |                    |
    | delegations (P2)   |                          |                    |
    | claims (P2)        |                          |                    |
    +--------------------+                          +--------------------+
```

## Component Boundaries

| Component | Responsibility | Communicates With | Current State |
|-----------|---------------|-------------------|---------------|
| **Auth Middleware** | Token validation via SHA-256 hash lookup; injects entity into request context | Store (token lookup), all protected handlers | Implemented in `api.go` |
| **Entity Registry** | Registration, lookup, metadata management for all entity types | Store (CRUD), Crypto (keypair generation) | Implemented (agent-only) |
| **Signal Router** | Accept signals, persist to PostgreSQL, publish to Redis for fan-out, trigger secondary delivery | Store (save), Redis (publish), Push Manager (future) | Implemented |
| **SSE Streamer** | Long-lived HTTP connections, Redis subscription per connection, Last-Event-ID replay from PostgreSQL, 30s heartbeat | Redis (subscribe), Store (replay query) | Implemented |
| **Channel Manager** | Create/list/revoke inbound webhook URLs; accept external payloads and wrap as ATAP signals | Store (CRUD), Signal Router (internal signal creation) | Implemented |
| **Crypto Module** | Ed25519 keypair generation, signing, verification; token generation; ID generation (ULID-based) | None (pure functions) | Implemented |
| **Store Layer** | All PostgreSQL access via pgx connection pool; single struct with methods per domain | PostgreSQL | Implemented (single file) |
| **Config** | Environment variable loading with defaults | None | Implemented |
| **Push Manager** | FCM/APNs delivery when signals arrive for entities with push tokens | Store (push token lookup), Firebase Admin SDK | Not yet implemented |
| **Webhook Pusher** | Outbound webhook delivery with signature and exponential backoff retry | HTTP client, Store (delivery tracking) | Not yet implemented |
| **Delivery Manager** | Orchestrates which delivery methods fire for a given signal (SSE is always via Redis; conditionally push + webhook) | Signal Router, Push Manager, Webhook Pusher | Not yet implemented |

## Data Flow

### Flow 1: Agent-to-Agent Signal (primary path)

```
1. Agent A sends POST /v1/inbox/{agentB-id}
   Headers: Authorization: Bearer atap_...
   Body: { "data": {...}, "type": "application/json" }

2. Auth middleware:
   - Extract token from Authorization header
   - SHA-256 hash the token
   - Look up entity by token_hash in PostgreSQL
   - Inject entity into c.Locals("entity")

3. Signal Router (SendSignal handler):
   - Verify target entity exists in PostgreSQL
   - Build Signal struct with ULID-based ID, route metadata, timestamp
   - INSERT into signals table (PostgreSQL) -- DURABILITY POINT
   - PUBLISH to Redis channel "inbox:{targetId}" -- REAL-TIME FAN-OUT

4. If Agent B has active SSE connection:
   - Redis subscriber on "inbox:{agentB-id}" receives message
   - SSE handler writes: event: signal\nid: sig_...\ndata: {...}\n\n
   - Agent B receives signal in <100ms

5. If Agent B is offline:
   - Signal persists in PostgreSQL
   - Next time Agent B connects via SSE with Last-Event-ID, missed signals replay
   - Or Agent B polls GET /v1/inbox/{agentB-id}?after=sig_...
```

### Flow 2: External Service via Inbound Channel

```
1. Entity creates channel: POST /v1/entities/{id}/channels
   Response: { "webhook_url": "https://api.atap.app/v1/channels/chn_8f3a/signals" }

2. Entity gives webhook_url to external service (GitHub, Stripe, etc.)

3. External service POSTs JSON to the channel URL (no auth required)

4. Channel handler:
   - Look up channel by ID
   - Verify channel is active and not expired
   - Wrap raw JSON payload into ATAP signal envelope
     (origin: "external", target: entity URI, source: "webhook")
   - Save signal + publish to Redis
   - Increment channel signal_count

5. Entity receives signal via SSE/poll/push like any other signal
```

### Flow 3: SSE Reconnection (reliability)

```
1. Agent connects: GET /v1/inbox/{id}/stream
   Headers: Last-Event-ID: sig_01HQ3K9X8W

2. SSE handler:
   a. Query PostgreSQL: SELECT * FROM signals WHERE target=$1 AND id > $2
   b. Write all missed signals as SSE events (replay)
   c. Subscribe to Redis channel "inbox:{entityId}"
   d. Enter event loop:
      - Redis message -> write SSE event
      - 30s tick -> write heartbeat comment
      - Context cancelled -> close

3. If connection drops:
   - Browser/SDK auto-reconnects with Last-Event-ID = last received signal ID
   - Step 2 replays anything missed during disconnection
```

### Flow 4: Mobile Push (Phase 1 -- to be implemented)

```
1. Signal arrives for entity with push_token set

2. Signal Router (after PostgreSQL save + Redis publish):
   - Check if target entity has push_token
   - If yes, send FCM/APNs notification:
     Title: "New signal from {origin}"
     Body: signal preview
     Data: { signal_id, entity_id }

3. Mobile app receives push:
   - If foreground: SSE already delivering, push is supplementary
   - If background: push wakes app, app fetches signal via API
```

## Patterns to Follow

### Pattern 1: Write-Then-Fan-Out (Signal Delivery)

**What:** Always persist to PostgreSQL first, then publish to Redis for real-time delivery. Never rely on Redis alone for durability.

**Why:** Redis pub/sub is fire-and-forget. If no subscriber is listening, the message is lost. PostgreSQL is the source of truth. Redis is the notification channel.

**Current implementation follows this correctly:**
```go
// 1. Durable write
if err := h.store.SaveSignal(c.Context(), sig); err != nil { ... }

// 2. Best-effort real-time fan-out
sigJSON, _ := json.Marshal(sig)
h.redis.Publish(c.Context(), "inbox:"+targetID, string(sigJSON))
```

**Rule:** If Redis publish fails, the signal is still safe in PostgreSQL. The recipient will get it on next poll or SSE reconnect. Do not fail the request if Redis is down.

### Pattern 2: Token-Hash Authentication

**What:** Store SHA-256 hash of bearer tokens, never plaintext. Look up by hash on every request.

**Why:** If the database is compromised, tokens cannot be recovered. This is industry standard (GitHub, Stripe use the same approach).

**Already implemented correctly.** The `atap_` prefix on tokens aids debugging (you can identify it's an ATAP token without revealing the secret).

### Pattern 3: Cursor-Based Pagination with ULID

**What:** Use signal IDs (ULIDs) as cursors. `?after=sig_01HQ3K9X8W` returns signals with IDs lexicographically greater.

**Why:** ULIDs are time-sorted. `id > $cursor` is a simple, index-friendly query. No offset counting, no skipped/duplicated rows.

**Already implemented.** One inconsistency to fix: when `afterID` is empty, the current code orders by `created_at DESC` but when cursor is present, it orders by `id ASC`. Both should order by `id ASC` for consistent cursor behavior (newest-first is fine for the initial "latest N" query, but the API semantics should be consistent).

### Pattern 4: Handler-Store Separation (No Business Logic in Store)

**What:** Handlers contain all business logic (validation, authorization, signal construction). Store methods are pure data access (SQL queries, scans).

**Why:** Keeps the store testable with simple assertions. Business logic changes don't require touching SQL. Store can be swapped (e.g., for testing with in-memory store).

**Already followed.** Keep it this way. Resist the temptation to add authorization checks or signal construction into store methods.

### Pattern 5: RFC 7807 Problem Details for All Errors

**What:** Every error response follows RFC 7807 format with `type`, `title`, `status`, `detail`, `instance`.

**Why:** Consistent error format across all endpoints. Machine-parseable. Industry standard.

**Already implemented** via the `problem()` helper function.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Monolithic Handler File

**What:** All handlers in a single `api.go` file (current state: 540 lines covering health, registration, inbox, channels, entities, auth, and error helpers).

**Why bad:** As Phase 2 adds claims, delegations, attestations, and templates, this file will balloon to 2000+ lines. Finding and modifying specific handlers becomes painful.

**Instead:** Split into separate files per domain as the build guide prescribes:
- `api/health.go` -- health check
- `api/entities.go` -- registration, lookup, update
- `api/inbox.go` -- send, poll, stream
- `api/channels.go` -- CRUD + inbound webhook
- `api/auth.go` -- middleware + token rotation
- `api/claims.go` (Phase 2)
- `api/delegations.go` (Phase 2)

The `Handler` struct and `SetupRoutes` can stay in a shared `api/routes.go` or `api/handler.go`.

### Anti-Pattern 2: Inline Redis Pub/Sub in SSE Handler

**What:** The SSE handler directly creates a Redis subscription per HTTP connection (current state).

**Why problematic at scale:** Each SSE connection creates a Redis subscription. At 10K concurrent SSE connections, that's 10K Redis subscriptions. Redis handles this, but it's not the most efficient pattern.

**Instead (when needed):** Extract a `delivery.SSEManager` that:
- Maintains a single Redis subscription per entity (not per HTTP connection)
- Fans out to multiple SSE connections for the same entity
- Manages connection lifecycle and cleanup

**Timing:** The current inline approach is fine for Phase 1. Extract when you hit ~1000 concurrent connections.

### Anti-Pattern 3: Missing Delivery Orchestration

**What:** Currently, signal delivery only publishes to Redis. No webhook push delivery, no mobile push, no delivery tracking.

**Why bad:** The build guide specifies multiple delivery methods (SSE, webhook, push). Without a delivery manager, each handler must independently check delivery preferences and trigger the right channels.

**Instead:** Build a `delivery.Manager` that the signal router calls after persisting:
```go
// In signal handler, after store.SaveSignal:
h.delivery.Deliver(ctx, targetEntity, signal)
// Manager internally: always Redis pub, conditionally push, conditionally webhook
```

### Anti-Pattern 4: Storing Signals Without Expiry Cleanup

**What:** Signals with TTL have `expires_at` set, but no cleanup mechanism exists.

**Why bad:** The signals table will grow unbounded. Expired signals waste storage and slow queries.

**Instead:** Implement a periodic cleanup goroutine (or use PostgreSQL's `pg_cron`):
```sql
DELETE FROM signals WHERE expires_at IS NOT NULL AND expires_at < NOW();
```
Run every hour. Consider partitioning the signals table by month if volume is high.

## Component Dependency Graph and Build Order

```
Level 0 (no deps):     Config, Crypto, Models
                              |
Level 1 (needs L0):    Store (needs Models, Config for DB URL)
                              |
Level 2 (needs L1):    Auth Middleware (needs Store for token lookup)
                              |
Level 3 (needs L2):    Entity Registry (needs Store, Crypto, Auth)
                        Signal Router (needs Store, Redis, Auth)
                              |
Level 4 (needs L3):    SSE Streamer (needs Store, Redis, Auth)
                        Channel Manager (needs Store, Signal Router, Auth)
                              |
Level 5 (needs L3):    Delivery Manager (needs Signal Router, Push, Webhook)
                        Push Manager (needs Firebase SDK, Store)
                        Webhook Pusher (needs HTTP client, Crypto for signatures)
```

### Suggested Build Order for Phase 1

The existing codebase has Levels 0-4 implemented in a single pass. The remaining work:

1. **Split handler file** into per-domain files (structural, no new logic)
2. **Delivery Manager** -- orchestrate SSE + webhook + push
3. **Webhook Pusher** -- outbound delivery with retry
4. **Push Manager** -- FCM/APNs integration
5. **Signal validation** -- enforce schema, content-type checks
6. **Rate limiting** -- per-entity, per-endpoint
7. **Flutter app foundation** -- connects to existing API

**Rationale for this order:**
- Steps 1 is structural cleanup that makes everything after easier
- Steps 2-4 complete the delivery pipeline (core value of Phase 1)
- Step 5 hardens the system against malformed input
- Step 6 protects the system from abuse
- Step 7 can proceed in parallel with steps 2-6 since the API surface is already defined

## Scalability Considerations

| Concern | At 100 entities | At 10K entities | At 100K entities |
|---------|-----------------|-----------------|-------------------|
| **SSE connections** | Direct Redis sub per connection; trivial | Still fine (Redis handles 10K subs) | Need SSEManager with shared subscriptions; consider Redis Streams |
| **Signal throughput** | Single PostgreSQL instance, no concerns | Index on `target` + `created_at` handles it; monitor query plans | Partition signals table by time; read replicas for poll queries |
| **Webhook delivery** | In-process goroutine retry is fine | Need persistent retry queue (Redis list or separate table) | Dedicated webhook worker process; separate from API server |
| **Push notifications** | Direct FCM calls in-process | Batch FCM sends (up to 500/request) | Dedicated push worker; rate limit per FCM project limits |
| **Token auth** | Hash lookup with index; <1ms | Same; B-tree index on `token_hash` is fast | Consider Redis cache for hot tokens (TTL 5min) |
| **Database connections** | pgx pool default (4 conns) fine | Pool size 20-50; monitor connection wait times | PgBouncer for connection pooling; read replicas |

## Phase 2+ Architecture Implications

### Delegation Verification (Phase 2)
Adds a `verify/` package that is stateless -- it takes a delegation document, looks up public keys, and validates the chain. This should be a pure function that can work offline with cached keys. No new infrastructure needed.

### Claim Flow (Phase 2)
Adds a time-limited state machine (pending -> approved/declined/expired). The claim itself is just a row in `claims` table. The approval triggers delegation minting and a signal to the agent's inbox. Reuses existing signal delivery infrastructure.

### Federation (Phase 4)
Major architectural change: key discovery across registries. Adds DNS TXT lookups and `.well-known/atap.json` fetching. This is a new `federation/` package that the entity registry calls when it encounters a non-local entity URI. Design for this later; the current single-registry model is correct for Phase 1-3.

## Sources

- Build guide: `ATAP-BUILD-GUIDE.md` (primary source for all architecture decisions)
- Existing codebase: `platform/internal/` (6 Go source files implementing the core)
- SSE specification: W3C Server-Sent Events (https://html.spec.whatwg.org/multipage/server-sent-events.html) -- Last-Event-ID reconnection is a browser standard
- Redis Pub/Sub documentation: Redis pub/sub is fire-and-forget by design, confirming the write-then-fan-out pattern is necessary
- RFC 7807 Problem Details: https://datatracker.ietf.org/doc/html/rfc7807
- Ed25519 in Go stdlib: `crypto/ed25519` -- no external crypto dependencies needed for signing
