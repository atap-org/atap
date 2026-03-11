# Technology Stack

**Project:** ATAP -- Agent Trust and Authority Protocol
**Researched:** 2026-03-11

## Stack Validation Summary

The build guide specifies Go/Fiber, PostgreSQL, Redis, Flutter. These are solid choices for a cryptographic identity platform with real-time delivery. The main issue: the go.mod pins Go 1.22 and Fiber v2.52.0 -- both are behind current stable releases. The go.mod also lacks several critical libraries (migrations, testing, config). This document prescribes the exact versions to use.

---

## Recommended Stack

### Language Runtime

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.24+ (target 1.24.x) | Platform language | Current stable branch supported by Go team. Go 1.26 is latest but 1.24 is safer minimum to set in go.mod -- compatible with all dependencies below. Avoids forcing bleeding-edge toolchain. | HIGH |

**Note on Go version:** The go.mod currently says `go 1.22`. Bump to `go 1.24` minimum. Go 1.22 is past end-of-life (only the two most recent major versions are supported). Go 1.24 or 1.25 are the actively supported branches as of March 2026. Setting `go 1.24` keeps compatibility broad while being within the supported window.

### Core Framework

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Fiber | v2.52.6+ (stay on v2) | HTTP framework | Fiber v3 is released but requires Go 1.25 minimum and has a different API surface. The build guide was designed around Fiber v2 semantics. Migrating to v3 adds unnecessary risk for Phase 1 -- the SSE and middleware patterns the project needs work fine in v2. Upgrade to v3 after Phase 1 ships. | HIGH |

**Why NOT Fiber v3 now:** Fiber v3 requires Go 1.25+, introduces breaking API changes (context handling, middleware signatures), and the ecosystem of v3-compatible contrib middleware is still maturing. The project's SSE delivery, auth middleware, and route structure all map cleanly to v2. Migrate later when the platform is stable.

**Why NOT chi/echo/stdlib:** Fiber's Express-like API matches the build guide's patterns exactly. Its built-in SSE support via `c.Context().SetBodyStreamWriter()` is well-documented. chi is excellent but would require rewriting all handler patterns from the build guide.

### Database

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| PostgreSQL | 16 | Primary data store | JSONB for signal payloads, pgcrypto for server-side hashing, proven ACID guarantees for delegation chains. PG 16 is specified in build guide and Docker Compose. PG 17 exists but 16 is stable and well-tested. | HIGH |
| pgx | v5.7.5+ | PostgreSQL driver | Native Go PostgreSQL driver. Faster than database/sql for native queries, supports LISTEN/NOTIFY, connection pooling via pgxpool. The go.mod pins v5.5.0 -- bump to v5.7.5+ for bug fixes and PG 16 improvements. v5.8.0 requires Go 1.24+. | HIGH |

**Why NOT database/sql + lib/pq:** pgx is the standard Go PostgreSQL driver in 2025/2026. lib/pq is in maintenance mode. pgx provides native protocol support, better performance, and pgxpool for connection management.

### Caching / Pub-Sub

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Redis | 7 | SSE fan-out pub/sub | Redis pub/sub for broadcasting signals to SSE connections across multiple platform instances. Also useful for rate limiting and token caching. Redis 7 as specified. | HIGH |
| go-redis | v9.7.0+ | Redis client | Official Redis Go client. v9 supports RESP3 protocol, OpenTelemetry hooks. The go.mod pins v9.4.0 -- bump to latest v9.7.x for stability fixes. | HIGH |

### Cryptography

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| golang.org/x/crypto | v0.36.0+ | Ed25519, X25519, NaCl | Standard Go extended crypto library. Ed25519 signing/verification is in stdlib `crypto/ed25519` but x/crypto provides the nacl/box package needed for X25519 encryption. Bump from v0.28.0 to latest. | HIGH |
| crypto/ed25519 (stdlib) | n/a | Ed25519 sign/verify | Use stdlib `crypto/ed25519` for key generation and signing -- it's the canonical implementation. Only pull in x/crypto for X25519/NaCl features. | HIGH |
| crypto/sha256 (stdlib) | n/a | Token hashing, human ID derivation | SHA-256 for bearer token storage and human ID generation from public keys. Pure stdlib, no external dependency. | HIGH |

**Why NOT libsodium/go-nacl bindings:** Pure Go crypto avoids CGO, simplifies cross-compilation, and the stdlib Ed25519 implementation is audited and performant. CGO bindings add build complexity for no meaningful security benefit at this scale.

### ID Generation

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| oklog/ulid | v2.1.1 | ULID generation | Time-sortable, lexicographically ordered, URL-safe identifiers. The go.mod pins v2.1.0 -- bump to v2.1.1 (latest, Nov 2025). Prefixed IDs (sig_, chn_, atap_) built as string concatenation on top. | HIGH |

**Why NOT UUID v7:** ULIDv2 is already in go.mod and matches the spec. UUID v7 provides similar time-sortability but the ULID encoding (Crockford base32) is more compact and URL-friendly. The spec explicitly calls for ULIDs.

### Database Migrations

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| golang-migrate | v4.19.1 | SQL schema migrations | The standard Go migration library. Supports numbered SQL files (which the build guide uses: 001_entities.sql, etc.), PostgreSQL driver, both CLI and library usage. Missing from go.mod -- must add. | HIGH |

**Why NOT goose:** golang-migrate has broader adoption (3,300+ importers for PG driver alone), supports the numbered SQL file convention already established in the migrations/ directory, and doesn't require Go code in migration files.

**Why NOT atlas:** Atlas is excellent for declarative migrations but overkill for Phase 1. The project needs simple sequential SQL migrations, not schema diffing.

### Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| kelseyhightower/envconfig | v1.4.0 | Environment config | Build guide specifies envconfig. Simple, proven, zero-dependency library for parsing env vars into structs. Mature and stable (v1.4.0 is latest). Missing from go.mod -- must add. | MEDIUM |

**Why NOT viper:** Viper is massively over-featured for this use case. ATAP needs env vars for Docker/Coolify deployment. envconfig does exactly that with no file-watching, remote-config, or YAML overhead.

**Alternative consideration:** For new Go projects in 2026, `caarlos0/env` (v11+) is gaining popularity as a more modern envconfig with generics support. However, envconfig is what the build guide specifies and it works. Stick with it.

### Logging

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| rs/zerolog | v1.34.0 | Structured JSON logging | Fast, zero-allocation structured logger. Already in go.mod at v1.32.0 -- bump to v1.34.0 (latest, March 2025). Outputs JSON logs compatible with Grafana/Loki stack specified in infrastructure. | MEDIUM |

**Why NOT log/slog (stdlib):** Go's stdlib slog (since 1.21) is now the default recommendation for new projects. However, zerolog is already specified in the build guide, scaffolding likely references it, and it's faster for the high-throughput signal delivery path. Keep zerolog. If starting fresh with no existing guide, slog would be the choice.

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| testing (stdlib) | n/a | Unit test runner | Go standard testing. No external framework needed for test execution. | HIGH |
| stretchr/testify | v1.10.0+ | Assertions and test suites | Reduces assertion boilerplate significantly. `assert` for non-fatal, `require` for fatal assertions. Standard in Go ecosystem. Missing from go.mod -- add as test dependency. | HIGH |
| testcontainers-go | v0.35.0+ | Integration test containers | Spin up real PostgreSQL and Redis containers in tests. Eliminates mocking the database layer for integration tests. Use the postgres and redis modules. Missing from go.mod -- add as test dependency. | HIGH |

**Why NOT gomock/mockgen:** For a platform this size, test against real databases with testcontainers rather than mocking the store layer. Mocks hide integration bugs. Use interfaces for the store layer (good practice) but test against real PostgreSQL.

**Why NOT ginkgo/gomega:** Testify + stdlib testing is simpler and more idiomatic Go. Ginkgo's BDD style adds cognitive overhead without proportional benefit for this project type.

### Supporting Libraries (Missing from go.mod)

| Library | Version | Purpose | Why | Confidence |
|---------|---------|---------|-----|------------|
| golang-migrate/migrate | v4.19.1 | DB migrations | See migrations section above | HIGH |
| kelseyhightower/envconfig | v1.4.0 | Env config | See config section above | MEDIUM |
| stretchr/testify | v1.10.0+ | Test assertions | See testing section above | HIGH |
| testcontainers/testcontainers-go | v0.35.0+ | Integration testing | See testing section above | HIGH |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| HTTP Framework | Fiber v2 | Fiber v3 | Requires Go 1.25+, breaking API changes, ecosystem still stabilizing |
| HTTP Framework | Fiber v2 | chi | Would require rewriting all handler patterns from build guide |
| HTTP Framework | Fiber v2 | stdlib net/http | Lacks built-in middleware ecosystem, more boilerplate for SSE |
| DB Driver | pgx v5 | database/sql + lib/pq | lib/pq is maintenance mode, pgx is faster and more featured |
| Migrations | golang-migrate | goose | golang-migrate matches existing numbered SQL convention |
| Migrations | golang-migrate | atlas | Overkill declarative approach for sequential SQL migrations |
| Logging | zerolog | log/slog | zerolog already in build guide, faster for hot path |
| Config | envconfig | viper | Viper is overengineered for env-var-only configuration |
| Testing | testcontainers | docker-compose test setup | testcontainers is self-contained in Go test files, no external setup |
| IDs | ULID | UUID v7 | ULID specified in protocol spec, more compact encoding |
| Crypto | stdlib ed25519 | libsodium CGO | Pure Go avoids CGO, audited stdlib implementation |

---

## Version Bump Summary (go.mod updates needed)

Current go.mod is stale. Here are the required changes:

| Dependency | Current | Target | Change Reason |
|------------|---------|--------|---------------|
| go directive | 1.22 | 1.24 | Go 1.22 is EOL, 1.24 is minimum supported |
| gofiber/fiber/v2 | v2.52.0 | v2.52.6+ | Latest v2 patch, bug fixes |
| jackc/pgx/v5 | v5.5.0 | v5.7.5 | Bug fixes, performance (v5.8.0 if using Go 1.24+) |
| redis/go-redis/v9 | v9.4.0 | v9.7.0+ | Stability fixes, RESP3 improvements |
| rs/zerolog | v1.32.0 | v1.34.0 | Latest stable |
| golang.org/x/crypto | v0.28.0 | v0.36.0+ | Security patches, latest stable |
| oklog/ulid/v2 | v2.1.0 | v2.1.1 | Latest patch |
| **NEW** golang-migrate/migrate/v4 | -- | v4.19.1 | Database migrations (required) |
| **NEW** kelseyhightower/envconfig | -- | v1.4.0 | Environment configuration (required) |
| **NEW** stretchr/testify | -- | v1.10.0+ | Test assertions (dev dependency) |
| **NEW** testcontainers/testcontainers-go | -- | v0.35.0+ | Integration testing (dev dependency) |

---

## Installation

```bash
cd platform

# Update go directive
go mod edit -go=1.24

# Core dependencies (update existing)
go get github.com/gofiber/fiber/v2@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/redis/go-redis/v9@latest
go get github.com/rs/zerolog@latest
go get golang.org/x/crypto@latest
go get github.com/oklog/ulid/v2@latest

# New required dependencies
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
go get github.com/kelseyhightower/envconfig

# Test dependencies
go get github.com/stretchr/testify
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/testcontainers/testcontainers-go/modules/redis

# Tidy
go mod tidy
```

---

## Mobile Stack (Flutter -- validation only)

The build guide specifies Flutter 3.x with the following packages. These are validated but not the focus of Phase 1 platform work.

| Package | Purpose | Status | Confidence |
|---------|---------|--------|------------|
| riverpod | State management | Standard choice, actively maintained | HIGH |
| dio | HTTP client | Standard choice for Flutter | HIGH |
| flutter_secure_storage | Keychain/Keystore access | Required for Ed25519 private key storage | HIGH |
| cryptography_flutter | Ed25519, X25519 | Need to verify Ed25519 support depth -- may need pointycastle as fallback | MEDIUM |
| firebase_messaging | Push notifications | Standard for FCM/APNs | HIGH |
| go_router | Deep linking | Standard for Flutter navigation | HIGH |

**Flag:** The `cryptography_flutter` package should be validated during the mobile phase. Ed25519 support in Dart/Flutter is less mature than in Go. `pointycastle` is a potential fallback but has worse performance.

---

## Infrastructure Stack (validation)

| Component | Technology | Status | Confidence |
|-----------|-----------|--------|------------|
| Hosting | Hetzner Cloud | Specified, shared with SIMRelay | HIGH |
| Deployment | Coolify (Docker-based) | Specified, compatible with Dockerfile approach | HIGH |
| Reverse proxy | Caddy | Automatic HTTPS, good SSE support | HIGH |
| Monitoring | Grafana + Loki + Prometheus | Standard observability stack, zerolog JSON output feeds Loki | HIGH |
| CI/CD | GitHub Actions | Standard for open-source Go projects | HIGH |

**Note on SSE and Caddy:** Caddy handles SSE connections well by default but verify `flush_interval` is set appropriately for the reverse proxy config. Without it, Caddy may buffer SSE events.

---

## Sources

- [Fiber GitHub releases](https://github.com/gofiber/fiber/releases) -- v2/v3 version status
- [Fiber v3 what's new](https://docs.gofiber.io/next/whats_new/) -- Go 1.25 requirement confirmed
- [pgx GitHub](https://github.com/jackc/pgx) -- v5.7.5, v5.8.0 available
- [golang-migrate GitHub](https://github.com/golang-migrate/migrate) -- v4.19.1, Nov 2025
- [go-redis GitHub](https://github.com/redis/go-redis) -- v9 latest
- [oklog/ulid releases](https://github.com/oklog/ulid/releases) -- v2.1.1, Nov 2025
- [Go release history](https://go.dev/doc/devel/release) -- Go 1.24/1.25/1.26 status
- [zerolog Go packages](https://pkg.go.dev/github.com/rs/zerolog) -- v1.34.0
- [testcontainers-go](https://golang.testcontainers.org/) -- PostgreSQL and Redis modules
- [testify GitHub](https://github.com/stretchr/testify) -- standard Go test assertions
