# Phase 1: Foundation - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

A running platform where agents can register with cryptographic identity and be looked up by other entities. Phase 1 ships 4 endpoints (health, register, entity lookup, auth middleware), crypto primitives, database migration for entities only, Docker Compose setup, and unit + API-level tests. No signal delivery, no channels, no SSE — those are Phase 2.

</domain>

<decisions>
## Implementation Decisions

### Agent key ownership
- Server generates the Ed25519 keypair and returns both public and private key in the registration response
- Private key is returned once in the 201 response JSON, never stored by the server (generate-and-forget)
- If an agent loses its private key, it re-registers as a new identity (new ULID, new URI)
- Key rotation/recovery deferred — not needed for Trust Level 0 agents in Phase 1
- For humans (Phase 2): server stores private key encrypted with user passphrase for recovery; humans can also use other forms of IDs to authenticate

### Registration response
- Minimal identity-only response: uri, id, token, public_key, private_key, key_id
- No inbox_url or stream_url — those don't exist in Phase 1
- Private key field added to RegisterResponse model

### Entity lookup response
- Identity-only: id, type, uri, public_key, key_id, name, trust_level, registry, created_at
- No delivery info, no webhook URLs, no internal fields
- Purpose is cryptographic verification, not addressing — you never send to an entity directly

### Communication model (Phase 2 context)
- All inbound communication goes through channels that the receiving entity explicitly created
- No open inbox, no unsolicited signals from unknown parties
- Channels are consent tokens with flexible lifetimes:
  - Short-lived: one-time webhook for a specific transaction
  - Long-lived: ongoing promotions, news feeds, authorized senders
- Agent always controls the relationship: create, scope, revoke
- Each use case generates a new channel with one authorized party

### Phase 1 API surface
- Ship only 4 endpoints: GET /v1/health, POST /v1/register, GET /v1/entities/{id}, auth middleware
- Remove all Phase 2+ routes (signals, SSE, channels, webhooks, verify)
- Refactor everything as needed — no half-baked stubs

### Scaffolding strategy
- Clean slate for Phase 1 scope — don't inherit untested code
- Reuse patterns/structures from scaffolding where sound
- Existing files just overwritten (git history is sufficient reference)
- Write tests alongside code, not retroactively

### Migration scope
- 001_init.sql creates only the entities table
- Phase 2 adds separate migrations (002_signals.sql, 003_channels.sql, etc.)
- Each migration matches what the code actually uses

### Test scope
- Unit tests for crypto functions (keypair generation, signing, verification, canonical JSON)
- Unit tests for token generation and hash verification
- HTTP-level tests for the 4 endpoints (register, lookup, health, auth rejection)
- Validates Phase 1 works end-to-end without testcontainers (those come in Phase 2)

### Claude's Discretion
- Go module version upgrade (1.22 → current stable)
- Dependency version choices (pgx, go-redis, zerolog, fiber, ulid, golang-migrate)
- RFC 8785 JCS implementation approach (library vs hand-rolled)
- Channel ID entropy fix (64-bit → 128-bit) — Phase 2 code, but fix the crypto function now
- Dockerfile multi-stage build details
- Docker Compose configuration specifics
- Config struct cleanup (remove Phase 2+ fields like SIMRelay, WorldID, FCM)
- Error type taxonomy for RFC 7807 responses
- Test framework choice (stdlib vs testify)

</decisions>

<specifics>
## Specific Ideas

- Communication model is consent-based: "if the agent wants Lufthansa to send messages, the agent must actively create a webhook endpoint for exactly this purpose"
- Channels as consent tokens: allow long-lived consent for things like promotions or news feeds (great trust use case)
- Entity lookup is for verification only — never for addressing

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/crypto/crypto.go`: Ed25519 key generation, token generation, ULID-based IDs — foundation is sound but needs fixes (JCS, channel entropy)
- `internal/config/config.go`: Env-based config with defaults — keep pattern, trim Phase 2+ fields
- `internal/models/models.go`: Entity, Signal, Channel structs — trim to Phase 1 (Entity + ProblemDetail + RegisterRequest/Response)
- `platform/migrations/001_init.sql`: Entities table DDL is well-structured — extract just the entities portion

### Established Patterns
- Fiber v2 HTTP framework with handler struct pattern (`api.Handler` with store, redis, config, log)
- pgx/v5 connection pool via `store.Store` wrapper
- zerolog structured JSON logging
- RFC 7807 error responses via `problem()` helper
- Bearer token auth via SHA-256 hash lookup middleware

### Integration Points
- `cmd/server/main.go` wires config → store → redis → handler → routes → graceful shutdown
- Redis is initialized in main.go but not needed for Phase 1 (no pub/sub without SSE) — may simplify to optional

</code_context>

<deferred>
## Deferred Ideas

- Human key recovery with encrypted storage and passphrase/alternative auth — Phase 2
- Long-lived channels as consent tokens for promotions/news feeds — Phase 2 (great trust use case to highlight)
- Agent-to-agent direct addressing vs channel-only model — decide in Phase 2

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-03-11*
