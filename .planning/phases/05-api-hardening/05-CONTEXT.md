# Phase 5: API Hardening - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Enforce IP-based rate limiting on all API endpoints to prevent abuse. Requirement API-07. Does not include per-entity rate limiting (already exists for fan-out and OTP), new auth mechanisms, or API versioning changes.

</domain>

<decisions>
## Implementation Decisions

### Rate limit scope & thresholds
- Tiered rate limits: public endpoints get stricter limits, authenticated endpoints get more generous limits
- Per-group buckets (not per-endpoint): one bucket for public endpoints, one for authenticated endpoints per IP
- Public tier: 30 requests/minute per IP
- Authenticated tier: higher limit (Claude's discretion on exact multiplier, suggest 2-4x public)
- Thresholds stored in database config, not env vars or hardcoded constants
- Config cached in-memory on startup with periodic refresh interval (not queried per request)

### Response behavior
- Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After) on ALL responses, not just 429s
- Retry-After value reflects actual seconds until the rate limit window resets (dynamic, not fixed)
- 429 response body follows existing RFC 7807 pattern using problem() helper with type "rate-limit-exceeded" — consistent with fan-out limiter (approvals.go:233)
- Rate-limited requests logged at warn level with IP, endpoint group, and current count — matches existing zerolog conventions

### Bypass & allowlisting
- Health endpoint (/v1/health) and discovery (/.well-known/atap.json) are exempt from rate limiting
- DID document resolution (/:type/:id/did.json) is NOT exempt — included in public tier
- Server DID resolution (/server/platform/did.json) follows same rule as other DID resolution
- Configurable IP/CIDR allowlist stored in database config alongside thresholds — bypasses rate limiting entirely
- Client IP determined via Fiber's c.IP() — trust the framework's existing proxy header config

### Storage & expiry
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

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing rate limiting patterns
- `platform/internal/api/approvals.go` — Fan-out rate limiter (lines 17-252): Redis INCR pattern, RFC 7807 429 response, fail-open on Redis error
- `platform/internal/api/credentials.go` — OTP rate limiter (lines 21-81): Redis INCR pattern, per-entity limiting

### API structure
- `platform/internal/api/api.go` — Route definitions (SetupRoutes), middleware chain, Handler struct, problem() helper, TODO at line 162
- `platform/internal/config/config.go` — Existing env-based configuration pattern

### Requirements
- `.planning/REQUIREMENTS.md` — API-07: API endpoints enforce IP-based rate limiting per client

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `problem()` helper (api.go:219): RFC 7807 error responses — reuse for 429 rate limit responses
- Fan-out rate limiter (approvals.go:169-252): Redis INCR + EXPIRE pattern, can be generalized into middleware
- `redis.Client` already injected into Handler struct (api.go:98)
- `GlobalErrorHandler` (api.go:231): Fiber error handler — may need 429 case added

### Established Patterns
- Redis INCR with TTL for rate counting (fan-out + OTP both use this)
- RFC 7807 Problem Details for all error responses
- Fiber middleware chain (DPoPAuthMiddleware, RequireScope) — rate limiter fits as another middleware
- zerolog structured logging with warn level for limit violations

### Integration Points
- Rate limit middleware registers in SetupRoutes() — before route handlers, after Fiber app creation
- Database config table needs migration in platform/migrations/
- Handler struct may need rate limiter dependency or middleware can be standalone with its own Redis reference

</code_context>

<specifics>
## Specific Ideas

- Fail closed on Redis outage is a deliberate choice — this is a security-hardening phase, availability sacrifice is acceptable
- Database-backed config allows changing thresholds without redeployment — important for responding to active abuse
- IP allowlist covers internal services and monitoring that shouldn't consume rate limit budget

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-api-hardening*
*Context gathered: 2026-03-18*
