# Feature Landscape

**Domain:** Agent identity, trust delegation, and signal delivery protocol platform
**Researched:** 2026-03-11

## Table Stakes

Features users expect for a Phase 1 agent trust and inbox platform. Missing any of these means the platform feels broken or incomplete to early adopters (agent developers, framework authors, protocol experimenters).

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Agent self-registration with Ed25519 keypair | Zero-friction onboarding is the #1 adoption driver. Agents must register in <1s with no human approval. Every competing system (A2A Agent Cards, Microsoft Entra Agent ID, AgentMail) has instant programmatic registration. | Low | `POST /v1/register` returns entity URI, keypair, and bearer token. ULID-based IDs. |
| Entity lookup (public key + metadata) | Verifiers need to resolve an entity to its public key. Without this, no signature verification is possible. Foundational for all trust operations. | Low | `GET /v1/entities/{id}` is public, returns public key and metadata only. No secrets. |
| Signal sending to inbox | The core value proposition: "give your agent a doorbell." If agents cannot receive messages, there is no platform. AgentMail raised $6M proving inbox-for-agents has demand. | Medium | `POST /v1/inbox/{target-id}` with JSON payload, routing metadata, optional TTL/priority/tags. |
| Durable inbox with persistence | Agents are ephemeral and offline frequently. Signals must persist across disconnections. Every messaging system (email, SQS, Kafka) durably stores messages. Losing signals is unacceptable. | Medium | PostgreSQL-backed, cursor-based pagination (`?after=sig_xxx&limit=50`). Signals survive restarts. |
| SSE streaming delivery | Real-time push is expected in 2026. MCP uses SSE for server-to-client streaming. Built-in reconnection via `Last-Event-ID` makes SSE the natural choice for agent signal delivery. | Medium | Redis pub/sub fan-out, 30s heartbeat, Last-Event-ID replay from PostgreSQL on reconnect. |
| Webhook push delivery | Many agents run in serverless/cron environments where persistent connections are impossible. Webhook delivery is the universal integration pattern. | Medium | Platform POSTs to entity's registered URL, Ed25519 signature in `X-ATAP-Signature` header, exponential backoff retry (1s, 5s, 30s, 5m, 30m). |
| Inbound channels (webhook URLs) | Agents need to receive signals from external services (Stripe, GitHub, SIMRelay). Unique webhook URL per service enables per-service revocation and audit trails. | Medium | `POST /v1/entities/{id}/channels` returns unique webhook URL. Labels, tags, expiration, rate limiting. |
| Channel lifecycle management | Create, list, revoke channels. Without revocation, a compromised channel URL cannot be disabled without affecting other integrations. | Low | `GET /v1/entities/{id}/channels`, `DELETE /v1/channels/{channel-id}`. |
| Bearer token authentication | Every API needs auth. SHA-256 hashed token storage (never plaintext) is baseline security. Token rotation must be supported from day one. | Low | `Authorization: Bearer {atap_...}` header, `POST /v1/auth/rotate` for rotation. |
| RFC 7807 error responses | Structured errors are table stakes for any developer-facing API. Without them, debugging is painful and integration is slow. | Low | All error responses follow Problem Details format with `type`, `title`, `status`, `detail`. |
| Health endpoint | Required for container orchestration (Docker, Kubernetes), load balancers, and monitoring. Every production service has one. | Low | `GET /v1/health` returns protocol version, status, uptime. |
| Idempotency support | Agents retry. Networks fail. Without idempotency keys, duplicate signals are inevitable. This is table stakes for any message delivery system. | Low | `context.idempotency` field, deduplication within 24-hour window via unique index. |
| Docker Compose local development | Developers must be able to `docker compose up` and have a working platform in under 60 seconds. Friction here kills adoption. | Low | PostgreSQL 16 + Redis 7 + Go platform binary, single command. |
| Signal format with route/signal/context blocks | The signal format is the protocol's core data structure. It must be well-defined, versioned, and documented from day one. Opaque signal body ensures forward compatibility. | Low | Version field `v: "1"`, route (origin/target/reply_to/channel/thread/ref), signal (type/encrypted/data), context (source/idempotency/tags/ttl/priority). |
| Polling fallback | SSE and webhooks cover most cases, but some environments (serverless functions, batch jobs) can only poll. This is the safety net. | Low | `GET /v1/inbox/{entity-id}?after={cursor}&limit=50`. Cursor-based pagination. |

## Differentiators

Features that set ATAP apart from the current ecosystem. Not expected by users yet (because the agent identity space is nascent), but these create the competitive moat.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Cryptographic entity identity (Ed25519) | Unlike AgentMail (email-based) or Microsoft Entra Agent ID (AAD-based), ATAP gives every entity a cryptographic keypair. This enables offline verification, self-sovereign identity, and protocol-level trust without depending on any cloud provider. | Medium | Server-side keypair generation for agents in Phase 1. Secure enclave generation for humans in Phase 2. |
| Human-derived identity from public key | `human_id = lowercase(base32(sha256(pubkey))[:16])` means the identity IS the key. No email, no phone number, no external provider dependency. GDPR-friendly by design. No competing platform does this. | Low | Phase 2 feature, but the design decision shapes Phase 1 entity model. |
| Entity URI scheme (`agent://`, `machine://`, `human://`, `org://`) | A universal addressing scheme for the agent economy. Comparable to email addresses but purpose-built for machine-to-machine trust. A2A has Agent Cards but no universal addressing. | Low | URI format: `{type}://{identifier}`. Enforced from registration. |
| Signal threading and references | Signals can reference previous signals (`ref`) and group into threads (`thread`). This enables conversational patterns, approval flows, and task chains that simple message queues cannot express. | Low | Fields in route block. Thread ID for grouping, ref ID for reply chains. |
| Progressive trust levels (0-3) | Trust is not binary. An anonymous agent (Level 0) can still receive signals. Elevating trust unlocks capabilities. No competing protocol has this graduated model with clear verification requirements per level. | Low | Trust level stored on entity, recalculated on attestation changes. Services set minimum trust requirements. Phase 1 supports Level 0 only; higher levels in Phase 2+. |
| Protocol-level position below MCP/A2A/AP2 | ATAP is explicitly the identity and trust layer that MCP, A2A, and AP2 assume exists but do not define. This positioning as infrastructure (not application) is unique. Microsoft and Google are building identity into their own stacks; ATAP is the open, vendor-neutral alternative. | N/A | Architectural positioning, not a feature to build. But it shapes every design decision. |
| Canonical JSON signature scheme | Deterministic signature computation over route + signal blocks enables offline verification. Self-verifying documents (like passports) do not require contacting a central server. | Medium | `canonical(route) + "." + canonical(signal)` signed with Ed25519. Verifiers need only the public key. |
| Channel-per-service isolation | One inbox, many channels. Each external service gets its own webhook URL. Revoking a compromised GitHub webhook does not affect the Stripe webhook. AgentMail does not have this concept (it is email-inbox-per-agent). | Low | Already in table stakes as a basic feature; the isolation and per-service audit trail is the differentiator. |
| Flutter mobile app as human ATAP client | Humans participate in the signal protocol as first-class entities, not through a dashboard. The mobile app is the human's ATAP client: receive signals, approve delegations, send signed instructions to agents. No competing platform has a mobile-native trust interface. | High | Phase 1: foundation (registration, inbox view, push notifications). Phase 2: claim approval, delegation management. |
| Push notifications for signal delivery | When a signal arrives for a human entity, the mobile app receives a push notification. This bridges the gap between machine-speed agent communication and human attention. | Medium | FCM for Android, APNs for iOS. Push token stored per entity. Signal triggers push. |
| Open protocol with Apache 2.0 license | Everything is open: spec, platform, SDKs, mobile app. Microsoft Entra Agent ID is proprietary. Indicio ProvenAI is commercial. ATAP's openness enables ecosystem adoption. | N/A | Licensing decision, not a feature. But it is a major differentiator. |

## Anti-Features

Features to explicitly NOT build in Phase 1. These are either premature, out of scope, or would create unnecessary complexity.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Delegation documents and verification | Phase 2 feature. Building trust chains before the inbox works end-to-end is premature optimization. The spec is ready, but the platform must prove signal delivery works first. | Ship signal delivery. Design the entity model to accommodate delegations (owner_id, trust_level fields), but do not implement delegation CRUD or chain verification. |
| Human entity registration and attestations | Humans are Phase 2. Phase 1 focuses on agent-to-agent signal delivery. Adding human registration adds email/phone verification flows, attestation storage, and trust level elevation logic. | Build the Flutter app foundation (registration screen, inbox view, push setup), but do not wire up human entity creation on the backend. |
| Claim flow (agent-initiated trust elevation) | Requires both human entities and delegation documents. Cannot work without Phase 2 infrastructure. | Design the claim flow in the spec, but do not implement `POST /v1/claims` or approval screens. |
| End-to-end encryption (X25519) | Phase 3 feature. Encryption adds significant complexity (key exchange, encrypted signal handling, zero-knowledge relay). The platform must be trusted initially; encryption comes after trust is established. | Store signals in plaintext JSONB. Reserve the `trust.enc` field in the signal format for future use. The `encrypted` boolean on signals should default to `false`. |
| Branded approval templates | Phase 3 feature. Requires machine entities, delegation chains, and a rich mobile approval UI. Premature without the trust chain. | Reserve template schema in the spec. Do not implement `POST /v1/templates` or template rendering. |
| Organization entities | Phase 3 feature. Orgs are trust umbrellas for humans and machines. They add domain verification, org-level revocation, and hierarchical entity management. Not needed until the trust chain exists. | Reserve `org://` URI scheme. Do not implement `POST /v1/orgs`. |
| Federation and key discovery | Phase 4 feature. DNS TXT records, `.well-known/atap.json`, cross-registry key resolution. This is the "become a standard" play and is meaningless without adoption. | Run a single registry (`atap.app`). Design the entity record to include `registry` and `discovery` fields for future federation. |
| Client SDKs (Python, JS, Go) | The build guide includes SDKs in Phase 1 Weeks 3-4, but SDKs are premature before the API is stable. API changes during Phase 1 development would require SDK updates. Ship the platform first, then SDKs. | Provide curl examples and OpenAPI spec. Build SDKs after Phase 1 API is frozen. **Note: the build guide disagrees here -- it includes SDKs in Phase 1. If adoption speed is prioritized over API stability, a minimal Python SDK could ship.** |
| Landing page (atap.dev) | Marketing before the product works is premature. The build guide includes it in Week 3. | Write a good README with quickstart examples. Defer the landing page until the platform is deployed and functional. |
| Rate limiting per tier / monetization | Phase 3 feature. Adding Stripe integration and tier management before proving product-market fit is wasted effort. | Implement basic rate limiting (global, per-entity) for abuse prevention, but no tier system or billing. |
| World ID integration | Phase 2 feature (Trust Level 2). Requires human entities, attestation storage, and mobile WebView integration with IDKit v4. | Reserve the `world_id` attestation type in the schema. Do not implement the verification flow. |
| Signal signature verification middleware | Phase 1 entities are Trust Level 0 (anonymous). Signature verification becomes mandatory only when Trust Level 1+ entities exist (Phase 2). The trust block in signals is optional for Level 0. | Accept the `trust` block in signals if provided, store it, but do not verify signatures in Phase 1. Pass-through only. |
| Key recovery (encrypted backup) | Phase 2 feature. Only needed for human entities whose identity is derived from their keypair. Agents get new keypairs on re-registration. | Reserve `recovery_backup` JSONB column in entities table. Do not implement `POST /v1/recovery/backup`. |

## Feature Dependencies

```
Agent Registration --> Entity Lookup (must be able to look up registered entities)
Agent Registration --> Bearer Token Auth (registration issues the token)
Signal Sending --> Agent Registration (sender and target must exist)
Signal Sending --> Durable Inbox (signals must be stored)
Durable Inbox --> Signal Format (inbox stores signals in defined format)
SSE Streaming --> Durable Inbox (replay from PostgreSQL on reconnect)
SSE Streaming --> Redis Pub/Sub (real-time fan-out)
Webhook Push --> Durable Inbox (webhook delivers stored signals)
Webhook Push --> Ed25519 Signing (platform signs webhook payloads)
Inbound Channels --> Agent Registration (channels belong to entities)
Inbound Channels --> Signal Sending (channel webhook creates signal in inbox)
Push Notifications --> Flutter App Foundation (app registers push token)
Push Notifications --> Signal Sending (signal triggers push)
Flutter App Foundation --> Agent Registration (app registers entity)

Phase 2 Dependencies:
Human Registration --> Agent Registration (same entity model)
Claim Flow --> Human Registration + Delegation Documents
Delegation Documents --> Ed25519 Signing + Canonical JSON
Attestations --> Human Registration
Trust Level Elevation --> Attestations + Claim Flow

Phase 3 Dependencies:
Branded Templates --> Machine Registration + Delegation Documents
Organizations --> Human Registration + Delegation Documents
E2E Encryption --> X25519 Key Generation + Signal Format Extension
```

## MVP Recommendation

**Phase 1 MVP: "The Doorbell" -- An agent can register, get an inbox, and receive signals.**

Prioritize in this order:

1. **Agent registration with Ed25519 keypair and bearer token** -- The entry point. Everything else depends on this.
2. **Signal sending and durable inbox** -- The core value. "Send a message to an agent and it arrives even if the agent was offline."
3. **SSE streaming delivery** -- Real-time push. This is what makes the experience feel alive vs. polling.
4. **Inbound channels** -- External services can push signals to agent inboxes via unique webhook URLs. This is the "integrate with everything" feature.
5. **Webhook push delivery** -- Agents in serverless environments get signals pushed to their callback URL.
6. **Flutter mobile app foundation** -- Registration, inbox view, push notification setup. This is the foundation for Phase 2's human-in-the-loop approval flows.
7. **Polling fallback and health endpoint** -- Safety nets and operational basics.

Defer:
- **Client SDKs**: Ship after API stabilizes. Provide curl examples and OpenAPI spec instead. (Caveat: if the goal is developer adoption speed, a minimal Python SDK with `register()`, `send()`, `listen()` could accelerate testing.)
- **Landing page**: README with quickstart is sufficient for Phase 1.
- **All Phase 2+ features**: Delegation, claims, attestations, human entities, encryption, templates, orgs, federation.

## Competitive Landscape Context

The agent identity space is heating up fast:

- **Microsoft Entra Agent ID** (preview, 2026): Enterprise-grade but proprietary, Azure-locked. Focuses on governance, conditional access, lifecycle management. No open protocol, no self-sovereign identity.
- **A2A Protocol** (Google, v0.3 July 2025): Agent-to-agent communication with Agent Cards for capability discovery. JWT/OIDC auth. Does NOT define persistent identity or trust delegation -- it assumes these exist.
- **AgentMail** ($6M seed, March 2026): Email-inbox-for-agents. Two-way email, threading, parsing. Pure communication, no identity or trust layer.
- **Indicio ProvenAI** (commercial): W3C Verifiable Credentials for AI agents. DID-based. Enterprise-focused (travel, finance). More mature trust model but complex setup, not developer-friendly.
- **MCP** (Anthropic, Linux Foundation): Tool protocol, not identity protocol. November 2025 spec added auth concerns. Needs a trust layer underneath -- exactly where ATAP positions itself.

ATAP's unique positioning: the open, vendor-neutral identity and trust layer that sits below MCP, A2A, and AP2. No competing open protocol fills this exact niche. The "doorbell" (inbox + signal delivery) is the wedge; the trust chain is the moat.

## Sources

- [NIST AI Agent Standards Initiative](https://www.nist.gov/news-events/news/2026/02/announcing-ai-agent-standards-initiative-interoperable-and-secure)
- [Microsoft Entra Agent ID](https://learn.microsoft.com/en-us/entra/agent-id/identity-platform/what-is-agent-id)
- [A2A Protocol](https://a2a-protocol.org/latest/)
- [AgentMail ($6M raise, March 2026)](https://techcrunch.com/2026/03/10/agentmail-raises-6m-to-build-an-email-service-for-ai-agents/)
- [Indicio ProvenAI](https://indicio.tech/blog/indicio-announces-provenai-a-privacy-preserving-identity-infrastructure-for-ai-agents/)
- [Mastercard Verifiable Intent](https://www.mastercard.com/us/en/news-and-trends/stories/2026/verifiable-intent.html)
- [AI Agents with DIDs and VCs (arxiv)](https://arxiv.org/html/2511.02841v1)
- [Strata: AI Agent Identity Playbook](https://www.strata.io/blog/agentic-identity/new-identity-playbook-ai-agents-not-nhi-8b/)
- [MCP Auth Guide (Permit.io)](https://www.permit.io/blog/the-ultimate-guide-to-mcp-auth)
- [Identity Foundation: Building the Agentic Economy](https://blog.identity.foundation/building-the-agentic-economy/)
