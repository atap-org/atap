# Domain Pitfalls

**Domain:** Cryptographic identity platform with real-time signal delivery (ATAP)
**Researched:** 2026-03-11

## Critical Pitfalls

Mistakes that cause rewrites, security vulnerabilities, or architectural dead ends.

### Pitfall 1: Redis Pub/Sub Message Loss Treated as Reliable Delivery

**What goes wrong:** Redis pub/sub is fire-and-forget with at-most-once semantics. If an SSE client disconnects and reconnects, any signals published to Redis during the gap are permanently lost from the pub/sub channel. The current architecture in the build guide relies on `Last-Event-ID` replay from PostgreSQL to cover this gap, but a race condition exists: a signal can be published to Redis *before* it is committed to PostgreSQL, meaning the replay query misses it. Conversely, if the signal is committed first but Redis publish fails silently, the connected client never gets the real-time notification.

**Why it happens:** Redis pub/sub is designed for ephemeral broadcast, not durable messaging. Developers treat `PUBLISH` as "send to queue" when it is actually "broadcast to whoever is listening right now." The build guide's SSE handler subscribes to Redis *after* replaying from PostgreSQL, creating a window where signals can fall through.

**Consequences:** Silent signal loss. An agent misses a critical approval signal. No error is raised because Redis considers the publish successful regardless of whether anyone received it. In a trust protocol, missed signals directly undermine reliability guarantees.

**Prevention:**
1. Write to PostgreSQL first (single source of truth), then publish to Redis as a notification-only channel (just the signal ID, not the full payload).
2. On SSE reconnect, replay from PostgreSQL using `Last-Event-ID` cursor, then subscribe to Redis. Use a short overlap window: after subscribing, re-query PostgreSQL for any signals created between the replay cursor and "now" to close the race.
3. Alternatively, use Redis Streams (`XADD`/`XREAD`) instead of pub/sub for the real-time channel. Streams persist messages and support consumer groups with acknowledgment. This is a better fit for durable delivery but adds complexity.
4. Add delivery confirmation: track whether a signal was actually received via SSE and expose undelivered signals in the poll endpoint.

**Detection:** Integration tests that kill and restart SSE connections mid-signal-send. Monitor for signals that exist in PostgreSQL but were never delivered via SSE or webhook.

**Phase:** Phase 1 -- this is core to the signal delivery system and must be correct from day one.

---

### Pitfall 2: Canonical JSON Signing Without RFC 8785 (JCS)

**What goes wrong:** The current `CanonicalJSON` implementation in `crypto.go` relies on Go's `encoding/json` default behavior (sorted map keys, no extra whitespace). This works within Go but breaks cross-language signature verification. Different languages serialize JSON differently: floating point precision varies, Unicode escaping varies, key ordering in nested structures may differ. A signal signed by a Go server cannot be reliably verified by a Python or JavaScript SDK.

**Why it happens:** Go's `json.Marshal` happens to sort top-level map keys, so it *looks* canonical in simple tests. But it does not implement RFC 8785 (JSON Canonicalization Scheme), which specifies exact rules for number formatting (IEEE 754 double-precision representation), Unicode normalization, and recursive key sorting. The comment in the code (`// Go's encoding/json sorts map keys by default`) shows awareness but not the full picture.

**Consequences:** Cross-SDK signature verification fails intermittently depending on payload content. Signals containing floating-point numbers, Unicode characters, or deeply nested objects break first. This becomes a showstopper when Python and JavaScript SDKs ship in Phase 1.

**Prevention:**
1. Implement RFC 8785 JCS properly or use a tested JCS library for Go (e.g., `github.com/AmbitionEng/go-jcs` or similar).
2. Define a clear signing specification: "The signable payload is JCS(route) + '.' + JCS(signal)" and document it in the spec.
3. Build cross-language signature verification tests as part of CI: sign in Go, verify in Python and JS (and vice versa).
4. Avoid floating-point numbers in signal payloads where possible (use string representations for money, timestamps as ISO 8601 strings).
5. If JCS feels heavyweight for Phase 1, an alternative is to sign the raw bytes as received (don't re-serialize), but this requires transmitting the exact signed bytes alongside the signature, which complicates the API.

**Detection:** Cross-language integration tests that sign payloads in one language and verify in another. Test with edge-case payloads: Unicode emoji, large integers, floating-point values, deeply nested objects.

**Phase:** Phase 1 -- the `SignablePayload` function in `crypto.go` must be correct before any SDK ships. Retrofitting canonical serialization after SDKs are published is a breaking change.

---

### Pitfall 3: Private Key Returned in Registration Response

**What goes wrong:** The build guide's `POST /v1/register` endpoint returns the entity's private key in the HTTP response. This means the private key transits the network, passes through TLS termination (Caddy), potentially gets logged by reverse proxies, and is visible in server memory. For a protocol whose core value proposition is "cryptographically verify who authorized that agent," having the platform generate and handle private keys undermines the trust model.

**Why it happens:** It is the simplest developer experience -- register, get back everything you need. Client-side key generation adds friction (the client needs crypto libraries). For agents (ephemeral, server-side), this is arguably acceptable. But the pattern sets a precedent that extends to human keys, where it is unacceptable.

**Consequences:** The platform becomes a single point of key compromise. If the server is breached, all private keys generated since deployment are exposed. The security model claims "identity is the key" but the platform has held every key. Audit-aware adopters will reject this design.

**Prevention:**
1. For Phase 1 (agents only): Accept client-generated public keys in the registration request. The client generates the Ed25519 keypair locally and sends only the public key. The server never sees the private key.
2. Provide a convenience mode for quick testing: allow the server to generate keypairs and return them, but mark this as "dev mode only" and log a warning. Never enable this in production deployments.
3. For the mobile app (Phase 1 foundation): Generate keys in the device's secure enclave / keystore from day one. Never transmit the private key.
4. Document the trust model clearly: "The platform never possesses your private key. If the platform generated your key, you should rotate it."

**Detection:** Grep server logs and response bodies for base64-encoded private key material. Code review any endpoint that calls `GenerateKeyPair()` and returns both values.

**Phase:** Phase 1 -- must be decided before the registration API ships. Changing the registration flow after SDKs are published is a breaking change.

---

### Pitfall 4: SSE Connection Exhaustion and Proxy Buffering

**What goes wrong:** SSE connections are long-lived HTTP connections. Each connected agent holds open a goroutine, a Redis subscription, and a TCP connection. Without connection limits, a modest number of agents (thousands) can exhaust server resources. Additionally, any proxy between the server and client (Caddy, nginx, cloud load balancers) may buffer SSE responses, causing signals to arrive in bursts rather than real-time, or triggering idle timeouts that silently kill connections.

**Why it happens:** SSE looks simple (it is just HTTP), but long-lived connections have fundamentally different resource characteristics than request-response. The build guide's SSE handler creates a new Redis subscription per connection, which does not scale. Proxy buffering is the most commonly reported SSE production issue -- proxies legally store response chunks and forward them when the stream closes.

**Consequences:** At scale: server OOM from accumulated goroutines and Redis subscriptions. In production: signals appear delayed by 30-60 seconds (proxy buffering) or connections silently die (idle timeout) and the client does not know it missed signals.

**Prevention:**
1. Set `X-Accel-Buffering: no` (already in build guide), `Cache-Control: no-cache`, and `Connection: keep-alive` headers.
2. Send heartbeat comments (`: heartbeat\n\n`) every 15-30 seconds to keep connections alive through proxies and detect dead connections.
3. Implement connection limits per entity (one active SSE connection per entity ID; new connection closes the old one).
4. Use a shared Redis subscription fan-out pattern: one Redis subscription per server instance (not per client), then fan out internally to connected clients using Go channels.
5. Set a maximum SSE connection duration (e.g., 24 hours) and force reconnection to rebalance across server instances.
6. Configure Caddy explicitly for SSE: ensure `flush_interval -1` is set for the SSE route.
7. Monitor active SSE connections as a key metric.

**Detection:** Load test with 100+ concurrent SSE connections. Monitor goroutine count and Redis subscription count. Test through the full proxy chain (Caddy), not just direct to the Go server.

**Phase:** Phase 1 -- the SSE delivery system is a Phase 1 deliverable. The fan-out optimization can be deferred to later in Phase 1, but proxy configuration and heartbeats must be correct from day one.

---

### Pitfall 5: Token Lookup Without Constant-Time Comparison

**What goes wrong:** The auth middleware hashes the bearer token with SHA-256 and looks it up in PostgreSQL. The SHA-256 hash eliminates character-by-character timing attacks on the token itself (good). However, if the database comparison uses a standard `WHERE token_hash = $1` query, the database index lookup leaks timing information about whether a hash *prefix* exists. More practically: if the `HashToken` function or any intermediate comparison uses `bytes.Equal` instead of `crypto/subtle.ConstantTimeCompare`, a timing side-channel exists.

**Why it happens:** The SHA-256 hashing approach is the standard pattern (used by GitHub, Stripe, etc.) and is correct in principle. The pitfall is in implementation details: Go's `bytes.Equal` is not constant-time, and PostgreSQL's `=` operator on `BYTEA` is not constant-time. For most applications this is acceptable because the hash provides sufficient protection, but for a security-focused identity protocol, it warrants attention.

**Consequences:** In practice, the risk is low because SHA-256's avalanche property means a single-bit change in the token produces a completely different hash. An attacker cannot incrementally guess the token through timing. The real risk is more mundane: token enumeration via error message differences ("invalid token" vs. "token expired" vs. no response) can leak information about which tokens exist.

**Prevention:**
1. Keep the SHA-256 hash approach (it is correct).
2. Use `crypto/subtle.ConstantTimeCompare` if comparing hashes in application code.
3. Return identical error responses for "token not found" and "token invalid" -- same HTTP status, same response body, same response time.
4. Add rate limiting on auth failures (e.g., 10 failures per IP per minute).
5. Consider adding a token prefix check before the database lookup: if the token does not start with `atap_`, reject immediately. This is fast and leaks no information about valid tokens.

**Detection:** Review auth middleware for variable-time responses. Test that invalid tokens and non-existent tokens produce identical responses. Measure response time distribution for valid vs. invalid tokens.

**Phase:** Phase 1 -- auth middleware is a Phase 1 deliverable.

---

### Pitfall 6: ULID Monotonic Entropy Source Shared Across Goroutines

**What goes wrong:** The current `NewEntityID()` and `NewSignalID()` functions in `crypto.go` create a new `ulid.Monotonic` entropy source on every call. This is wasteful but not dangerous. The real pitfall comes if this is "optimized" to share a single `ulid.Monotonic` source across goroutines: `ulid.Monotonic` is not goroutine-safe. Concurrent access causes data races or panics. Alternatively, if a global monotonic source is protected by a mutex, it becomes a contention bottleneck under load.

**Why it happens:** The ULID spec's monotonic mode increments the random component within the same millisecond to preserve ordering. This requires mutable state, which is inherently unsafe for concurrent access. Developers see "create entropy source once, reuse" as an optimization without realizing the concurrency implications.

**Consequences:** Data races causing duplicate or corrupted IDs (silent data corruption). Under mutex protection: latency spikes during high signal throughput. With per-call creation (current code): no monotonic ordering guarantee across concurrent requests within the same millisecond, but IDs are still globally unique.

**Prevention:**
1. Keep the current per-call pattern for now -- it is safe and correct for Phase 1 volumes.
2. If monotonic ordering matters (e.g., for cursor-based pagination), use a per-goroutine or pooled entropy source with proper synchronization.
3. Document that signal ordering is by `created_at` timestamp in PostgreSQL, not by signal ID lexicographic order. Do not rely on ULID ordering for pagination -- use database timestamps.
4. Add a `sync.Pool` of entropy sources if benchmarking shows allocation pressure.

**Detection:** Run race detector (`go test -race`) on all ID generation code. Load test concurrent signal creation and verify no duplicate IDs.

**Phase:** Phase 1 -- the current implementation is acceptable. Document the ordering contract. Optimize only if benchmarks justify it.

---

## Moderate Pitfalls

### Pitfall 7: Webhook Signature Using Platform Key Instead of Sender Key

**What goes wrong:** The build guide's webhook delivery signs outbound webhooks with the *platform's* private key, not the *sending entity's* key. This means the webhook recipient can verify the payload came from the ATAP platform but cannot verify which entity sent the signal. The platform becomes a trusted intermediary, which contradicts the "verify without depending on a central authority" design principle.

**Prevention:**
1. For Phase 1 (platform-hosted): signing with the platform key is acceptable because all signals route through the platform anyway.
2. Include the original signal's `trust` block (with the sender's signature) in the webhook payload so the recipient can verify both the platform delivery signature and the sender's content signature.
3. Document this as a known limitation that federation (Phase 4) will address.

**Phase:** Phase 1 -- acceptable compromise. Flag for Phase 4 federation work.

---

### Pitfall 8: Unbounded Inbox Growth

**What goes wrong:** Without TTL enforcement and inbox size limits, a heavily-targeted entity's inbox grows indefinitely. PostgreSQL performance degrades as the signals table grows. The `idx_signals_target_created` index becomes bloated. Expired signals (past TTL) continue consuming storage and slowing queries.

**Prevention:**
1. Implement a background job that deletes expired signals (`WHERE expires_at < NOW()`) on a schedule (every 5 minutes).
2. Set a default TTL for signals that do not specify one (e.g., 30 days).
3. Add a per-entity inbox size limit (e.g., 10,000 signals). When exceeded, the oldest delivered signals are purged.
4. Partition the signals table by month if expecting high volume early.
5. Add `EXPLAIN ANALYZE` monitoring on inbox queries to detect degradation.

**Phase:** Phase 1 -- the TTL expiration job should ship with the initial delivery system. Partitioning can wait.

---

### Pitfall 9: Channel Webhook URLs Guessable or Enumerable

**What goes wrong:** The `NewChannelID` function generates 8 random bytes (16 hex characters) for channel IDs. The inbound webhook URL is `POST /v1/channels/{channel-id}/signals`. If an attacker can guess or enumerate channel IDs, they can inject signals into any entity's inbox. 8 bytes (64 bits) of entropy is borderline -- it resists brute force but is vulnerable to birthday attacks at scale.

**Prevention:**
1. Increase channel ID entropy to at least 16 bytes (128 bits): `chn_` + 32 hex characters.
2. Add rate limiting on the inbound webhook endpoint per source IP.
3. Consider adding an optional HMAC signature requirement for inbound webhooks (the channel creator gets a shared secret).
4. Log and alert on high-volume 404s on the channels endpoint (enumeration attempt).

**Detection:** Review the entropy of `NewChannelID()` -- currently 64 bits. Calculate birthday bound: with 2^32 channels (~4 billion), collision probability reaches 50%. With fewer channels but active enumeration, 64 bits may be guessed in feasible time if rate limiting is absent.

**Phase:** Phase 1 -- increase entropy before shipping. The HMAC option can be Phase 2.

---

### Pitfall 10: Flutter Ed25519 Library Mismatch with Go

**What goes wrong:** The build guide specifies `cryptography_flutter` and `pointycastle` for Ed25519 on mobile. Go uses the standard library `crypto/ed25519`. These implementations must produce identical signatures for identical inputs. Ed25519 is deterministic (same key + same message = same signature), but subtle differences in key encoding, message pre-processing, or the distinction between Ed25519 and Ed25519ph (pre-hashed) can cause cross-platform verification failures.

**Prevention:**
1. Write a cross-platform test suite: generate a keypair in Go, export the seed (32 bytes), import in Dart, sign the same message, verify signatures match exactly.
2. Use only standard Ed25519 (not Ed25519ph or Ed25519ctx) across all platforms.
3. Standardize key encoding: the spec says base64 standard encoding for public keys. Ensure Dart and Go agree on the encoding of the 64-byte private key (or 32-byte seed).
4. Pin specific versions of `cryptography_flutter` and `pointycastle` and test against them.
5. Prefer `cryptography_flutter` (delegates to native OS APIs) over `pointycastle` (pure Dart) for production -- native implementations are faster and better audited.

**Detection:** Cross-platform signing tests in CI. Sign a known message with a known key in each platform and assert byte-identical signatures.

**Phase:** Phase 1 mobile foundation -- must be verified before the mobile app registers its first entity.

---

### Pitfall 11: Missing Entity Ownership Authorization

**What goes wrong:** The authorization rules allow "any entity" to send a signal to "any other entity." Combined with the public entity lookup endpoint, this means any registered agent can spam any other entity's inbox. There is no concept of "I only accept signals from entities I have a relationship with" in Phase 1.

**Prevention:**
1. For Phase 1: accept the open inbox model (agents need to receive unsolicited signals from services they interact with).
2. Implement rate limiting per sender-target pair (e.g., 10 signals per minute from entity A to entity B).
3. Add optional allowlist/blocklist per entity (can be Phase 2).
4. Monitor for abuse patterns: single entity sending to many targets, high-volume signal creation.

**Phase:** Phase 1 rate limiting; Phase 2 for allowlist/blocklist.

---

## Minor Pitfalls

### Pitfall 12: Docker Compose Secrets in Environment Variables

**What goes wrong:** Database passwords, Redis credentials, and platform signing keys are passed via environment variables in Docker Compose. These are visible in `docker inspect`, process listings, and container orchestrator UIs.

**Prevention:** Use Docker secrets or `.env` files with restricted permissions for local development. For production (Coolify/Hetzner), use the deployment platform's secret management. Never commit `.env` files.

**Phase:** Phase 1 -- set up proper secret handling from the start.

---

### Pitfall 13: No Signal Validation Beyond JSON Parsing

**What goes wrong:** The inbound webhook handler accepts "any JSON" and wraps it in an ATAP signal. Without schema validation, malformed or malicious payloads (deeply nested objects, enormous strings, unexpected types) can cause storage bloat, rendering issues in the mobile app, or JSON parsing bombs.

**Prevention:**
1. Set a maximum signal payload size (e.g., 64 KB).
2. Limit JSON nesting depth (e.g., 10 levels).
3. Validate required signal fields (version, route.target) before storage.
4. For inbound webhooks: sanitize but do not reject unknown fields (channels receive arbitrary external data).

**Phase:** Phase 1 -- validation middleware should ship with the signal ingestion endpoints.

---

### Pitfall 14: SSE Reconnection Thundering Herd

**What goes wrong:** If the server restarts or a network blip disconnects all SSE clients simultaneously, they all reconnect at once. Each reconnection triggers a PostgreSQL replay query (`GetSignalsAfter`). Thousands of simultaneous complex queries can overwhelm the database.

**Prevention:**
1. Add jittered reconnection delay on the client side (SDK and mobile app): `baseDelay + random(0, baseDelay)`.
2. Cache recent signals in Redis (last N signals per entity) for fast replay without hitting PostgreSQL.
3. Add connection admission control: if the server is overloaded, return `503 Retry-After` with a random delay.

**Phase:** Phase 1 SDKs should implement jittered reconnection. Server-side caching can be Phase 2.

---

### Pitfall 15: Public Key Encoding Inconsistency

**What goes wrong:** The current `EncodePublicKey` uses `base64.StdEncoding` (with padding), but tokens use `base64.RawURLEncoding` (no padding, URL-safe). The spec should pick one encoding for all base64 in the protocol. Mixed encodings cause confusion and bugs when developers copy-paste between contexts.

**Prevention:**
1. Standardize on `base64.RawURLEncoding` (no padding, URL-safe) for all binary data in the protocol: public keys, signatures, encrypted payloads.
2. Document this in the spec's cryptographic primitives section.
3. Accept both padded and unpadded base64 on input (be liberal in what you accept) but always produce URL-safe unpadded on output.

**Phase:** Phase 1 -- decide before the first public API response is sent.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Entity Registration | Private key exposure in HTTP response | Client-side key generation, server receives only public key |
| Signal Delivery (SSE) | Message loss during reconnect gap | PostgreSQL as source of truth, Redis as notification only, overlap replay window |
| Signal Delivery (SSE) | Proxy buffering breaks real-time | Explicit Caddy config (`flush_interval -1`), heartbeats every 15-30s |
| Signal Delivery (Webhook) | Signature only proves platform, not sender | Include original sender signature in webhook payload |
| Crypto (Signing) | Cross-language signature verification failure | Implement RFC 8785 JCS, cross-platform CI tests |
| Crypto (Keys) | Flutter/Go Ed25519 incompatibility | Cross-platform signing test suite with known test vectors |
| Auth Middleware | Token timing side-channel | SHA-256 hash (already planned), identical error responses, rate limiting |
| Channels | Webhook URL enumeration | Increase to 128-bit entropy, rate limit inbound endpoint |
| Database | Unbounded inbox growth | TTL expiration job, default TTL, per-entity size limits |
| Mobile Foundation | Key not in secure enclave | Generate Ed25519 keys via native secure enclave APIs from day one |
| Docker/Deployment | Secrets in environment variables | Docker secrets, restricted `.env` files, never commit credentials |
| SDK Interop | Base64 encoding inconsistency | Standardize on base64url (no padding) across the entire protocol |

## Sources

- [Redis Pub/Sub documentation](https://redis.io/docs/latest/develop/pubsub/) -- at-most-once delivery semantics
- [Redis Pub/Sub reliability issue #7855](https://github.com/redis/redis/issues/7855) -- connection detection problems
- [RFC 8785: JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785) -- standard for deterministic JSON serialization
- [SSE production readiness concerns](https://dev.to/miketalbot/server-sent-events-are-still-not-production-ready-after-a-decade-a-lesson-for-me-a-warning-for-you-2gie) -- proxy buffering issues
- [ULID monotonic collision analysis](https://zendesk.engineering/how-probable-are-collisions-with-ulids-monotonic-option-d604d3ed2de) -- concurrency and ordering
- [oklog/ulid Go library](https://github.com/oklog/ulid) -- goroutine safety considerations
- [Timing attacks on token comparison](https://paragonie.com/blog/2015/11/preventing-timing-attacks-on-string-comparison-with-double-hmac-strategy) -- constant-time comparison patterns
- [Flutter secure enclave key storage](https://vibe-studio.ai/insights/using-secure-enclaves-for-key-storage-in-flutter) -- mobile key management
- [Flutter application security considerations](https://www.cossacklabs.com/blog/flutter-application-security-considerations/) -- mobile crypto pitfalls
- [10 Cryptography Mistakes](https://www.appsecengineer.com/blog/10-cryptography-mistakes-youre-probably-making) -- general crypto implementation errors
