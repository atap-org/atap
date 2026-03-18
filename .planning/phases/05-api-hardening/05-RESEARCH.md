# Phase 5: API Hardening - Research

**Researched:** 2026-03-18
**Domain:** Go/Fiber middleware, Redis rate limiting, PostgreSQL config tables
**Confidence:** HIGH

## Summary

Phase 5 adds IP-based rate limiting middleware to all ATAP API endpoints. The codebase already has two working Redis INCR rate limiters (fan-out in `approvals.go` and OTP in `credentials.go`). This phase generalises that pattern into a Fiber middleware that runs before route handlers, tiered by endpoint group (public vs. authenticated), with thresholds and an IP allowlist stored in a new database config table and cached in-memory.

The key implementation decision is fail-closed on Redis unavailability (diverges from the existing fail-open pattern). This is a deliberate security-hardening tradeoff. The middleware must also inject rate limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`) on every response, not just 429s.

The entire feature is expressible as: one new migration, one new store method, one middleware file, and wiring changes in `SetupRoutes`. No new third-party dependencies are required.

**Primary recommendation:** Build a standalone `RateLimitMiddleware` function in `platform/internal/api/ratelimit.go` that closes over a `*rateLimitConfig` cache struct; wire it into `SetupRoutes` at line 138 before the route group definitions, applying it to all routes except `/v1/health` and `/.well-known/atap.json`.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Rate limit scope & thresholds**
- Tiered rate limits: public endpoints get stricter limits, authenticated endpoints get more generous limits
- Per-group buckets (not per-endpoint): one bucket for public endpoints, one for authenticated endpoints per IP
- Public tier: 30 requests/minute per IP
- Authenticated tier: higher limit (Claude's discretion on exact multiplier, suggest 2-4x public)
- Thresholds stored in database config, not env vars or hardcoded constants
- Config cached in-memory on startup with periodic refresh interval (not queried per request)

**Response behavior**
- Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After) on ALL responses, not just 429s
- Retry-After value reflects actual seconds until the rate limit window resets (dynamic, not fixed)
- 429 response body follows existing RFC 7807 pattern using problem() helper with type "rate-limit-exceeded" — consistent with fan-out limiter (approvals.go:233)
- Rate-limited requests logged at warn level with IP, endpoint group, and current count — matches existing zerolog conventions

**Bypass & allowlisting**
- Health endpoint (/v1/health) and discovery (/.well-known/atap.json) are exempt from rate limiting
- DID document resolution (/:type/:id/did.json) is NOT exempt — included in public tier
- Server DID resolution (/server/platform/did.json) follows same rule as other DID resolution
- Configurable IP/CIDR allowlist stored in database config alongside thresholds — bypasses rate limiting entirely
- Client IP determined via Fiber's c.IP() — trust the framework's existing proxy header config

**Storage & expiry**
- If Redis is unavailable: fail closed (reject requests with 503) — diverges from existing fan-out pattern which fails open
- Fixed window algorithm: Redis INCR + EXPIRE per minute window — matches existing fan-out and OTP rate limit patterns
- Rate limit config (thresholds + allowlist) cached in-memory with periodic refresh from database

### Claude's Discretion
- Exact authenticated tier multiplier (suggested 2-4x the public 30 req/min)
- In-memory config refresh interval
- Redis key naming scheme for IP rate limit counters
- Database schema for rate limit configuration table
- 503 response body when Redis is down
- Credential status list endpoint (/v1/credentials/status/:listId) grouping — public or exempt

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| API-07 | API endpoints enforce IP-based rate limiting per client | Redis INCR fixed-window middleware, tiered by endpoint group (public/authenticated), thresholds from DB config table, fail-closed on Redis unavailability |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/gofiber/fiber/v2` | v2 (already in use) | HTTP framework, middleware chain | Project standard; `c.IP()`, `c.Locals()`, middleware handler signatures already used |
| `github.com/redis/go-redis/v9` | v9 (already in use) | Redis INCR + EXPIRE for rate counters | Already injected into `Handler.redis`; identical to fan-out and OTP patterns |
| `github.com/jackc/pgx/v5` | v5 (already in use) | PostgreSQL for rate limit config table | Project standard; pool already in `store.Store` |
| `github.com/rs/zerolog` | already in use | Structured warn logging | Project standard; `h.log.Warn()` used consistently |

No new dependencies are required. This feature uses only what is already present.

**Installation:** No new packages to install.

---

## Architecture Patterns

### Recommended Project Structure

The new code spans three files:

```
platform/
├── migrations/
│   └── 015_rate_limit_config.up.sql    # new table: rate_limit_config
│   └── 015_rate_limit_config.down.sql
├── internal/
│   ├── api/
│   │   ├── ratelimit.go                # new: middleware + config cache
│   │   ├── ratelimit_test.go           # new: middleware unit tests
│   │   └── api.go                      # modified: wire middleware in SetupRoutes
│   └── store/
│       └── store.go                    # modified: add GetRateLimitConfig()
```

### Pattern 1: Fixed-Window Redis INCR

The existing fan-out rate limiter (`approvals.go:169-198`) and OTP rate limiter (`credentials.go:69-84`) both use the same idiom: `INCR` the key, then `EXPIRE` on first increment. This middleware generalises that pattern to IP + endpoint group.

**Key naming convention (Claude's discretion):** `rl:ip:{group}:{ip}:{window_minute}`

Where `window_minute` is `time.Now().Unix() / 60` — gives a clean per-minute fixed window without needing separate TTL math.

Example Redis key: `rl:ip:public:203.0.113.42:28938172`

```go
// Source: adapted from platform/internal/api/approvals.go:177-196
rateKey := fmt.Sprintf("rl:ip:%s:%s:%d", group, ip, time.Now().Unix()/60)
count, err := h.redis.Incr(ctx, rateKey).Result()
if err != nil {
    // fail closed — return 503
    return problem503(c)
}
if count == 1 {
    h.redis.Expire(ctx, rateKey, 2*time.Minute) // 2x window for safety
}
```

Using `time.Now().Unix()/60` as the window suffix avoids a second `TTL` call to compute `Retry-After` — you can derive remaining seconds as `(window+1)*60 - time.Now().Unix()`.

### Pattern 2: Database Config Table with In-Memory Cache

The database config table stores key-value pairs for thresholds and the allowlist. Loaded once at startup and refreshed on a background ticker.

**Recommended schema (Claude's discretion):**

```sql
-- 015_rate_limit_config.up.sql
CREATE TABLE rate_limit_config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO rate_limit_config (key, value) VALUES
    ('public_rpm',     '30'),
    ('auth_rpm',       '120'),   -- 4x public
    ('ip_allowlist',   '[]');    -- JSON array of CIDRs
```

A single `GetRateLimitConfig(ctx) (*RateLimitConfig, error)` method on `store.Store` reads all rows and builds the struct. A background goroutine in the middleware (or at server startup) refreshes the in-memory copy every N seconds (see discretion section).

```go
// Config cache struct
type rateLimitConfig struct {
    mu          sync.RWMutex
    publicRPM   int
    authRPM     int
    allowlist   []*net.IPNet
    refreshedAt time.Time
}
```

### Pattern 3: Fiber Middleware Registration

Fiber middleware is a `fiber.Handler` — `func(*fiber.Ctx) error`. Exempt paths are checked at the top of the middleware by matching `c.Path()`.

```go
// Source pattern: platform/internal/api/api.go:137-195
func (h *Handler) SetupRoutes(app *fiber.App) {
    // Rate limit middleware applies to all routes registered after this line.
    // The middleware itself short-circuits on exempt paths.
    app.Use(h.RateLimitMiddleware(cfg))

    app.Get("/.well-known/atap.json", h.Discovery)
    // ... rest of routes unchanged
}
```

The middleware must determine endpoint group (public vs. authenticated). The cleanest approach: check whether a `DPoP` or `Authorization` header is present. If yes, use the authenticated bucket. If no, use the public bucket. This avoids needing to duplicate the route table.

### Pattern 4: RFC 7807 429 Response

Consistent with existing 429 in `respondWithFanOutRateLimitError` (`approvals.go:231-239`):

```go
// Source: platform/internal/api/approvals.go:231-238
return c.Status(fiber.StatusTooManyRequests).JSON(models.ProblemDetail{
    Type:     "https://atap.dev/errors/rate-limit-exceeded",
    Title:    "Rate limit exceeded",
    Status:   fiber.StatusTooManyRequests,
    Detail:   "Too many requests from this IP address. Try again later.",
    Instance: c.Path(),
}, mimeApplicationProblemJSON)
```

### Pattern 5: Rate Limit Headers on All Responses

Fiber's `c.Set()` injects headers before `c.Next()` returns. Set the headers after the INCR result but before calling `c.Next()`:

```go
limit := cfg.publicRPM // or authRPM
windowEnd := (time.Now().Unix()/60 + 1) * 60
remaining := max(0, int64(limit)-count)
retryAfter := windowEnd - time.Now().Unix()

c.Set("X-RateLimit-Limit", strconv.FormatInt(int64(limit), 10))
c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
c.Set("Retry-After", strconv.FormatInt(retryAfter, 10))

return c.Next()
```

For 429 responses the same headers apply — the `Retry-After` value is dynamic (seconds to window reset), not a fixed number.

### Anti-Patterns to Avoid

- **Querying DB per request for rate limit config:** The config must be cached in-memory. Per-request DB calls would add ~1ms per request and negate the "no latency impact" success criterion.
- **Using Fiber's built-in limiter package (`github.com/gofiber/contrib/limiter`):** It does not support database-backed config or CIDR allowlists. Build directly on Redis INCR per the existing codebase pattern.
- **Setting `Retry-After` to a fixed value:** The requirement specifies dynamic seconds until window reset. Use `(time.Now().Unix()/60 + 1) * 60 - time.Now().Unix()`.
- **Failing open on Redis error (for this middleware):** Existing limiters fail open but this phase explicitly chooses fail-closed. The 503 response signals infrastructure failure, not client error.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis connection | Custom connection logic | Existing `h.redis *redis.Client` | Already injected, tested, reconnecting |
| CIDR matching | Custom IP/mask parsing | `net.ParseCIDR` + `(*net.IPNet).Contains` from stdlib | Handles IPv4, IPv6, /32, exact IPs |
| Atomic increment | Lua script or transactions | Redis `INCR` | Atomic by nature on single key |
| HTTP error format | Custom JSON shape | `models.ProblemDetail` + `mimeApplicationProblemJSON` | Ensures RFC 7807 compliance, matches all other errors |

**Key insight:** The codebase already contains two working Redis rate limiters. This phase is about generalising them to middleware, not inventing a new pattern.

---

## Common Pitfalls

### Pitfall 1: Race Between INCR and EXPIRE
**What goes wrong:** If the process crashes between `INCR` and `Expire`, the key has no TTL and the rate counter never resets — IPs get permanently blocked.
**Why it happens:** INCR and EXPIRE are two separate commands; not atomic.
**How to avoid:** Use the `count == 1` guard (set TTL only on first increment, with NX semantics) as already done in `approvals.go:188-191`. The key will expire naturally. Alternatively use a Lua script, but the existing pattern is proven.
**Warning signs:** Redis keys for old windows persisting beyond 2 minutes.

### Pitfall 2: IPv6 Address Normalization
**What goes wrong:** `c.IP()` may return IPv4-mapped IPv6 (`::ffff:1.2.3.4`) or full IPv6. Rate limit keys include the IP string — without normalization, the same client could get different keys.
**Why it happens:** Go's `net.ParseIP` normalizes IPv4-mapped IPv6 to IPv4 when calling `.String()`.
**How to avoid:** Normalize with `net.ParseIP(c.IP()).String()` before building the Redis key.
**Warning signs:** Rate limit not triggering for a known IP.

### Pitfall 3: X-Forwarded-For Trust
**What goes wrong:** An attacker sends a spoofed `X-Forwarded-For: 1.2.3.4` header and bypasses rate limiting for their real IP.
**Why it happens:** If Fiber is configured to trust proxy headers and the proxy chain is not validated, any client can inject this header.
**How to avoid:** The decision is to use `c.IP()` and trust Fiber's existing proxy header configuration. No change is needed — but it is important to verify that Fiber's `ProxyHeader` config is set correctly for the deployment environment.
**Warning signs:** Rate limit keys using IPs from the `X-Forwarded-For` header that don't match the TCP remote address.

### Pitfall 4: Exempt Path Matching Too Broad/Narrow
**What goes wrong:** Exempt `/.well-known/atap.json` and `/v1/health` but accidentally exempt paths like `/v1/health/extra` or miss the DID resolution path (`/:type/:id/did.json`).
**Why it happens:** String prefix matching catches too much; exact matching misses dynamic paths.
**How to avoid:** Use exact string comparison for the two exempt paths. DID resolution (`/:type/:id/did.json`) is deliberately NOT exempt — confirmed in CONTEXT.md.
**Warning signs:** Rate limit test showing non-exempt paths not being counted.

### Pitfall 5: Background Refresh Goroutine Leak
**What goes wrong:** Starting a `time.Ticker` in the middleware factory function results in a goroutine that never stops, causing leaks in tests.
**Why it happens:** Middleware is created per-test if using `NewHandler` test helpers.
**How to avoid:** Accept a `context.Context` or `done <-chan struct{}` for the refresh goroutine, or start the refresh ticker at server startup (in `cmd/server/main.go`) rather than inside the middleware constructor.
**Warning signs:** Test suite hangs or goroutine count increases across test runs.

---

## Code Examples

### INCR + EXPIRE Pattern (verified from existing codebase)
```go
// Source: platform/internal/api/approvals.go:177-196
rateKey := fmt.Sprintf("fanout:rate:%s:%s", sourceDID, orgDID)
count, err := h.redis.Incr(ctx, rateKey).Result()
if err != nil {
    h.log.Warn().Err(err).Str("key", rateKey).Msg("rate limit: Redis INCR failed")
    return nil // fan-out fails open; IP middleware will fail closed
}
if count == 1 {
    h.redis.Expire(ctx, rateKey, time.Hour)
}
if count > limit {
    return &rateLimitError{}
}
```

### RFC 7807 429 Response Pattern (verified from existing codebase)
```go
// Source: platform/internal/api/approvals.go:231-238
return c.Status(fiber.StatusTooManyRequests).JSON(models.ProblemDetail{
    Type:     "https://atap.dev/errors/rate-limit-exceeded",
    Title:    "Rate limit exceeded",
    Status:   fiber.StatusTooManyRequests,
    Detail:   "Too many requests. Try again later.",
    Instance: c.Path(),
}, mimeApplicationProblemJSON)
```

### CIDR Allowlist Check (stdlib)
```go
// Standard library — no import needed beyond "net"
func ipAllowed(ip string, allowlist []*net.IPNet) bool {
    parsed := net.ParseIP(ip)
    if parsed == nil {
        return false
    }
    for _, cidr := range allowlist {
        if cidr.Contains(parsed) {
            return true
        }
    }
    return false
}
```

### Fiber Middleware Skeleton
```go
// platform/internal/api/ratelimit.go
func (h *Handler) RateLimitMiddleware(cfg *rateLimitConfig) fiber.Handler {
    return func(c *fiber.Ctx) error {
        path := c.Path()
        // Exempt paths
        if path == "/v1/health" || path == "/.well-known/atap.json" {
            return c.Next()
        }

        ip := net.ParseIP(c.IP()).String()

        // Check allowlist
        cfg.mu.RLock()
        allowed := ipAllowed(ip, cfg.allowlist)
        publicRPM := cfg.publicRPM
        authRPM := cfg.authRPM
        cfg.mu.RUnlock()

        if allowed {
            return c.Next()
        }

        // Determine group
        group := "public"
        limit := publicRPM
        if c.Get("Authorization") != "" || c.Get("DPoP") != "" {
            group = "auth"
            limit = authRPM
        }

        window := time.Now().Unix() / 60
        rateKey := fmt.Sprintf("rl:ip:%s:%s:%d", group, ip, window)

        count, err := h.redis.Incr(c.Context(), rateKey).Result()
        if err != nil {
            h.log.Error().Err(err).Str("ip", ip).Msg("rate limit: Redis unavailable")
            return c.Status(fiber.StatusServiceUnavailable).JSON(models.ProblemDetail{
                Type:   "https://atap.dev/errors/service-unavailable",
                Title:  "Service Unavailable",
                Status: fiber.StatusServiceUnavailable,
                Detail: "Rate limiting service unavailable.",
            }, mimeApplicationProblemJSON)
        }
        if count == 1 {
            h.redis.Expire(c.Context(), rateKey, 2*time.Minute)
        }

        windowEnd := (window + 1) * 60
        remaining := max(0, int64(limit)-count)
        retryAfter := windowEnd - time.Now().Unix()

        c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
        c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
        c.Set("Retry-After", strconv.FormatInt(retryAfter, 10))

        if count > int64(limit) {
            h.log.Warn().Str("ip", ip).Str("group", group).
                Int64("count", count).Msg("rate limit exceeded")
            return c.Status(fiber.StatusTooManyRequests).JSON(models.ProblemDetail{
                Type:     "https://atap.dev/errors/rate-limit-exceeded",
                Title:    "Rate limit exceeded",
                Status:   fiber.StatusTooManyRequests,
                Detail:   "Too many requests from this IP address. Try again later.",
                Instance: c.Path(),
            }, mimeApplicationProblemJSON)
        }

        return c.Next()
    }
}
```

---

## Claude's Discretion Recommendations

### Authenticated tier multiplier
**Recommendation: 4x (120 req/min)**. Public tier is 30 rpm. Authenticated clients are registered entities making legitimate API calls. 120 rpm (2 req/sec) provides generous headroom for normal usage while still bounding abuse. The 4x multiplier matches the suggested upper bound from CONTEXT.md.

### Config refresh interval
**Recommendation: 60 seconds.** Short enough to pick up allowlist changes quickly during an active abuse incident. Long enough to avoid DB polling overhead. Suitable for `time.NewTicker(60 * time.Second)`.

### Redis key naming
**Recommendation:** `rl:ip:{group}:{normalized_ip}:{window_minute}`
- Short prefix `rl:ip:` distinguishes from existing `fanout:rate:` and `otp:rate:` keys
- Including `group` means public and authenticated counters are separate (correct per tiering decision)
- Window minute (`time.Now().Unix()/60`) gives natural key expiry without a separate TTL call

### Database schema
**Recommendation:** Key-value table with known keys. Simple to query (`SELECT key, value FROM rate_limit_config`), easy to update via SQL without schema changes, consistent with the "no frameworks beyond Fiber" principle.

```sql
-- 015_rate_limit_config.up.sql
CREATE TABLE rate_limit_config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO rate_limit_config (key, value) VALUES
    ('public_rpm',   '30'),
    ('auth_rpm',     '120'),
    ('ip_allowlist', '[]');
```

### 503 response body
**Recommendation:** Use `models.ProblemDetail` with type `service-unavailable` (consistent with RFC 7807 pattern). Do not use the `problem()` helper directly since 503 is not currently in the switch in `GlobalErrorHandler` — write inline like `respondWithFanOutRateLimitError` does.

### Credential status list endpoint grouping
**Recommendation:** Public tier. `GET /v1/credentials/status/:listId` is queried by verifiers without auth, identical to `GET /v1/revocations`. Apply public tier rate limiting. Not exempt.

---

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|------------------|-------|
| Per-handler rate checks (fan-out, OTP) | Global middleware | Phase 5 generalises existing handler-level checks to a single middleware |
| Fail open on Redis error | Fail closed (this phase) | Security-hardening phase accepts availability sacrifice |
| Hardcoded constants (`fanOutRateLimit = 10`) | DB-backed config | Allows threshold changes without redeployment |

---

## Open Questions

1. **Fiber app-level `Use` vs. route-group-level `Use`**
   - What we know: `app.Use()` applies to all routes including Fiber's built-in 404 handler. `v1.Use()` applies only within the `/v1` group.
   - What's unclear: Whether middleware should apply to `/.well-known/atap.json` and `/:type/:id/did.json` (outside `/v1`). The exempt list only covers `/v1/health` and `/.well-known/atap.json`, so DID resolution outside `/v1` must also be covered.
   - Recommendation: Use `app.Use()` (global) and handle exemptions by path string inside the middleware. This ensures DID resolution endpoints are covered without duplicating registration.

2. **GlobalErrorHandler coverage for 503**
   - What we know: `GlobalErrorHandler` (`api.go:231-265`) handles standard Fiber error codes but has no 503 case.
   - What's unclear: Whether the planner should add a 503 case to `GlobalErrorHandler` or keep 503 responses inline in the middleware.
   - Recommendation: Keep 503 inline in the middleware (consistent with how fan-out and OTP 429s are handled inline). No change to `GlobalErrorHandler` needed.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) |
| Config file | None — Go tests are file-adjacent (`*_test.go`) |
| Quick run command | `cd /Users/svenloth/dev/atap/platform && go test ./internal/api/ -run TestRateLimit -v` |
| Full suite command | `cd /Users/svenloth/dev/atap/platform && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| API-07 | Public IP exceeds 30 rpm — receives 429 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_PublicExceeded -v` | Wave 0 |
| API-07 | Authenticated IP exceeds 120 rpm — receives 429 | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_AuthExceeded -v` | Wave 0 |
| API-07 | Requests within limit — 200 with headers | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_WithinLimit -v` | Wave 0 |
| API-07 | X-RateLimit-Limit, Remaining, Retry-After present | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_Headers -v` | Wave 0 |
| API-07 | IP in allowlist — not rate limited | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_Allowlist -v` | Wave 0 |
| API-07 | /v1/health exempt from rate limiting | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_HealthExempt -v` | Wave 0 |
| API-07 | /.well-known/atap.json exempt | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_WellKnownExempt -v` | Wave 0 |
| API-07 | Redis unavailable — 503 returned | unit | `go test ./internal/api/ -run TestRateLimitMiddleware_RedisDown -v` | Wave 0 |

Note: Existing Redis-dependent tests use `t.Skip` when Redis is unavailable (`approvals_test.go:420-421`). The same pattern applies here. Tests that mock Redis (using miniredis or a mock client) can avoid the skip.

### Sampling Rate
- **Per task commit:** `cd /Users/svenloth/dev/atap/platform && go test ./internal/api/ -run TestRateLimit -v`
- **Per wave merge:** `cd /Users/svenloth/dev/atap/platform && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `platform/internal/api/ratelimit_test.go` — covers all API-07 test cases above
- [ ] `platform/migrations/015_rate_limit_config.up.sql` — schema and seed data
- [ ] `platform/migrations/015_rate_limit_config.down.sql` — teardown

*(No new framework install needed — uses existing Go `testing` + `go-redis/v9` client)*

---

## Sources

### Primary (HIGH confidence)
- Codebase: `platform/internal/api/approvals.go:169-256` — Redis INCR pattern, RFC 7807 429, fail-open
- Codebase: `platform/internal/api/credentials.go:69-84` — OTP INCR pattern with per-entity key
- Codebase: `platform/internal/api/api.go` — SetupRoutes, Handler struct, problem() helper, GlobalErrorHandler
- Codebase: `platform/migrations/013_credentials.up.sql`, `014_approvals_recreate.up.sql` — migration patterns
- Codebase: `platform/internal/store/store.go` — pgxpool query patterns
- Codebase: `platform/internal/api/approvals_test.go:409-490` — rate limit test pattern (Redis skip, pre-seed counter)

### Secondary (MEDIUM confidence)
- Go stdlib `net` package — `net.ParseIP`, `net.ParseCIDR`, `(*net.IPNet).Contains` for CIDR allowlist

### Tertiary (LOW confidence)
None.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, no new dependencies
- Architecture: HIGH — directly derived from two existing rate limiters in the codebase
- Pitfalls: HIGH — derived from code inspection of the actual implementation patterns and CONTEXT.md decisions

**Research date:** 2026-03-18
**Valid until:** 2026-04-17 (30 days — stable internal codebase)
