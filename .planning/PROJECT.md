# ATAP — Agent Trust and Authority Protocol

## What This Is

An open protocol and hosted platform for verifiable delegation of trust between AI agents, machines, humans, and organizations. Every entity gets a cryptographic identity (Ed25519), a durable inbox for signal delivery, and a verifiable delegation chain. ATAP sits below A2A, MCP, and AP2 — providing the identity and trust layer they assume but don't define.

## Core Value

Any party receiving a request from an AI agent can cryptographically verify who authorized that agent, what it is permitted to do, and under what constraints — without depending on a central authority.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Agent registration with Ed25519 keypair generation, token issuance, and ULID-based IDs
- [ ] Entity lookup by ID (public endpoint, returns public key + metadata)
- [ ] Signal sending between entities (JSON payload, routing metadata, optional TTL/priority/tags)
- [ ] Durable inbox with PostgreSQL persistence and cursor-based pagination
- [ ] SSE streaming delivery with Redis pub/sub, Last-Event-ID replay, and 30s heartbeat
- [ ] Webhook push delivery with signature verification and exponential backoff retry
- [ ] Inbound channels (unique webhook URLs per external service, wrapping payloads into ATAP signals)
- [ ] Channel lifecycle (create, list, revoke, expiration)
- [ ] Bearer token auth middleware (SHA-256 hashed storage, never plaintext)
- [ ] RFC 7807 error responses on all endpoints
- [ ] Health endpoint returning protocol version and status
- [ ] Database migrations (numbered SQL, pgcrypto extension)
- [ ] Docker Compose setup for full stack (platform + PostgreSQL 16 + Redis 7)
- [ ] Dockerfile for cloud-deployable platform binary
- [ ] Flutter mobile app foundation (entity registration, inbox view, push notification setup)
- [ ] Integration tests covering the full agent lifecycle (register → send signal → receive via SSE)

### Out of Scope

- Human entities, attestations, claim flow — Phase 2
- Delegation documents and verification — Phase 2
- World ID and SMS verification integrations — Phase 2
- Branded approval templates — Phase 3
- Organizations — Phase 3
- End-to-end encryption — Phase 3
- Federation and key discovery — Phase 4
- Client SDKs (Python, JS, Go) — deferred past Phase 1

## Context

- The protocol spec is defined in `ATAP-SPEC-v0.1.md` and `spec/ATAP-SPEC-v0.1.md`
- The full build guide with detailed phase breakdowns, API specs, and database schemas is in `ATAP-BUILD-GUIDE.md` and `docs/BUILD-GUIDE.md`
- Existing scaffolding lives in `platform/` — Go code with the right structure but not yet tested or fully wired up
- Entity model has four types: `agent://`, `machine://`, `human://`, `org://` — Phase 1 focuses on agents
- Signal IDs use `sig_` prefix + ULID, channel IDs use `chn_` prefix + hex, tokens use `atap_` prefix + base64url
- Human IDs are derived from public key (`lowercase(base32(sha256(pubkey))[:16])`) — identity is the key, not email/phone

## Constraints

- **Tech stack**: Go 1.22+ with Fiber v2, PostgreSQL 16 with JSONB, Redis 7 for pub/sub, Flutter 3.x for mobile
- **Crypto**: Ed25519 signing, X25519 encryption, tokens stored as SHA-256 hashes
- **Deployment**: Must work via `docker compose up` locally and be deployable to cloud via Dockerfile
- **Phase order**: Phase 1 must work end-to-end before Phase 2 begins — strict sequential dependency
- **License**: Apache 2.0 for everything (spec, platform, SDKs, mobile app)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Ed25519 for all signatures | Small keys (32B), fast, deterministic, wide library support | — Pending |
| SSE as primary delivery | Unidirectional, HTTP-native, built-in reconnection, aligns with MCP | — Pending |
| ULID-based entity IDs | Time-sortable, globally unique, URL-safe | — Pending |
| Human ID derived from public key | Self-sovereign identity, GDPR-friendly, no external provider dependency | — Pending |
| PostgreSQL JSONB for flexible payloads | Attestations, signal data, delegation constraints all benefit from schema flexibility | — Pending |
| Backend only for Phase 1 focus + Flutter foundation | Get core platform solid before building full mobile experience | — Pending |

---
*Last updated: 2026-03-11 after initialization*
