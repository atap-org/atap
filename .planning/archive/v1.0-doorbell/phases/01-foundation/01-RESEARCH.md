# Phase 1: Foundation - Research

**Researched:** 2026-03-11
**Domain:** Go backend — agent registration, crypto identity, Docker infrastructure
**Confidence:** HIGH

## Summary

Phase 1 is a focused foundation: 4 HTTP endpoints (health, register, entity lookup, auth middleware), Ed25519 crypto primitives, a single PostgreSQL migration for the entities table, Docker Compose infrastructure, and unit tests. The existing codebase has solid scaffolding (patterns for Fiber handlers, pgx store, zerolog, crypto utilities) but ships all-phases code that must be trimmed to Phase 1 scope only.

The main technical decisions are: (1) upgrade Go from 1.22 to 1.24 (current stable with long support), (2) stay on Fiber v2 rather than migrating to v3 (v3 requires Go 1.25+ and has significant breaking changes for minimal Phase 1 benefit), (3) use `gowebpki/jcs` for RFC 8785 canonical JSON instead of the naive `json.Marshal` currently in `CanonicalJSON()`, (4) fix channel ID entropy from 64-bit to 128-bit now, and (5) add `private_key` to the registration response per CONTEXT.md decisions.

**Primary recommendation:** Rewrite from existing scaffolding patterns, trimming to Phase 1 scope. Keep Fiber v2, upgrade Go to 1.24, add proper JCS library, run golang-migrate for database migrations.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Server generates Ed25519 keypair and returns both public and private key in registration response
- Private key returned once in 201 response JSON, never stored by server (generate-and-forget)
- Lost private key means re-register as new identity
- Registration response: uri, id, token, public_key, private_key, key_id (no inbox_url, no stream_url)
- Entity lookup response: id, type, uri, public_key, key_id, name, trust_level, registry, created_at (identity-only, no delivery info)
- Ship only 4 endpoints: GET /v1/health, POST /v1/register, GET /v1/entities/{id}, auth middleware
- Remove all Phase 2+ routes (signals, SSE, channels, webhooks, verify)
- Clean slate for Phase 1 scope -- reuse patterns, overwrite files, git history is reference
- 001_init.sql creates only the entities table
- Unit tests for crypto (keypair, signing, verification, canonical JSON) and tokens (generation, hash verification)
- HTTP-level tests for 4 endpoints
- No testcontainers in Phase 1

### Claude's Discretion
- Go module version upgrade (1.22 to current stable)
- Dependency version choices (pgx, go-redis, zerolog, fiber, ulid, golang-migrate)
- RFC 8785 JCS implementation approach (library vs hand-rolled)
- Channel ID entropy fix (64-bit to 128-bit)
- Dockerfile multi-stage build details
- Docker Compose configuration specifics
- Config struct cleanup (remove Phase 2+ fields)
- Error type taxonomy for RFC 7807 responses
- Test framework choice (stdlib vs testify)

### Deferred Ideas (OUT OF SCOPE)
- Human key recovery with encrypted storage and passphrase/alternative auth -- Phase 2
- Long-lived channels as consent tokens for promotions/news feeds -- Phase 2
- Agent-to-agent direct addressing vs channel-only model -- Phase 2
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REG-01 | Agent self-register via POST /v1/register, receive URI/token/keys in <1s | Existing handler pattern + private_key addition |
| REG-02 | Registration generates Ed25519 keypair and ULID entity ID | Existing crypto.go GenerateKeyPair + NewEntityID |
| REG-03 | Bearer token uses atap_ prefix + 32 bytes base64url, stored as SHA-256 hash | Existing crypto.go NewToken/HashToken |
| REG-04 | Entity lookup via GET /v1/entities/{id}, public endpoint, public key + metadata | Existing GetEntity handler, needs response trimming |
| REG-05 | Entity URI scheme: agent://{ulid} | Existing pattern in RegisterAgent handler |
| CRY-01 | Ed25519 keypair generation using Go stdlib crypto/ed25519 | Existing, needs unit tests |
| CRY-02 | Canonical JSON signing uses RFC 8785 (JCS) | NEEDS FIX: replace json.Marshal with gowebpki/jcs |
| CRY-03 | Signable payload: JCS(route) + "." + JCS(signal) | Existing SignablePayload, update to use JCS lib |
| CRY-04 | Channel IDs use 128-bit random entropy (chn_ + 32 hex chars) | NEEDS FIX: currently 64-bit (8 bytes) |
| AUTH-01 | Protected endpoints require Bearer token header | Existing AuthMiddleware pattern |
| AUTH-02 | Auth middleware validates token by SHA-256 hash lookup | Existing GetEntityByTokenHash |
| ERR-01 | RFC 7807 Problem Details for all errors | Existing problem() helper |
| ERR-02 | Health endpoint returns protocol version, status, timestamp | Existing Health handler |
| INF-01 | docker compose up starts full stack in <60s | Existing docker-compose.yml, needs fixes |
| INF-02 | Dockerfile produces cloud-deployable binary (multi-stage Alpine) | Existing Dockerfile, has typo (atep -> atap) |
| INF-03 | Database migrations via golang-migrate | NEW: add golang-migrate dependency |
| INF-04 | Structured JSON logging via zerolog | Existing |
| INF-05 | Graceful shutdown on SIGTERM/SIGINT | Existing in main.go |
| INF-06 | Go dependencies updated to current versions | Stale go.mod needs full update |
| TST-03 | Unit tests for crypto functions | NEW: write tests |
| TST-04 | Unit tests for token generation and hash verification | NEW: write tests |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.x | Language runtime | Current LTS-equivalent, supported until Go 1.26+2. Go 1.22 is EOL. |
| gofiber/fiber/v2 | v2.52.x | HTTP framework | Already in use. v3 requires Go 1.25+ and has major breaking changes -- not worth migration risk for 4 endpoints. |
| jackc/pgx/v5 | v5.7.x+ | PostgreSQL driver | Current stable for Go 1.24. v5.8.0 requires Go 1.24+. Use v5.7.5 for broader compat. |
| redis/go-redis/v9 | v9.7.x+ | Redis client | Latest stable. Phase 1 uses Redis minimally (compose health only), but keep for Phase 2 readiness. |
| rs/zerolog | v1.34.x | Structured logging | Already in use, latest stable. |
| oklog/ulid/v2 | v2.1.0 | ULID generation | Already in use, stable. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| gowebpki/jcs | latest | RFC 8785 JCS canonical JSON | Replace naive json.Marshal in CanonicalJSON() |
| golang-migrate/migrate/v4 | v4.x | Database migrations | Run numbered SQL migrations programmatically or via CLI |
| golang.org/x/crypto | latest | Extended crypto support | Already required by other deps |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gowebpki/jcs | cyberphone/json-canonicalization | cyberphone is the JCS spec author's impl, but gowebpki is more idiomatic Go. Both pass RFC test vectors. |
| gowebpki/jcs | encoding/json/jsontext (stdlib) | Only available as GOEXPERIMENT in Go 1.25+, not stable. Do not use. |
| Fiber v2 | Fiber v3 | v3 needs Go 1.25+, major API changes (Ctx is context.Context, Listen config unified, static removed). Not worth it for 4 endpoints. |
| golang-migrate | manual SQL exec | golang-migrate gives CLI + programmatic migration with versioning, rollback, dirty state tracking. |
| stdlib testing | testify | stdlib is sufficient for Phase 1 unit tests. No assertion library needed for table-driven tests. |

**Installation:**
```bash
cd platform
go mod edit -go=1.24
go get github.com/gofiber/fiber/v2@latest
go get github.com/jackc/pgx/v5@v5.7.5
go get github.com/redis/go-redis/v9@latest
go get github.com/rs/zerolog@latest
go get github.com/oklog/ulid/v2@latest
go get github.com/gowebpki/jcs@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/golang-migrate/migrate/v4/database/postgres@latest
go get github.com/golang-migrate/migrate/v4/source/file@latest
go mod tidy
```

## Architecture Patterns

### Recommended Project Structure (Phase 1)
```
platform/
  cmd/server/
    main.go              # Wiring: config -> store -> handler -> routes -> shutdown
  internal/
    api/
      api.go             # Handler struct, SetupRoutes, 4 handlers, auth middleware, problem()
      api_test.go         # HTTP-level tests for all endpoints
    config/
      config.go           # Trimmed to Phase 1 fields only
    crypto/
      crypto.go           # Ed25519, JCS, tokens, IDs
      crypto_test.go       # Unit tests for all crypto functions
    models/
      models.go           # Entity, RegisterRequest/Response (with private_key), ProblemDetail only
    store/
      store.go            # CreateEntity, GetEntity, GetEntityByTokenHash only
      store_test.go        # Unit tests for store (against real Postgres or with interface mock)
  migrations/
    001_entities.up.sql    # Only entities table
    001_entities.down.sql  # Drop entities table
  Dockerfile              # Multi-stage Alpine build
  go.mod
  go.sum
docker-compose.yml        # platform + postgres + redis
```

### Pattern 1: Handler Struct with Dependency Injection
**What:** Single `Handler` struct holds store, redis, config, logger. All HTTP handlers are methods on it.
**When to use:** All endpoint implementations.
**Example:**
```go
type Handler struct {
    store  *store.Store
    config *config.Config
    log    zerolog.Logger
}

func NewHandler(s *store.Store, cfg *config.Config, log zerolog.Logger) *Handler {
    return &Handler{store: s, config: cfg, log: log}
}
```
Note: Redis removed from Handler for Phase 1 since no pub/sub is needed. Re-add in Phase 2.

### Pattern 2: RFC 7807 Problem Details
**What:** All errors return structured JSON with type, title, status, detail, instance.
**When to use:** Every error response.
**Example:**
```go
func problem(c *fiber.Ctx, status int, errType, title, detail string) error {
    return c.Status(status).JSON(models.ProblemDetail{
        Type:     fmt.Sprintf("https://atap.dev/errors/%s", errType),
        Title:    title,
        Status:   status,
        Detail:   detail,
        Instance: c.Path(),
    })
}
```

### Pattern 3: Token Auth via SHA-256 Hash Lookup
**What:** Bearer token hashed with SHA-256, looked up in entities.token_hash column.
**When to use:** Auth middleware for protected endpoints.
**Key detail:** Token is `atap_` + 32 bytes base64url. Hash the full string including prefix.

### Pattern 4: Generate-and-Forget Key Management
**What:** Server generates Ed25519 keypair, returns private key in registration response, never stores it.
**When to use:** Agent registration only.
**Key detail:** Private key encoded as base64 standard encoding, same as public key.

### Anti-Patterns to Avoid
- **Storing private keys on server:** Per CONTEXT.md, server never stores private keys. Generate, return, forget.
- **Returning delivery info in Phase 1:** No inbox_url, stream_url, webhook_url in registration or lookup responses.
- **Using json.Marshal for canonical JSON:** Does NOT comply with RFC 8785. Float serialization differs. Use gowebpki/jcs.
- **Phase 2+ stubs:** No stub endpoints. Clean Phase 1 surface only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON Canonicalization (RFC 8785) | json.Marshal with sorted keys | gowebpki/jcs Transform() | RFC 8785 requires specific float serialization per ECMAScript. json.Marshal does not comply. |
| Database migrations | Raw SQL exec in main.go | golang-migrate/migrate/v4 | Versioning, rollback, dirty state detection, CLI tooling |
| ULID generation | Custom time-based IDs | oklog/ulid/v2 | Monotonic ordering, proper entropy, time extraction |
| Token entropy | math/rand | crypto/rand (already used) | Security-critical random bytes must use crypto/rand |

**Key insight:** The existing `CanonicalJSON()` function is the biggest hidden bug. It uses `json.Marshal` which does NOT produce RFC 8785 compliant output for floating point numbers. This will cause cross-language signature verification failures when non-Go clients verify signatures.

## Common Pitfalls

### Pitfall 1: CanonicalJSON is not RFC 8785 compliant
**What goes wrong:** Go's `json.Marshal` serializes floats differently than ECMAScript. Signatures created with Go cannot be verified by JS/Python clients.
**Why it happens:** `json.Marshal` sorts map keys (good) but does not follow ECMAScript number serialization (bad).
**How to avoid:** Replace `CanonicalJSON` with `gowebpki/jcs.Transform()`. Feed it `json.Marshal` output, get RFC 8785 output.
**Warning signs:** Cross-language signature verification fails silently.

### Pitfall 2: Channel ID entropy too low
**What goes wrong:** Current `NewChannelID()` uses 8 bytes (64 bits) of entropy. Brute-forceable for webhook URLs that accept unauthenticated POST.
**Why it happens:** Original code uses `make([]byte, 8)` instead of `make([]byte, 16)`.
**How to avoid:** Change to 16 bytes, producing `chn_` + 32 hex chars.
**Warning signs:** CRY-04 explicitly requires 128-bit entropy.

### Pitfall 3: Dockerfile has typo
**What goes wrong:** Binary is named `atep-platform` instead of `atap-platform` in the Dockerfile.
**Why it happens:** Typo in the original scaffolding.
**How to avoid:** Fix during Dockerfile rewrite. Also: go.sum is missing (go mod tidy needed).

### Pitfall 4: go.mod has no go.sum and stale dependencies
**What goes wrong:** `go mod download` fails because go.sum does not exist. Dependencies are 12+ months old.
**Why it happens:** Scaffolding was generated without running `go mod tidy`.
**How to avoid:** Run `go mod tidy` as the very first step after updating go.mod versions.

### Pitfall 5: Redis is required in main.go but not needed for Phase 1
**What goes wrong:** Platform fails to start if Redis is unavailable, but Phase 1 has no pub/sub functionality.
**Why it happens:** main.go wires Redis as mandatory.
**How to avoid:** Two options: (a) keep Redis in Docker Compose but make connection optional in main.go, or (b) keep it required for Phase 2 readiness. Recommendation: keep required in Docker Compose (cheap), but remove from Handler struct since no handler uses it in Phase 1.

### Pitfall 6: Entity lookup returns too many fields
**What goes wrong:** Current GetEntity handler returns the full Entity struct including delivery_pref, webhook_url, attestations.
**Why it happens:** Uses `c.JSON(entity)` directly without a response DTO.
**How to avoid:** Create a dedicated `EntityLookupResponse` struct with only the fields specified in CONTEXT.md.

## Code Examples

### RFC 8785 Canonical JSON with gowebpki/jcs
```go
// Source: https://pkg.go.dev/github.com/gowebpki/jcs
import "github.com/gowebpki/jcs"

func CanonicalJSON(v interface{}) ([]byte, error) {
    // First marshal to standard JSON
    raw, err := json.Marshal(v)
    if err != nil {
        return nil, fmt.Errorf("marshal json: %w", err)
    }
    // Then canonicalize per RFC 8785
    canonical, err := jcs.Transform(raw)
    if err != nil {
        return nil, fmt.Errorf("canonicalize json: %w", err)
    }
    return canonical, nil
}
```

### Fixed Channel ID with 128-bit entropy
```go
func NewChannelID() string {
    b := make([]byte, 16) // 128 bits
    if _, err := rand.Read(b); err != nil {
        panic("crypto/rand failed")
    }
    return fmt.Sprintf("chn_%x", b) // chn_ + 32 hex chars
}
```

### Registration Response with Private Key
```go
type RegisterResponse struct {
    URI        string `json:"uri"`
    ID         string `json:"id"`
    Token      string `json:"token"`
    PublicKey  string `json:"public_key"`
    PrivateKey string `json:"private_key"`
    KeyID      string `json:"key_id"`
}
```

### Entity Lookup Response (trimmed)
```go
type EntityLookupResponse struct {
    ID         string    `json:"id"`
    Type       string    `json:"type"`
    URI        string    `json:"uri"`
    PublicKey  string    `json:"public_key"`
    KeyID      string    `json:"key_id"`
    Name       string    `json:"name,omitempty"`
    TrustLevel int       `json:"trust_level"`
    Registry   string    `json:"registry"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### Entities-Only Migration (001_entities.up.sql)
```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE entities (
    id                  TEXT PRIMARY KEY,
    type                TEXT NOT NULL CHECK (type IN ('agent', 'machine', 'human', 'org')),
    uri                 TEXT UNIQUE NOT NULL,
    public_key_ed25519  BYTEA NOT NULL,
    key_id              TEXT NOT NULL,
    name                TEXT,
    trust_level         INTEGER NOT NULL DEFAULT 0 CHECK (trust_level BETWEEN 0 AND 3),
    token_hash          BYTEA NOT NULL,
    registry            TEXT NOT NULL DEFAULT 'atap.app',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_token ON entities(token_hash);
```

### golang-migrate Integration in main.go
```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func runMigrations(databaseURL string) error {
    m, err := migrate.New("file://migrations", databaseURL)
    if err != nil {
        return fmt.Errorf("create migrator: %w", err)
    }
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("run migrations: %w", err)
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Go 1.22 | Go 1.24 (1.26 is latest but 1.24 is stable LTS-equivalent) | Feb 2025 | 1.22 is EOL. 1.24 adds iterator improvements, crypto enhancements |
| Fiber v2 | Fiber v3 available (requires Go 1.25+) | 2025 | Stay on v2 for Phase 1: simpler, no migration risk |
| pgx v5.5.0 | pgx v5.7.5+ | 2024-2025 | Bug fixes, performance improvements |
| json.Marshal for canonical JSON | gowebpki/jcs for RFC 8785 | N/A | Critical fix for cross-language interop |
| Docker Compose v2 format ("3.9") | Docker Compose v2 format (drop version key) | 2023 | version key is deprecated, remove it |

**Deprecated/outdated:**
- `version: "3.9"` in docker-compose.yml: Docker Compose v2 ignores the version key. Remove it.
- Go 1.22: End of life. Must upgrade.
- `go-redis/redis/v9` import path: Moved to `redis/go-redis/v9`. Already correct in scaffolding.

## Open Questions

1. **Should Redis be optional in Phase 1 main.go?**
   - What we know: Phase 1 has no Redis-dependent features (no pub/sub, no SSE)
   - What's unclear: Whether making it optional adds complexity vs just requiring it in Docker Compose
   - Recommendation: Keep Redis in Docker Compose (required for health check), keep connection in main.go for simplicity, but remove from Handler struct. The overhead is negligible and avoids re-wiring in Phase 2.

2. **golang-migrate: embed migrations or use file source?**
   - What we know: Go 1.16+ supports embed. golang-migrate supports both file:// and embed sources.
   - What's unclear: Whether embedded migrations are better for Docker deployment
   - Recommendation: Use `file://migrations` for local dev and Docker (volume-mounted). Embed is nice but adds complexity. File source matches existing Docker Compose pattern of mounting migrations into initdb.d.

3. **Entity table: keep Phase 2 columns or trim?**
   - What we know: CONTEXT.md says 001_init.sql creates only the entities table. Existing migration has many Phase 2+ columns.
   - What's unclear: Whether to keep extra columns for forward compatibility or trim strictly
   - Recommendation: Trim strictly. Phase 2 adds its own migrations. Keeping unused columns is confusing and violates "clean slate" principle.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing (go test) |
| Config file | None needed (Go convention) |
| Quick run command | `cd platform && go test ./internal/crypto/ -v -count=1` |
| Full suite command | `cd platform && go test ./... -v -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CRY-01 | Ed25519 keypair generation | unit | `cd platform && go test ./internal/crypto/ -run TestGenerateKeyPair -v` | No -- Wave 0 |
| CRY-02 | RFC 8785 JCS canonical JSON | unit | `cd platform && go test ./internal/crypto/ -run TestCanonicalJSON -v` | No -- Wave 0 |
| CRY-03 | Signable payload format | unit | `cd platform && go test ./internal/crypto/ -run TestSignablePayload -v` | No -- Wave 0 |
| CRY-04 | Channel ID 128-bit entropy | unit | `cd platform && go test ./internal/crypto/ -run TestNewChannelID -v` | No -- Wave 0 |
| TST-03 | Crypto unit tests (keypair, signing, verification) | unit | `cd platform && go test ./internal/crypto/ -v` | No -- Wave 0 |
| TST-04 | Token generation and hash verification | unit | `cd platform && go test ./internal/crypto/ -run TestToken -v` | No -- Wave 0 |
| REG-01 | POST /v1/register returns entity in <1s | HTTP test | `cd platform && go test ./internal/api/ -run TestRegister -v` | No -- Wave 0 |
| REG-02 | Registration generates Ed25519 + ULID | unit (covered by CRY-01 + REG-01) | See CRY-01 and REG-01 | No |
| REG-03 | Token format atap_ + 32 bytes base64url | unit | `cd platform && go test ./internal/crypto/ -run TestNewToken -v` | No -- Wave 0 |
| REG-04 | GET /v1/entities/{id} public lookup | HTTP test | `cd platform && go test ./internal/api/ -run TestGetEntity -v` | No -- Wave 0 |
| REG-05 | URI scheme agent://{ulid} | unit (covered by REG-01 test) | See REG-01 | No |
| AUTH-01 | Protected endpoints require Bearer token | HTTP test | `cd platform && go test ./internal/api/ -run TestAuthRequired -v` | No -- Wave 0 |
| AUTH-02 | Token validated by SHA-256 hash lookup | HTTP test (covered by AUTH-01) | See AUTH-01 | No |
| ERR-01 | RFC 7807 error responses | HTTP test | `cd platform && go test ./internal/api/ -run TestErrorFormat -v` | No -- Wave 0 |
| ERR-02 | Health endpoint response format | HTTP test | `cd platform && go test ./internal/api/ -run TestHealth -v` | No -- Wave 0 |
| INF-01 | docker compose up starts full stack | manual/smoke | `docker compose up -d && curl localhost:8080/v1/health` | No -- manual |
| INF-02 | Multi-stage Alpine Dockerfile | manual/smoke | `docker build -t atap-platform ./platform` | No -- manual |
| INF-03 | golang-migrate migrations | integration (covered by store tests) | See store tests | No |
| INF-04 | zerolog structured logging | manual inspection | N/A | No -- manual-only |
| INF-05 | Graceful shutdown | manual | N/A | No -- manual-only |
| INF-06 | Dependencies updated | build verification | `cd platform && go build ./...` | No -- build step |

### Sampling Rate
- **Per task commit:** `cd platform && go test ./... -v -count=1`
- **Per wave merge:** `cd platform && go test ./... -v -count=1 -race`
- **Phase gate:** Full suite green + `docker compose up -d && curl localhost:8080/v1/health`

### Wave 0 Gaps
- [ ] `platform/internal/crypto/crypto_test.go` -- covers CRY-01, CRY-02, CRY-03, CRY-04, TST-03, TST-04
- [ ] `platform/internal/api/api_test.go` -- covers REG-01, REG-04, AUTH-01, ERR-01, ERR-02
- [ ] go.sum file -- `go mod tidy` needed before any tests can run
- [ ] golang-migrate dependency -- `go get github.com/golang-migrate/migrate/v4`
- [ ] gowebpki/jcs dependency -- `go get github.com/gowebpki/jcs`

## Sources

### Primary (HIGH confidence)
- [Go Release History](https://go.dev/doc/devel/release) -- Go 1.24.13 current, 1.22 EOL
- [Go End of Life](https://endoflife.date/go) -- version support policy
- [pgx v5 GitHub](https://github.com/jackc/pgx) -- v5.7.5/v5.8.0 latest
- [Fiber v2 pkg.go.dev](https://pkg.go.dev/github.com/gofiber/fiber/v2) -- v2.52.x latest v2
- [gowebpki/jcs](https://github.com/gowebpki/jcs) -- RFC 8785 Go implementation
- [golang-migrate](https://github.com/golang-migrate/migrate) -- v4 latest, Nov 2025

### Secondary (MEDIUM confidence)
- [Fiber v3 what's new](https://docs.gofiber.io/next/whats_new/) -- v3 breaking changes, Go 1.25+ requirement
- [go-redis/v9 releases](https://github.com/redis/go-redis/releases) -- latest Feb 2026
- [encoding/json/jsontext](https://pkg.go.dev/encoding/json/jsontext) -- experimental JCS in stdlib (Go 1.25+)

### Tertiary (LOW confidence)
- [json-canon](https://dev.to/lenny321/json-canon-a-strict-rfc-8785-implementation-in-go-for-deterministic-json-3mfg) -- alternative JCS lib (Feb 2026), not widely adopted yet

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified via pkg.go.dev and GitHub releases
- Architecture: HIGH -- existing codebase provides clear patterns, changes are trimming not inventing
- Pitfalls: HIGH -- identified from direct code review of existing scaffolding
- JCS library choice: MEDIUM -- gowebpki/jcs is well-established but not personally verified against RFC test vectors

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable domain, slow-moving dependencies)
