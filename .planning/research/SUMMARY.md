# Project Research Summary

**Project:** ATAP — Agent Trust and Authority Protocol
**Domain:** Cryptographic agent identity platform with real-time signal delivery
**Researched:** 2026-03-11
**Confidence:** HIGH

## Executive Summary

ATAP is a hub-and-spoke signal broker that provides cryptographic identity and inbox-as-a-service for AI agents. The core pattern is well-established: persist signals durably to PostgreSQL first, then fan out via Redis pub/sub for real-time SSE delivery, with webhook and mobile push as secondary delivery channels. The Go/Fiber/PostgreSQL/Redis stack is the right choice. The existing codebase already implements Levels 0-4 of the component dependency graph (config through SSE streamer and channel manager); Phase 1 build work is primarily about completing the delivery pipeline, hardening the system, and building the Flutter mobile foundation.

The most important design decision is positioning ATAP below MCP, A2A, and AP2 as the open, vendor-neutral identity and trust layer that those protocols assume exists but do not define. The "doorbell" (registration + inbox + signal delivery) is the wedge product; the cryptographic trust chain (delegations, claims, attestations) is the moat that no competing open protocol currently occupies. Microsoft Entra Agent ID and Google A2A are the closest competitors but are either proprietary/Azure-locked or do not define persistent identity. AgentMail ($6M seed, March 2026) validates the "inbox for agents" market segment. ATAP's open protocol and cryptographic identity are genuine differentiators.

The critical risks are all in Phase 1: Redis pub/sub message loss during SSE reconnect gaps requires a careful write-then-fan-out pattern with replay; cross-language signature verification will fail without RFC 8785 JCS canonicalization; private key handling in the registration response is a fundamental trust model contradiction that must be resolved before the API ships; and SSE proxy buffering will silently break real-time delivery in production without explicit Caddy configuration. All of these are solvable with known patterns — none are architectural dead ends — but they must be addressed before the first public deployment.

## Key Findings

### Recommended Stack

The stack specified in the build guide is correct. The primary issue is that go.mod is stale: Go directive is 1.22 (EOL), and several libraries are pinned below their current stable versions. Four dependencies are entirely missing from go.mod: `golang-migrate/migrate` (required for database migrations), `kelseyhightower/envconfig` (environment configuration), `stretchr/testify` (test assertions), and `testcontainers-go` (integration testing against real PostgreSQL and Redis). These must be added before serious development begins.

Stay on Fiber v2 (not v3) for Phase 1. Fiber v3 requires Go 1.25+, has breaking API changes, and the contrib middleware ecosystem is still stabilizing. The existing SSE, auth middleware, and route patterns all map cleanly to v2. Upgrade after Phase 1 ships. For the mobile Flutter stack, the `cryptography_flutter` package's Ed25519 cross-platform compatibility with Go's stdlib implementation must be validated before the first entity registration — this is a known gap that requires cross-language test vectors.

**Core technologies:**
- Go 1.24+ / Fiber v2.52.6+: HTTP platform — stay on v2, bump Go to supported version (1.22 is EOL)
- PostgreSQL 16 / pgx v5: Primary store — ACID guarantees for signal durability, JSONB for payloads
- Redis 7 / go-redis v9: Pub/sub fan-out — real-time SSE delivery (notification only, not durable)
- Ed25519 (stdlib crypto/ed25519): Keypair generation and signing — pure Go, no CGO
- golang-migrate v4: Database migrations — numbered SQL file convention, missing from go.mod
- testcontainers-go: Integration testing — test against real PostgreSQL and Redis, not mocks
- Flutter 3.x / riverpod / flutter_secure_storage: Mobile client — human entity support, push notifications

### Expected Features

**Must have (table stakes):**
- Agent registration with Ed25519 keypair and bearer token — the entry point; everything depends on it
- Signal sending to durable inbox — the core value proposition ("give your agent a doorbell")
- SSE streaming delivery with Last-Event-ID replay — real-time push; makes the platform feel alive
- Inbound channels (per-service webhook URLs) — external services can push signals into agent inboxes
- Webhook push delivery with Ed25519 signature — agents in serverless environments get signals pushed out
- Polling fallback with cursor-based pagination — safety net for batch/cron consumers
- Entity lookup (public key resolution) — without this, no signature verification is possible
- RFC 7807 error responses — baseline for any developer-facing API
- Health endpoint and idempotency support — operational basics and retry safety

**Should have (Phase 1 differentiators):**
- Flutter mobile app foundation — registration, inbox view, push notifications; foundation for human-in-the-loop flows
- Entity URI scheme (agent://, human://, machine://, org://) — universal addressing for the agent economy
- Signal threading and references (thread/ref fields) — enables conversational patterns and approval flows
- Canonical JSON signing (RFC 8785 JCS) — enables offline verification by any party with the public key
- Progressive trust levels (0-3) — Phase 1 is Level 0 only, but the model must be designed in from day one

**Defer to Phase 2+:**
- Delegation documents and chain verification — requires trust infrastructure that doesn't exist yet
- Human entity registration with attestations — adds verification flows; Phase 2
- Claim flow (agent-initiated trust elevation) — requires humans and delegations; Phase 2
- End-to-end encryption (X25519) — Phase 3; reserve the `trust.enc` field for future use
- Client SDKs (Python, JS, Go) — premature before API is frozen; provide OpenAPI spec and curl examples first
- Organizations, federation, branded approval templates — Phase 3-4

### Architecture Approach

ATAP follows a hub-and-spoke signal broker pattern. The component dependency graph (Config → Store → Auth Middleware → Entity Registry + Signal Router → SSE Streamer + Channel Manager → Delivery Manager) is the correct build order. The existing codebase implements through Level 4; the remaining Phase 1 work is the Delivery Manager, Webhook Pusher, and Push Manager at Level 5. The single monolithic `api.go` file (currently 540 lines) must be split into per-domain files before Phase 2 adds claims, delegations, and attestations — it will otherwise balloon past 2000 lines.

**Major components:**
1. Entity Registry — registration, lookup, ULID-based ID generation, keypair management
2. Signal Router — accept, persist (PostgreSQL), fan-out (Redis), trigger secondary delivery
3. SSE Streamer — long-lived connections, Redis subscription, Last-Event-ID replay, 30s heartbeats
4. Channel Manager — per-service inbound webhook URLs, wraps external payloads as ATAP signals
5. Delivery Manager — orchestrates SSE (always via Redis), webhook push (conditional), mobile push (conditional)
6. Webhook Pusher — outbound delivery with Ed25519 signature and exponential backoff retry
7. Push Manager — FCM/APNs delivery for mobile app entities (not yet implemented)

### Critical Pitfalls

1. **Redis pub/sub message loss during SSE reconnect** — Write to PostgreSQL first (durability), then publish signal ID to Redis (notification only). On reconnect, replay from PostgreSQL with a short overlap window after subscribing to Redis to close the race. Never rely on Redis alone for delivery.

2. **Cross-language canonical JSON signature failure** — The current `CanonicalJSON` in `crypto.go` uses Go's `json.Marshal` which is not RFC 8785 compliant. Add a proper JCS implementation before any SDK ships. Build cross-language signing tests (sign in Go, verify in Python/JS) in CI from day one.

3. **Private key returned in registration response** — The build guide has the server generate keypairs and return private keys in HTTP responses. This is a fundamental trust model violation for a protocol whose value proposition is cryptographic identity. Change to client-generated keypairs (client sends only the public key at registration). The server should never hold private keys.

4. **SSE proxy buffering silently breaks real-time delivery** — Configure Caddy with `flush_interval -1` for the SSE route. Set `X-Accel-Buffering: no` header. Send 30s heartbeat comments to keep connections alive through idle-timeout-happy proxies.

5. **Channel webhook URL enumeration (insufficient entropy)** — `NewChannelID` currently generates 8 random bytes (64-bit entropy). Increase to 16 bytes (128-bit) before shipping. Add rate limiting on the inbound webhook endpoint.

## Implications for Roadmap

Based on research, the build order is constrained by the component dependency graph. Levels 0-4 are already implemented; the remaining Phase 1 work is Level 5 delivery pipeline plus structural cleanup, hardening, and mobile. Suggesting 4 phases total.

### Phase 1: Foundation Hardening and Delivery Pipeline Completion

**Rationale:** The core entity/signal infrastructure exists but the delivery pipeline is incomplete (no Webhook Pusher, no Push Manager, no Delivery Manager) and several critical security/correctness issues must be fixed before any public deployment. This must come first because everything else depends on a trustworthy foundation.

**Delivers:** A production-ready signal delivery platform with all three delivery channels (SSE, webhook push, mobile push), hardened authentication, and correct cryptographic primitives.

**Addresses (from FEATURES.md):** All table-stakes features: registration, inbox, SSE, polling, channels, webhook delivery, entity lookup, health, idempotency.

**Avoids (from PITFALLS.md):**
- Fix private key handling in registration (Pitfall 3) — client-generated keys
- Fix canonical JSON (Pitfall 2) — RFC 8785 JCS before SDKs exist
- Fix channel entropy (Pitfall 9) — 128-bit channel IDs
- Fix base64 encoding inconsistency (Pitfall 15) — standardize on base64url
- Implement TTL expiration job (Pitfall 8) — prevents unbounded inbox growth
- Configure Caddy SSE headers (Pitfall 4) — real-time delivery through proxy

**Research flag:** No deep research needed. Architecture is well-documented in the existing codebase and build guide. Patterns (write-then-fan-out, token-hash auth, cursor pagination) are established.

### Phase 2: Flutter Mobile Foundation

**Rationale:** The Flutter app is the human-facing client and the foundation for Phase 3 human entity features. It can be built in parallel with Phase 1 delivery work since the API surface is already defined. However, mobile crypto compatibility (Ed25519 between Dart and Go) must be validated before the first entity registration from the app.

**Delivers:** Flutter app with agent registration, inbox view, signal list/detail, push notification receipt, and secure enclave key storage. Human entity registration on the backend (entity model already supports it; just needs the registration endpoint and push token handling).

**Addresses (from FEATURES.md):** Flutter mobile app foundation, push notifications for signal delivery (FCM/APNs).

**Avoids (from PITFALLS.md):**
- Flutter/Go Ed25519 compatibility (Pitfall 10) — cross-platform test vectors in CI
- Mobile keys in secure enclave (Pitfall 3 consequence) — never transmit private key

**Research flag:** Needs research on `cryptography_flutter` Ed25519 support depth and potential fallback to `pointycastle`. The gap identified in STACK.md must be validated before mobile entity registration is built.

### Phase 3: Trust Infrastructure (Delegations, Claims, Attestations)

**Rationale:** This is the moat — the feature set that no competing open protocol has. It can only be built after the basic signal delivery is proven and the mobile app exists (humans must be able to approve delegation requests from their phones). The component dependency chain requires human entities before claims, and claims before trust elevation.

**Delivers:** Delegation documents with canonical JSON signing, agent-initiated claim flow, human approval via mobile app, Trust Level 1-2 elevation (verified email, World ID attestation), attestation storage.

**Addresses (from FEATURES.md):** Human-derived identity, progressive trust levels, canonical JSON signature scheme as a functional protocol feature (not just infrastructure).

**Avoids (from PITFALLS.md):**
- Delegation verification requires RFC 8785 JCS to be already correct (Pitfall 2 — Phase 1 prerequisite)
- Claim flow state machine — time-limited transitions; needs careful implementation

**Research flag:** Needs research during planning. The claim flow approval UX on mobile, World ID IDKit v4 integration, and delegation chain verification logic are all new territory without existing codebase implementations.

### Phase 4: Ecosystem Expansion (SDKs, Organization Entities, Federation)

**Rationale:** Only after the API is stable and trust infrastructure is proven. SDKs before API stability cause breaking changes in published libraries. Federation is the "become a standard" play and requires adoption to be meaningful.

**Delivers:** Python, JavaScript, and Go SDKs; Organization entities with domain verification; DNS-based federation and `.well-known/atap.json` key discovery; client-side rate tier enforcement.

**Addresses (from FEATURES.md):** Open protocol ecosystem adoption, org:// URI scheme, federation across registries.

**Research flag:** SDK design needs research into comparable protocol SDKs (Matrix client-server SDK patterns, Stripe SDK design). Federation needs DNS TXT record design and `.well-known` spec work. Both require dedicated research-phase during planning.

### Phase Ordering Rationale

- Phase 1 before everything: the signal delivery core has known gaps (no Delivery Manager, no Webhook Pusher, security issues) that make the existing code unsuitable for production. Nothing else can be built on a broken foundation.
- Phase 2 (mobile) can overlap with Phase 1: the API surface is defined, so Flutter work can start as soon as Phase 1 fixes crypto/auth issues. But push notification infrastructure depends on Phase 1's Push Manager.
- Phase 3 (trust) requires both Phase 1 (correct crypto, durable delivery) and Phase 2 (humans with phones to approve delegations). Cannot be parallelized.
- Phase 4 (ecosystem) requires Phase 1 API stability for SDKs, Phase 3 trust infrastructure for organization entities, and sufficient adoption for federation to be meaningful.

### Research Flags

Phases needing deeper research during planning:
- **Phase 2 (Mobile):** Flutter Ed25519 library validation — `cryptography_flutter` vs `pointycastle` compatibility with Go stdlib. Sparse documentation; needs hands-on testing.
- **Phase 3 (Trust):** Claim flow approval UX, World ID IDKit v4 mobile integration, delegation chain verification algorithm. New implementation territory, no existing codebase reference.
- **Phase 4 (Ecosystem):** SDK design patterns for comparable identity protocols, DNS federation spec design. Broad scope; needs scoping research before planning.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Foundation):** Architecture is fully documented in the existing codebase and build guide. All patterns (write-then-fan-out, token-hash auth, SSE fan-out) are well-established in the industry. Execute from existing documentation.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Existing codebase gives direct evidence of what works. Version recommendations based on official release pages and Go support policy. |
| Features | HIGH | Competitive landscape is well-documented (Entra Agent ID, A2A, AgentMail all have public docs). Table stakes derived from clear analogs (email, SQS, Matrix). |
| Architecture | HIGH | Analysis based on existing codebase + build guide + well-established SSE/webhook patterns. Component boundaries are clear and already partially implemented. |
| Pitfalls | HIGH | Most pitfalls derived from existing code inspection (canonical JSON, private key handling, channel entropy). SSE proxy issues are well-documented in the industry. |

**Overall confidence:** HIGH

### Gaps to Address

- **Flutter Ed25519 compatibility:** The STACK.md research flags `cryptography_flutter` as MEDIUM confidence. Must run cross-platform signing tests (Go keypair, Dart verification) before building mobile entity registration. If `cryptography_flutter` fails, fall back to `pointycastle` with a documented performance penalty.

- **go.mod dependency updates:** The current go.mod is stale and missing 4 required dependencies. First task of Phase 1 is running the version bump commands from STACK.md and verifying `go mod tidy` succeeds before touching any feature code.

- **RFC 8785 JCS library selection:** PITFALLS.md identifies the need but does not prescribe a specific Go JCS library. The suggested `github.com/AmbitionEng/go-jcs` needs validation (license, maintenance status, test coverage). This should be selected and integrated in Phase 1 before any signing code is considered stable.

- **Caddy SSE configuration:** The exact Caddy config (`flush_interval -1` on the SSE route) needs to be validated in the actual deployment environment (Coolify on Hetzner). Local development with direct Go server will not surface proxy buffering issues.

- **Client-side key generation migration:** If the build guide's registration flow (server generates keypair, returns private key) is already partially implemented, changing to client-generated keys is a breaking change to the registration API. Must be decided and implemented before any SDK or external integration is built on top of the current flow.

## Sources

### Primary (HIGH confidence)
- `ATAP-BUILD-GUIDE.md` (existing codebase) — architecture decisions, component boundaries, build order
- `platform/internal/` Go source files — existing implementation state, identified gaps
- Go release history (go.dev/doc/devel/release) — Go 1.24/1.25/1.26 support status
- Fiber GitHub releases — v2/v3 version status and Go version requirements
- pgx GitHub — v5.7.5/v5.8.0 availability and requirements
- Redis Pub/Sub documentation — at-most-once delivery semantics confirmation
- RFC 8785 (IETF) — JSON Canonicalization Scheme specification
- W3C Server-Sent Events spec — Last-Event-ID reconnection standard
- RFC 7807 — Problem Details for HTTP APIs

### Secondary (MEDIUM confidence)
- Microsoft Entra Agent ID docs — competitive landscape
- A2A Protocol v0.3 spec — competitive positioning
- AgentMail $6M raise (TechCrunch, March 2026) — market validation
- Indicio ProvenAI — alternative DID-based approach
- SSE production readiness analysis — proxy buffering confirmation

### Tertiary (LOW confidence)
- `github.com/AmbitionEng/go-jcs` — specific Go JCS library; needs independent validation
- `cryptography_flutter` Ed25519 support depth — MEDIUM in STACK.md; needs hands-on test

---
*Research completed: 2026-03-11*
*Ready for roadmap: yes*
