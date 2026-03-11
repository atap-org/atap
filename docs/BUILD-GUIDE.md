# ATAP — From Zero to Running Platform

## Claude Code Master Document

**Version:** 0.1  
**Last Updated:** March 2026  
**Purpose:** Complete build guide for the ATAP platform. Everything Claude Code needs to go from empty repo to production.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Repository Structure](#2-repository-structure)
3. [Technology Stack](#3-technology-stack)
4. [Phase 1: The Doorbell (Weeks 1–4)](#4-phase-1-the-doorbell)
5. [Phase 2: The Trust Chain (Weeks 5–8)](#5-phase-2-the-trust-chain)
6. [Phase 3: The Marketplace (Weeks 9–12)](#6-phase-3-the-marketplace)
7. [Phase 4: The Standard (Months 4–6)](#7-phase-4-the-standard)
8. [Database Schema](#8-database-schema)
9. [API Specification](#9-api-specification)
10. [Signal Format](#10-signal-format)
11. [Delegation Documents](#11-delegation-documents)
12. [SSE Delivery System](#12-sse-delivery-system)
13. [Webhook Delivery System](#13-webhook-delivery-system)
14. [Authentication & Authorization](#14-authentication--authorization)
15. [Mobile App Specification](#15-mobile-app-specification)
16. [Branded Approval Templates](#16-branded-approval-templates)
17. [Federation & Key Discovery](#17-federation--key-discovery)
18. [World ID Integration](#18-world-id-integration)
19. [SIMRelay Integration](#19-simrelay-integration)
20. [Infrastructure & Deployment](#20-infrastructure--deployment)
21. [Testing Strategy](#21-testing-strategy)
22. [SDK Specifications](#22-sdk-specifications)

---

## 1. Project Overview

### What We Are Building

**ATAP** (Agent Trust and Authority Protocol) is an open protocol for verifiable delegation of trust between AI agents, machines, humans, and organizations.

**atap.dev** — Open source protocol spec, SDKs, JSON schemas, documentation.  
**atap.app** — Hosted platform: registration, inboxes, signal delivery, branded approvals, mobile app, trust graph.

### Core Problem

AI agents have no persistent identity, no way to receive notifications, and no verifiable link to the humans they act for. The current protocol stack (MCP, A2A, AP2) assumes someone built the identity and trust layer. Nobody did.

### Strategic Approach

1. Solve the notification problem first (agents need a doorbell).
2. Add identity and trust as natural next steps.
3. Let the protocol emerge from the platform adoption.

### Entity Model

```
agent://   — Ephemeral, goal-driven AI actors (ID assigned by registry)
machine:// — Persistent applications and services (ID assigned by registry)
human://   — Trust anchors, participate in signal protocol via mobile app
             (ID derived from Ed25519 public key — self-sovereign)
org://     — Organizational umbrellas
```

### Human Identity

Human IDs are derived from their public key, not from email or phone:

```
human_id = lowercase(base32_encode(sha256(ed25519_public_key))[:16])
```

Email and phone are **attestations** (verified properties that raise trust level), not identifiers. A human can change their email without breaking their delegation chains.

### Trust Levels

```
Level 0: Anonymous agent (self-registration, no human)
Level 1: Human-claimed (email + phone via SIMRelay reverse SMS)
Level 2: Proof-of-personhood (World ID, ZK proof)
Level 3: Verified identity (eID + org verification)
```

---

## 2. Repository Structure

```
atap/
├── platform/                    # Go backend (atap.app)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go          # Entry point
│   ├── internal/
│   │   ├── api/                 # HTTP handlers (Fiber)
│   │   │   ├── entities.go      # Entity registration
│   │   │   ├── inbox.go         # Inbox send/receive/stream
│   │   │   ├── channels.go      # Channel management
│   │   │   ├── claims.go        # Claim flow
│   │   │   ├── delegations.go   # Delegation CRUD + verify
│   │   │   ├── templates.go     # Branded approval templates
│   │   │   ├── auth.go          # Auth middleware
│   │   │   └── health.go        # Health check
│   │   ├── models/              # Data models
│   │   │   ├── entity.go
│   │   │   ├── signal.go
│   │   │   ├── delegation.go
│   │   │   ├── channel.go
│   │   │   ├── claim.go
│   │   │   └── template.go
│   │   ├── store/               # Database layer
│   │   │   ├── postgres.go      # PostgreSQL implementation
│   │   │   ├── entities.go
│   │   │   ├── signals.go
│   │   │   ├── delegations.go
│   │   │   ├── channels.go
│   │   │   ├── claims.go
│   │   │   └── templates.go
│   │   ├── delivery/            # Signal delivery
│   │   │   ├── sse.go           # SSE streaming
│   │   │   ├── webhook.go       # Webhook push
│   │   │   ├── push.go          # Mobile push (FCM/APNs)
│   │   │   └── manager.go       # Delivery orchestration
│   │   ├── crypto/              # Cryptographic operations
│   │   │   ├── ed25519.go       # Signing/verification
│   │   │   ├── x25519.go        # Encryption
│   │   │   ├── keys.go          # Key generation/management
│   │   │   └── canonical.go     # Canonical JSON serialization
│   │   ├── verify/              # Delegation verification
│   │   │   ├── chain.go         # Chain verification logic
│   │   │   ├── revocation.go    # Revocation checking
│   │   │   └── scope.go         # Scope validation
│   │   ├── federation/          # Key discovery
│   │   │   ├── dns.go           # DNS TXT lookup
│   │   │   ├── wellknown.go     # .well-known endpoint
│   │   │   └── registry.go      # Registry lookup
│   │   └── integrations/        # External integrations
│   │       ├── worldid.go       # World ID verification
│   │       ├── simrelay.go      # SIMRelay reverse SMS
│   │       └── firebase.go      # FCM push notifications
│   ├── migrations/              # SQL migrations
│   │   ├── 001_entities.sql
│   │   ├── 002_signals.sql
│   │   ├── 003_channels.sql
│   │   ├── 004_delegations.sql
│   │   ├── 005_claims.sql
│   │   └── 006_templates.sql
│   ├── config/
│   │   └── config.go            # Configuration
│   ├── Dockerfile
│   ├── docker-compose.yml
│   ├── go.mod
│   └── go.sum
│
├── mobile/                      # Flutter mobile app (atap.app)
│   ├── lib/
│   │   ├── main.dart
│   │   ├── app.dart
│   │   ├── models/              # Data models
│   │   │   ├── entity.dart
│   │   │   ├── signal.dart
│   │   │   ├── delegation.dart
│   │   │   ├── claim.dart
│   │   │   └── template.dart
│   │   ├── services/            # Business logic
│   │   │   ├── api_service.dart
│   │   │   ├── auth_service.dart
│   │   │   ├── crypto_service.dart
│   │   │   ├── push_service.dart
│   │   │   ├── sse_service.dart
│   │   │   └── storage_service.dart
│   │   ├── screens/             # UI screens
│   │   │   ├── onboarding/
│   │   │   │   ├── welcome_screen.dart
│   │   │   │   ├── email_verify_screen.dart
│   │   │   │   ├── phone_verify_screen.dart
│   │   │   │   └── worldid_screen.dart
│   │   │   ├── inbox/
│   │   │   │   ├── inbox_screen.dart
│   │   │   │   └── signal_detail_screen.dart
│   │   │   ├── approval/
│   │   │   │   ├── approval_screen.dart
│   │   │   │   └── approval_template_renderer.dart
│   │   │   ├── entities/
│   │   │   │   ├── entities_screen.dart
│   │   │   │   └── entity_detail_screen.dart
│   │   │   └── settings/
│   │   │       └── settings_screen.dart
│   │   ├── widgets/             # Reusable widgets
│   │   │   ├── signal_card.dart
│   │   │   ├── approval_card.dart
│   │   │   ├── entity_tile.dart
│   │   │   ├── trust_badge.dart
│   │   │   └── branded_container.dart
│   │   └── theme/
│   │       └── atap_theme.dart
│   ├── android/
│   ├── ios/
│   ├── pubspec.yaml
│   └── README.md
│
├── sdks/                        # Client SDKs
│   ├── python/
│   │   ├── atap/
│   │   │   ├── __init__.py
│   │   │   ├── client.py        # Main client
│   │   │   ├── models.py        # Data models
│   │   │   ├── crypto.py        # Ed25519/X25519
│   │   │   ├── inbox.py         # Inbox operations
│   │   │   ├── sse.py           # SSE listener
│   │   │   └── mcp.py           # MCP tool server
│   │   ├── setup.py
│   │   ├── pyproject.toml
│   │   └── README.md
│   ├── javascript/
│   │   ├── src/
│   │   │   ├── index.ts         # Main client
│   │   │   ├── models.ts        # Data models
│   │   │   ├── crypto.ts        # Ed25519/X25519
│   │   │   ├── inbox.ts         # Inbox operations
│   │   │   ├── sse.ts           # SSE listener
│   │   │   └── mcp.ts           # MCP tool server
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   └── README.md
│   └── go/
│       ├── atap.go              # Main client
│       ├── models.go
│       ├── crypto.go
│       ├── inbox.go
│       ├── sse.go
│       ├── go.mod
│       └── README.md
│
├── spec/                        # Protocol specification
│   ├── ATAP-SPEC-v0.1.md        # Full RFC-style spec
│   ├── schemas/                 # JSON schemas
│   │   ├── signal.json
│   │   ├── delegation.json
│   │   ├── entity-record.json
│   │   ├── revocation-list.json
│   │   ├── approval-template.json
│   │   └── claim-request.json
│   └── examples/                # Example payloads
│       ├── signal-basic.json
│       ├── signal-encrypted.json
│       ├── delegation-full.json
│       ├── claim-request.json
│       └── approval-template.json
│
├── web/                         # atap.dev website
│   ├── src/
│   ├── public/
│   └── package.json
│
├── docs/                        # Documentation
│   ├── quickstart.md
│   ├── concepts.md
│   ├── api-reference.md
│   ├── mobile-app.md
│   ├── federation.md
│   └── migration.md
│
├── scripts/                     # Build & deploy scripts
│   ├── setup.sh
│   ├── migrate.sh
│   └── deploy.sh
│
├── .github/
│   └── workflows/
│       ├── platform.yml
│       ├── mobile.yml
│       └── sdks.yml
│
├── LICENSE                      # Apache 2.0
├── README.md
└── CLAUDE.md                    # Claude Code instructions
```

---

## 3. Technology Stack

### Platform (atap.app)

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | Go 1.22+ | Single binary, fast, known stack |
| HTTP framework | Fiber v2 | Fast, Express-like API, great SSE support |
| Database | PostgreSQL 16 | JSONB for flexible payloads, proven at scale |
| Real-time pub/sub | Redis 7 | Pub/sub for SSE fan-out, caching |
| Cryptography | golang.org/x/crypto | Ed25519, X25519, NaCl |
| Migration | golang-migrate | SQL migrations |
| Config | envconfig | Environment-based config |
| Logging | zerolog | Structured JSON logging |
| Mobile push | Firebase Admin SDK (Go) | FCM for Android, APNs for iOS |

### Mobile App (atap.app)

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Framework | Flutter 3.x | Single codebase iOS + Android |
| State management | Riverpod | Reactive, testable |
| HTTP client | dio | Interceptors, retry |
| SSE client | eventsource | Dart SSE library |
| Push notifications | firebase_messaging | FCM + APNs |
| Biometric auth | local_auth | Face ID, fingerprint |
| Secure storage | flutter_secure_storage | Keychain / Keystore |
| Cryptography | cryptography_flutter | Ed25519, X25519 |
| World ID | Custom WebView integration | IDKit v4 |
| Deep linking | go_router | Claim URLs, universal links |
| QR code scanning | mobile_scanner | Claim code scanning |

### SDKs

| SDK | Language | Package Manager |
|-----|----------|----------------|
| Python | Python 3.9+ | pip (PyPI) |
| JavaScript | TypeScript | npm |
| Go | Go 1.22+ | Go modules |

### Infrastructure

| Component | Technology |
|-----------|-----------|
| Hosting | Hetzner Cloud (shared with SIMRelay) |
| Deployment | Coolify (Docker-based) |
| Reverse proxy | Caddy (automatic HTTPS) |
| VPN | WireGuard (internal services) |
| Monitoring | Grafana + Loki + Prometheus |
| CI/CD | GitHub Actions |

---

## 4. Phase 1: The Doorbell

**Duration:** Weeks 1–4  
**Goal:** An agent can register, get an inbox, and receive signals. A human can download the mobile app and receive push notifications.

### Week 1: Foundation

#### Day 1–2: Project Setup & Core API

```bash
# Initialize Go project
mkdir -p platform/cmd/server platform/internal/{api,models,store,delivery,crypto,config}
cd platform && go mod init github.com/atap-dev/atap
go get github.com/gofiber/fiber/v2
go get github.com/jackc/pgx/v5
go get github.com/redis/go-redis/v9
go get golang.org/x/crypto
go get github.com/rs/zerolog
```

**Deliverables:**
- Go project scaffold
- PostgreSQL schema: `entities`, `signals` tables
- `POST /v1/register` — register agent, returns entity URI + keypair
- `POST /v1/inbox/{entity-id}` — send signal to inbox
- `GET /v1/inbox/{entity-id}` — poll inbox
- Docker Compose with Go, PostgreSQL, Redis
- Health check endpoint

#### Day 3–4: Signal Delivery

**Deliverables:**
- `GET /v1/inbox/{entity-id}/stream` — SSE endpoint
- Redis pub/sub integration for real-time delivery
- Last-Event-ID reconnection support
- Webhook URL generation per entity
- Channel management: `POST /v1/entities/{id}/channels`
- Inbound webhook handler: `POST /v1/channels/{channel-id}/signals`

#### Day 5: Signal Payload

**Deliverables:**
- Signal JSON schema v0.1 (route + signal + context blocks)
- Canonical JSON serialization for signing
- Signal validation middleware
- Published JSON schema to spec/ directory

#### Day 6–7: Python SDK + MCP Tool

**Deliverables:**
- Python SDK: `pip install atap`
  - `atap.register()` → agent entity
  - `atap.send(target, data)` → send signal
  - `atap.listen()` → SSE listener (async generator)
  - `atap.poll()` → poll inbox
- MCP tool server: `check_inbox` tool
- README with 5-line quickstart example

### Week 2: Mobile App Foundation

#### Day 8–10: Flutter Project + Onboarding

```bash
flutter create --org dev.atep --project-name atap_app mobile
cd mobile
flutter pub add dio riverpod go_router firebase_messaging local_auth flutter_secure_storage
```

**Deliverables:**
- Flutter project scaffold with Riverpod
- ATAP theme (colors, typography, spacing)
- Onboarding flow: Welcome → Email input → Email verification → Done
- Secure storage for entity keypair and auth token
- API service connecting to platform backend

#### Day 11–12: Push Notifications

**Deliverables:**
- Firebase project setup (FCM)
- APNs configuration for iOS
- Push notification registration on app launch
- Platform stores device push tokens per entity
- Signals trigger push notifications to human entities
- Notification payload includes signal preview

#### Day 13–14: Inbox Screen

**Deliverables:**
- Inbox screen: chronological list of signals
- Signal card widget (sender, timestamp, preview)
- Signal detail screen (full signal data)
- Pull-to-refresh
- Real-time updates via SSE (background service)
- Badge count on app icon

### Week 3: JavaScript SDK + Landing Page

**Deliverables:**
- JavaScript/TypeScript SDK: `npm install atap`
  - Same API surface as Python SDK
  - Browser + Node.js compatible SSE
  - MCP tool definition for JS agent frameworks
- Landing page (atap.dev)
  - Hero: "Give your agent an identity in 30 seconds"
  - Live code example showing register → send → receive
  - Protocol overview with stack diagram
  - Link to GitHub, docs, SDKs
  - Built with Astro or plain HTML

### Week 4: Polish & Launch

**Deliverables:**
- Go SDK: `go get github.com/atap-dev/atap-go`
- End-to-end integration tests
- Rate limiting on all endpoints
- Error handling standardized (RFC 7807 problem details)
- API documentation (OpenAPI 3.1)
- Docker production build
- Deploy to Hetzner via Coolify
- Mobile app TestFlight (iOS) + Internal Testing (Android)
- README, CONTRIBUTING.md, LICENSE

### Phase 1 Success Criteria

- [ ] Agent can register and get an inbox in <1 second
- [ ] Signal delivery via SSE in <100ms
- [ ] Webhook inbound → inbox → SSE outbound works end-to-end
- [ ] Mobile app receives push notification when signal arrives
- [ ] Python, JS, and Go SDKs published
- [ ] MCP tool works with Claude
- [ ] atap.dev landing page live
- [ ] Mobile app on TestFlight

---

## 5. Phase 2: The Trust Chain

**Duration:** Weeks 5–8  
**Goal:** Agents can be claimed by humans. Delegation chains are born. SIMRelay and World ID integration.

### Week 5: Human Entity + Attestations

**Deliverables:**
- Human entity registration (via mobile app)
  - Keypair generated in secure enclave
  - Human ID derived from public key hash
  - Entity created at Level 0 immediately (no attestations required)
  - Encrypted key backup with Argon2id (passphrase-protected)
- Phone attestation via SIMRelay reverse SMS
  - App generates unique SMS body
  - User sends SMS to SIMRelay number
  - SIMRelay webhook confirms → phone attestation added
- Email attestation flow
- Attestations stored as JSONB, separate from identity
- Trust level recalculation on attestation change
- Trust level 1 requires email + phone attestations

### Week 6: Claim Flow

**Deliverables:**
- `POST /v1/claims` — agent requests claim, gets claim URL + code
- Claim URL handler: `GET /v1/claims/{claim-id}`
- Mobile app: deep link handling for claim URLs
- Mobile app: claim approval screen
  - Shows agent details, requested scopes
  - Biometric authentication (Face ID / fingerprint)
  - Approve / Decline buttons
- On approval: delegation document minted + signed
- Confirmation signal sent to agent's inbox
- QR code generation for claim codes

### Week 7: Delegation System

**Deliverables:**
- Delegation document schema (see Section 11)
- Ed25519 signing of delegation documents
- `POST /v1/delegations` — create delegation
- `GET /v1/delegations/{id}` — retrieve delegation
- `POST /v1/delegations/{id}/revoke` — revoke delegation
- `POST /v1/verify` — verify delegation chain
- Cascading revocation (revoke machine → invalidate child delegations)
- Trust block added to signal format
- Signal signature verification middleware

### Week 8: World ID + Machine Registration

**Deliverables:**
- World ID integration via IDKit v4 (mobile WebView)
- On World ID verification: World ID attestation added, trust level → 2
- Machine entity registration by humans
  - `POST /v1/machines` — human registers a machine
  - Delegation chain: human:// → machine://
- Machine keypair generation
- Machine can now spawn/approve agents
- Delegation chains: human:// → machine:// → agent://
- Signed signals between all entity types
- Human-to-agent signal sending from mobile app (signed with human's key)
- Agent can verify instructions came from its principal
- GDPR attestation deletion endpoint: `DELETE /v1/attestations/{type}`

### Phase 2 Success Criteria

- [ ] Human can create account via mobile app (email + phone + optional World ID)
- [ ] Agent can request claim, human approves via push notification
- [ ] Delegation document is minted and cryptographically verifiable
- [ ] `POST /v1/verify` validates full chain offline (with cached keys)
- [ ] Revocation cascades correctly
- [ ] All signals carry Ed25519 signatures
- [ ] Human can send signed signals to agents via mobile app

---

## 6. Phase 3: The Marketplace

**Duration:** Weeks 9–12  
**Goal:** Machines publish branded approval templates. Org entities. Encryption. Monetization.

### Week 9: Branded Approval Templates

**Deliverables:**
- Template schema (see Section 16)
- `POST /v1/templates` — machine uploads template
- `GET /v1/templates/{id}` — retrieve template
- Template validation (schema compliance, brand asset URLs)
- Domain verification for template brands (DNS TXT check)
- Mobile app: branded approval renderer
  - Loads machine's colors, logo
  - Renders display fields from signal data
  - Verified domain badge

### Week 10: Org Entities + Full Entity Model

**Deliverables:**
- Org entity registration: `POST /v1/orgs`
- Domain verification for orgs
- Full 4-entity delegation chains: org → human → machine → agent
- Org-level revocation
- Entity management screen in mobile app
  - List all claimed agents, registered machines
  - Revoke any entity
  - View delegation chains visually

### Week 11: End-to-End Encryption

**Deliverables:**
- X25519 key generation and storage
- Encrypted signal creation (sender encrypts with recipient's X25519 public key)
- Encrypted signal reception (recipient decrypts)
- Platform zero-knowledge relay (cannot read encrypted signal bodies)
- Key exchange protocol for new entity pairs
- Mobile app: encrypted signal handling

### Week 12: Monetization + Polish

**Deliverables:**
- Tier system:
  - Free: 1 human, 3 agents, 1 machine, generic approvals, 1000 signals/month
  - Pro: 5 humans, 25 agents, 10 machines, branded templates, 50k signals/month
  - Enterprise: unlimited, API, analytics, SLA
- Stripe integration for Pro tier
- Approval analytics dashboard for machines
  - Approval count, conversion rate, avg time to approve
- Rate limiting per tier
- Usage tracking and billing

---

## 7. Phase 4: The Standard

**Duration:** Months 4–6  
**Goal:** ATAP becomes a recognized open standard. Federation. Ecosystem integrations.

**Deliverables:**
- Federation: DNS TXT key discovery
- Federation: `.well-known/atap.json` endpoint support
- Revocation transparency log (append-only, signed)
- ATAP spec v1.0 publication
- A2A integration guide (ATAP delegation in Agent Cards)
- AP2 integration guide (ATAP delegation as payment mandate)
- eID verification for Level 3 trust
- Third-party template renderer specification
- Trust graph explorer (public web UI)
- Community governance for spec contributions

---

## 8. Database Schema

### Migration 001: Entities

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE entity_type AS ENUM ('agent', 'machine', 'human', 'org');
CREATE TYPE trust_level AS ENUM ('0', '1', '2', '3');
CREATE TYPE delivery_preference AS ENUM ('sse', 'webhook', 'poll');

CREATE TABLE entities (
    id              TEXT PRIMARY KEY,  -- e.g., "a1b2c3d4e5f6"
    type            entity_type NOT NULL,
    uri             TEXT UNIQUE NOT NULL GENERATED ALWAYS AS (type || '://' || id) STORED,

    -- Cryptographic identity
    public_key_ed25519  BYTEA NOT NULL,
    public_key_x25519   BYTEA,  -- optional, for encryption
    key_id              TEXT NOT NULL,

    -- Metadata
    name            TEXT,
    description     TEXT,
    trust_level     trust_level NOT NULL DEFAULT '0',

    -- Delivery
    delivery_pref   delivery_preference NOT NULL DEFAULT 'sse',
    webhook_url     TEXT,  -- for webhook delivery to entity
    push_token      TEXT,  -- FCM/APNs token for mobile push
    push_platform   TEXT,  -- 'fcm' or 'apns'

    -- Ownership (for machines and agents)
    owner_id        TEXT REFERENCES entities(id),
    org_id          TEXT REFERENCES entities(id),

    -- Attestations (verified properties, NOT the identity)
    -- Email, phone, World ID, eID are attestations that raise trust level
    -- They can be added/removed without changing the entity's identity
    attestations    JSONB NOT NULL DEFAULT '{}',
    -- Example:
    -- {
    --   "email": {"address": "sven@simrelay.com", "verified_at": "..."},
    --   "phone": {"number": "+49...", "verified_at": "...", "method": "reverse_sms"},
    --   "world_id": {"proof_type": "orb", "uniqueness": "verified", "verified_at": "..."},
    --   "eid": {"country": "DE", "verified_at": "..."}
    -- }

    -- Key recovery (for humans)
    recovery_backup JSONB,  -- encrypted private key backup
    -- {
    --   "kdf": "argon2id",
    --   "kdf_params": {"memory": 65536, "iterations": 3, "parallelism": 4},
    --   "encrypted_private_key": "base64...",
    --   "nonce": "base64...",
    --   "created_at": "..."
    -- }

    -- Registry
    registry        TEXT NOT NULL DEFAULT 'atap.app',
    revocation_url  TEXT,

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ  -- soft delete
);

CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_owner ON entities(owner_id);
CREATE INDEX idx_entities_org ON entities(org_id);
CREATE INDEX idx_entities_uri ON entities(uri);
```

### Migration 002: Signals

```sql
CREATE TABLE signals (
    id              TEXT PRIMARY KEY,  -- e.g., "sig_01HQ3K9X8W"
    version         TEXT NOT NULL DEFAULT '1',

    -- Route
    origin          TEXT NOT NULL,  -- sender entity URI
    target          TEXT NOT NULL,  -- recipient entity URI
    reply_to        TEXT,
    channel_id      TEXT,
    thread_id       TEXT,
    ref_id          TEXT,  -- references another signal

    -- Trust
    signature       BYTEA,
    signer_key_id   TEXT,
    delegation_id   TEXT,
    encrypted       BOOLEAN NOT NULL DEFAULT FALSE,

    -- Signal body
    content_type    TEXT NOT NULL DEFAULT 'application/json',
    data            JSONB,  -- plaintext payload
    data_encrypted  BYTEA,  -- encrypted payload (if encrypted=true)

    -- Context
    source_type     TEXT,  -- 'webhook', 'agent', 'machine', 'system'
    idempotency_key TEXT,
    tags            TEXT[],
    ttl             INTEGER,  -- seconds, NULL = no expiry
    priority        INTEGER NOT NULL DEFAULT 1,

    -- Delivery tracking
    delivered       BOOLEAN NOT NULL DEFAULT FALSE,
    delivered_at    TIMESTAMPTZ,
    delivery_method TEXT,  -- 'sse', 'webhook', 'poll', 'push'

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ  -- computed from created_at + ttl

);

CREATE INDEX idx_signals_target ON signals(target);
CREATE INDEX idx_signals_target_created ON signals(target, created_at DESC);
CREATE INDEX idx_signals_thread ON signals(thread_id);
CREATE INDEX idx_signals_idempotency ON signals(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_signals_expires ON signals(expires_at) WHERE expires_at IS NOT NULL;

-- Idempotency: prevent duplicate signals
CREATE UNIQUE INDEX idx_signals_idempotency_unique
    ON signals(idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

### Migration 003: Channels

```sql
CREATE TABLE channels (
    id              TEXT PRIMARY KEY,  -- e.g., "chn_8f3a9b2c"
    entity_id       TEXT NOT NULL REFERENCES entities(id),
    webhook_url     TEXT UNIQUE NOT NULL,  -- the inbound URL

    label           TEXT,
    tags            TEXT[],

    -- Limits
    expires_at      TIMESTAMPTZ,
    rate_limit      INTEGER,  -- signals per minute, NULL = no limit

    -- Status
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    revoked_at      TIMESTAMPTZ,

    -- Stats
    signal_count    BIGINT NOT NULL DEFAULT 0,
    last_signal_at  TIMESTAMPTZ,

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_entity ON channels(entity_id);
CREATE INDEX idx_channels_webhook ON channels(webhook_url);
```

### Migration 004: Delegations

```sql
CREATE TYPE delegation_status AS ENUM ('active', 'revoked', 'expired');

CREATE TABLE delegations (
    id              TEXT PRIMARY KEY,  -- e.g., "del_7f8a9b"
    version         TEXT NOT NULL DEFAULT '1',

    -- Chain
    principal_id    TEXT NOT NULL REFERENCES entities(id),
    delegate_id     TEXT NOT NULL REFERENCES entities(id),
    via_ids         TEXT[],  -- ordered array of intermediate entity IDs

    -- Scope
    actions         TEXT[] NOT NULL,
    spend_limit     JSONB,  -- {amount, currency, period}
    data_classes    TEXT[],
    expires_at      TIMESTAMPTZ NOT NULL,

    -- Constraints
    constraints     JSONB,  -- {geo, time_window, confirm_above, ...}

    -- Human verification snapshot
    human_verification JSONB,

    -- Signatures
    signatures      JSONB NOT NULL,  -- array of {entity, key_id, sig, signed_at}

    -- Status
    status          delegation_status NOT NULL DEFAULT 'active',
    revoked_at      TIMESTAMPTZ,
    revoked_by      TEXT,
    revoke_reason   TEXT,

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delegations_principal ON delegations(principal_id);
CREATE INDEX idx_delegations_delegate ON delegations(delegate_id);
CREATE INDEX idx_delegations_status ON delegations(status);
CREATE INDEX idx_delegations_via ON delegations USING GIN(via_ids);
```

### Migration 005: Claims

```sql
CREATE TYPE claim_status AS ENUM ('pending', 'approved', 'declined', 'expired');

CREATE TABLE claims (
    id              TEXT PRIMARY KEY,  -- e.g., "clm_xyz789"
    agent_id        TEXT NOT NULL REFERENCES entities(id),
    claim_code      TEXT UNIQUE NOT NULL,  -- e.g., "ATAP-7X9K-2M4P"

    -- Request
    requested_scopes TEXT[],
    machine_id      TEXT REFERENCES entities(id),
    description     TEXT,
    context         JSONB,

    -- Resolution
    status          claim_status NOT NULL DEFAULT 'pending',
    approved_by     TEXT REFERENCES entities(id),  -- human who approved
    delegation_id   TEXT REFERENCES delegations(id),  -- resulting delegation
    approved_scopes TEXT[],  -- may differ from requested

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_claims_agent ON claims(agent_id);
CREATE INDEX idx_claims_status ON claims(status);
CREATE INDEX idx_claims_code ON claims(claim_code);
```

### Migration 006: Templates

```sql
CREATE TABLE approval_templates (
    id              TEXT PRIMARY KEY,  -- e.g., "tpl_flight_booking"
    machine_id      TEXT NOT NULL REFERENCES entities(id),
    version         INTEGER NOT NULL DEFAULT 1,

    -- Brand
    brand_name      TEXT NOT NULL,
    brand_logo_url  TEXT,
    brand_colors    JSONB,  -- {primary, accent, background}
    verified_domain TEXT,
    domain_verified BOOLEAN NOT NULL DEFAULT FALSE,

    -- Schema
    approval_type   TEXT NOT NULL,  -- e.g., "flight_booking"
    title           TEXT NOT NULL,
    description     TEXT,
    display_fields  JSONB NOT NULL,  -- array of {key, label, type, format}
    required_trust_level trust_level NOT NULL DEFAULT '1',
    scopes_requested TEXT[],

    -- Status
    active          BOOLEAN NOT NULL DEFAULT TRUE,

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_machine ON approval_templates(machine_id);
CREATE UNIQUE INDEX idx_templates_machine_type ON approval_templates(machine_id, approval_type);
```

### Housekeeping

```sql
-- Periodic cleanup of expired signals
-- Run via pg_cron or application-level scheduler
-- DELETE FROM signals WHERE expires_at IS NOT NULL AND expires_at < NOW();

-- Periodic cleanup of expired claims
-- DELETE FROM claims WHERE status = 'pending' AND expires_at < NOW();

-- Update expired delegations
-- UPDATE delegations SET status = 'expired' WHERE status = 'active' AND expires_at < NOW();
```

---

## 9. API Specification

### Base URL

```
Production: https://api.atap.app/v1
```

### Authentication

All authenticated endpoints use Bearer tokens:

```
Authorization: Bearer {entity-token}
```

Entity tokens are issued at registration and can be rotated via `POST /v1/auth/rotate`.

### Endpoints Overview

#### Entities

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/register | Register new agent (self-service) | None |
| POST | /v1/humans | Register human (from mobile app) | None |
| POST | /v1/machines | Register machine | Human token |
| POST | /v1/orgs | Register organization | Human token |
| GET | /v1/entities/{id} | Get entity record | Optional |
| PATCH | /v1/entities/{id} | Update entity metadata | Entity token |
| DELETE | /v1/entities/{id} | Soft-delete entity | Entity token |

#### Inbox & Signals

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/inbox/{target-id} | Send signal to entity | Entity token |
| GET | /v1/inbox/{entity-id} | Poll inbox | Entity token |
| GET | /v1/inbox/{entity-id}/stream | SSE stream | Entity token |
| GET | /v1/signals/{signal-id} | Get specific signal | Entity token |

#### Channels

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/entities/{id}/channels | Create channel | Entity token |
| GET | /v1/entities/{id}/channels | List channels | Entity token |
| DELETE | /v1/channels/{channel-id} | Revoke channel | Entity token |
| POST | /v1/channels/{channel-id}/signals | Inbound webhook | None (validated by channel) |

#### Claims

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/claims | Request claim | Agent token |
| GET | /v1/claims/{claim-id} | Get claim details | Any token |
| POST | /v1/claims/{claim-id}/approve | Approve claim | Human token |
| POST | /v1/claims/{claim-id}/decline | Decline claim | Human token |

#### Delegations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/delegations | Create delegation | Human/Machine token |
| GET | /v1/delegations/{id} | Get delegation | Any token |
| POST | /v1/delegations/{id}/revoke | Revoke delegation | Principal/Via token |
| POST | /v1/verify | Verify delegation chain | None |

#### Templates

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/templates | Create template | Machine token |
| GET | /v1/templates/{id} | Get template | None |
| PUT | /v1/templates/{id} | Update template | Machine token |
| GET | /v1/machines/{id}/templates | List machine's templates | None |

#### Verification & Attestations

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | /v1/auth/rotate | Rotate entity token | Entity token |
| POST | /v1/attestations/email/start | Start email attestation | Human token |
| POST | /v1/attestations/email/verify | Verify email code | Human token |
| POST | /v1/attestations/phone/start | Start phone attestation (SIMRelay) | Human token |
| POST | /v1/attestations/worldid | Submit World ID proof | Human token |
| DELETE | /v1/attestations/{type} | Remove attestation (GDPR erasure) | Human token |
| POST | /v1/recovery/backup | Store encrypted key backup | Human token |
| POST | /v1/recovery/restore | Restore identity from backup | None (passphrase + attestation re-verification) |

### Error Format (RFC 7807)

```json
{
  "type": "https://atap.dev/errors/trust-level-insufficient",
  "title": "Trust Level Insufficient",
  "status": 403,
  "detail": "This endpoint requires trust level 2. Your current level is 1.",
  "instance": "/v1/inbox/agent-xyz",
  "required_level": 2,
  "current_level": 1
}
```

### Pagination

All list endpoints use cursor-based pagination:

```
GET /v1/inbox/{id}?after=sig_01HQ3&limit=50

Response:
{
  "data": [...],
  "cursor": "sig_01HQ5",
  "has_more": true
}
```

---

## 10. Signal Format

See ATAP Spec Section 8 for the full specification. Quick reference:

```json
{
  "v": "1",
  "id": "sig_01HQ3K9X8W",
  "ts": "2026-03-10T14:33:00Z",

  "route": {
    "origin": "agent://sender-id",
    "target": "agent://recipient-id",
    "reply_to": "agent://sender-id",
    "channel": "chn_8f3a9b2c",
    "thread": "thr_92fa",
    "ref": "sig_01HQ3K9X7V"
  },

  "trust": {
    "scheme": "ed25519",
    "key_id": "key_a1b2c3",
    "sig": "base64-signature",
    "delegation": "del_7f8a9b",
    "enc": {
      "scheme": "x25519-xsalsa20-poly1305",
      "ephemeral_key": "base64-key",
      "nonce": "base64-nonce"
    }
  },

  "signal": {
    "type": "application/json",
    "encrypted": false,
    "data": { ... }
  },

  "context": {
    "source": "agent",
    "idempotency": "idk_xyz",
    "tags": ["booking", "travel"],
    "ttl": 86400,
    "priority": 1
  }
}
```

### Signal ID Generation

Signal IDs use the format `sig_` + ULID (Universally Unique Lexicographically Sortable Identifier). This provides:
- Global uniqueness
- Lexicographic ordering (newest last)
- Timestamp encoded in the ID
- 128-bit randomness

Go implementation:
```go
import "github.com/oklog/ulid/v2"

func NewSignalID() string {
    return "sig_" + ulid.Make().String()
}
```

### Canonical JSON for Signatures

```go
// Canonical JSON: sorted keys, no whitespace, no trailing newline
// Used for signature computation over route + signal blocks

func CanonicalJSON(v interface{}) ([]byte, error) {
    // 1. Marshal to JSON
    // 2. Unmarshal to map[string]interface{} to normalize
    // 3. Re-marshal with sorted keys
    // Go's encoding/json sorts keys by default
    return json.Marshal(v)
}

func SignablePayload(route, signal interface{}) ([]byte, error) {
    r, _ := CanonicalJSON(route)
    s, _ := CanonicalJSON(signal)
    return append(append(r, '.'), s...), nil
}
```

---

## 11. Delegation Documents

See ATAP Spec Section 9 for the full specification. Quick reference in Section 9.2 of the spec.

### Verification Algorithm (Go pseudocode)

```go
func VerifyDelegation(del Delegation, action string, amount *Money) error {
    // 1. Check version
    if del.Version != "1" { return ErrUnsupportedVersion }

    // 2. Check expiration
    if time.Now().After(del.Scope.Expires) { return ErrExpired }

    // 3. Check all signatures present
    expectedSigners := append([]string{del.Principal}, del.Via...)
    if len(del.Signatures) < len(expectedSigners) { return ErrMissingSig }

    // 4. Verify each signature
    docBytes := canonicalWithoutSignatures(del)
    for i, sig := range del.Signatures {
        pubKey := lookupPublicKey(sig.Entity, sig.KeyID)
        if !ed25519.Verify(pubKey, docBytes, sig.Sig) {
            return ErrInvalidSig
        }
        if sig.Entity != expectedSigners[i] { return ErrWrongOrder }
    }

    // 5. Check revocation
    for _, entity := range expectedSigners {
        if isRevoked(entity, del.ID) { return ErrRevoked }
    }

    // 6. Check action scope
    if !scopeContains(del.Scope.Actions, action) { return ErrActionDenied }

    // 7. Check constraints
    if err := checkConstraints(del.Constraints, amount); err != nil {
        return err
    }

    return nil // Valid
}
```

---

## 12. SSE Delivery System

### Architecture

```
Agent connects via SSE
        │
        ▼
Fiber SSE Handler
        │
        ▼
Redis SUBSCRIBE inbox:{entity-id}
        │
        ▼
On message → write SSE event to response stream
```

### Go Implementation (Fiber)

```go
func (h *InboxHandler) Stream(c *fiber.Ctx) error {
    entityID := c.Params("entityId")
    lastEventID := c.Get("Last-Event-ID")

    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("X-Accel-Buffering", "no")  // disable nginx buffering

    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // 1. Replay missed signals (from PostgreSQL)
        if lastEventID != "" {
            missed := h.store.GetSignalsAfter(entityID, lastEventID)
            for _, sig := range missed {
                writeSSEEvent(w, sig)
            }
            w.Flush()
        }

        // 2. Subscribe to Redis for live signals
        sub := h.redis.Subscribe(ctx, "inbox:"+entityID)
        defer sub.Close()

        ch := sub.Channel()

        // 3. Heartbeat ticker (keep connection alive)
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case msg := <-ch:
                writeSSEEvent(w, msg.Payload)
                w.Flush()
            case <-ticker.C:
                fmt.Fprintf(w, ": heartbeat\n\n")
                w.Flush()
            case <-c.Context().Done():
                return
            }
        }
    })

    return nil
}

func writeSSEEvent(w *bufio.Writer, signalJSON string) {
    var sig Signal
    json.Unmarshal([]byte(signalJSON), &sig)
    fmt.Fprintf(w, "event: signal\n")
    fmt.Fprintf(w, "id: %s\n", sig.ID)
    fmt.Fprintf(w, "data: %s\n\n", signalJSON)
}
```

### Signal Routing

When a signal is sent to an entity:

```go
func (h *InboxHandler) Send(c *fiber.Ctx) error {
    targetID := c.Params("targetId")
    var signal Signal
    c.BodyParser(&signal)

    // 1. Validate signal format
    // 2. Verify signature (if trust block present)
    // 3. Store in PostgreSQL
    h.store.SaveSignal(signal)

    // 4. Publish to Redis for real-time SSE delivery
    h.redis.Publish(ctx, "inbox:"+targetID, signalJSON)

    // 5. If entity has push token, send push notification
    entity := h.store.GetEntity(targetID)
    if entity.PushToken != "" {
        h.push.Send(entity, signal)  // FCM/APNs
    }

    // 6. If entity prefers webhook, queue webhook delivery
    if entity.DeliveryPref == "webhook" && entity.WebhookURL != "" {
        h.webhook.Queue(entity.WebhookURL, signal)
    }

    return c.Status(202).JSON(fiber.Map{"id": signal.ID, "status": "accepted"})
}
```

---

## 13. Webhook Delivery System

### Outbound Webhooks (Platform → Entity)

For entities that prefer webhook delivery or as supplementary to SSE:

```go
type WebhookDelivery struct {
    URL       string
    Signal    Signal
    Attempt   int
    NextRetry time.Time
}

// Retry schedule: 1s, 5s, 30s, 5m, 30m
var retryDelays = []time.Duration{
    1 * time.Second,
    5 * time.Second,
    30 * time.Second,
    5 * time.Minute,
    30 * time.Minute,
}

func (w *WebhookManager) Deliver(url string, signal Signal) {
    body, _ := json.Marshal(signal)
    sig := ed25519.Sign(w.platformKey, body)

    req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-ATAP-Signature", base64.StdEncoding.EncodeToString(sig))
    req.Header.Set("X-ATAP-Signal-ID", signal.ID)

    resp, err := w.client.Do(req)
    if err != nil || resp.StatusCode != 200 {
        w.scheduleRetry(url, signal, 0)
    }
}
```

### Inbound Webhooks (External → Channel → Inbox)

For external services posting to an entity's channel:

```go
func (h *ChannelHandler) InboundWebhook(c *fiber.Ctx) error {
    channelID := c.Params("channelId")

    // 1. Look up channel
    channel := h.store.GetChannel(channelID)
    if channel == nil || !channel.Active { return c.Status(404) }
    if channel.ExpiresAt != nil && time.Now().After(*channel.ExpiresAt) { return c.Status(410) }

    // 2. Parse incoming payload (flexible — accept any JSON)
    var payload json.RawMessage
    c.BodyParser(&payload)

    // 3. Wrap in ATAP signal
    signal := Signal{
        ID:       NewSignalID(),
        Version:  "1",
        Route: Route{
            Origin:  "external",
            Target:  channel.EntityURI(),
            Channel: channelID,
        },
        Signal: SignalBody{
            Type: "application/json",
            Data: payload,
        },
        Context: SignalContext{
            Source: "webhook",
        },
    }

    // 4. Store and deliver
    h.store.SaveSignal(signal)
    h.redis.Publish(ctx, "inbox:"+channel.EntityID, signalJSON)

    // 5. Update channel stats
    h.store.IncrementChannelCount(channelID)

    return c.Status(202).JSON(fiber.Map{"signal_id": signal.ID})
}
```

---

## 14. Authentication & Authorization

### Entity Tokens

On registration, each entity receives a bearer token. Tokens are:
- 256-bit random, base64url encoded
- Stored hashed (SHA-256) in PostgreSQL
- Rotatable via `POST /v1/auth/rotate`
- Scoped to the entity that owns them

```go
func GenerateToken() (token string, hash []byte) {
    raw := make([]byte, 32)
    rand.Read(raw)
    token = "atap_" + base64.RawURLEncoding.EncodeToString(raw)
    h := sha256.Sum256([]byte(token))
    hash = h[:]
    return
}
```

### Auth Middleware

```go
func AuthMiddleware(store *Store) fiber.Handler {
    return func(c *fiber.Ctx) error {
        token := extractBearerToken(c)
        if token == "" { return c.Status(401).JSON(problem("unauthorized")) }

        hash := sha256Hash(token)
        entity := store.GetEntityByTokenHash(hash)
        if entity == nil { return c.Status(401).JSON(problem("invalid_token")) }

        c.Locals("entity", entity)
        return c.Next()
    }
}
```

### Authorization Rules

| Action | Who Can Do It |
|--------|--------------|
| Register agent | Anyone (no auth) |
| Register human | Anyone (no auth, via mobile app) |
| Register machine | Human (owns the machine) |
| Register org | Human |
| Send signal | Any entity (to any other entity) |
| Read own inbox | Entity owner |
| Create channel | Entity owner |
| Request claim | Agent |
| Approve claim | Human |
| Create delegation | Principal (human or machine) |
| Revoke delegation | Any entity in the chain |
| Create template | Machine owner |
| Verify delegation | Anyone (no auth) |

---

## 15. Mobile App Specification

### Overview

The ATAP mobile app is the human's ATAP client. It's an inbox that renders signals as approval cards, messages, and notifications. It's also the device where the human's private key lives (secure enclave).

### Platform Support

- iOS 15+ (iPhone)
- Android 10+ (API 29)
- Framework: Flutter 3.x

### Core Dependencies

```yaml
# pubspec.yaml
dependencies:
  flutter:
    sdk: flutter

  # State management
  flutter_riverpod: ^2.4.0
  riverpod_annotation: ^2.3.0

  # Networking
  dio: ^5.4.0
  eventsource: ^0.4.0  # SSE client

  # Navigation
  go_router: ^13.0.0

  # Push notifications
  firebase_core: ^2.24.0
  firebase_messaging: ^14.7.0

  # Security
  local_auth: ^2.1.0  # Biometric
  flutter_secure_storage: ^9.0.0
  cryptography_flutter: ^2.7.0
  pointycastle: ^3.7.0  # Ed25519

  # UI
  cached_network_image: ^3.3.0
  shimmer: ^3.0.0
  lottie: ^3.0.0
  qr_flutter: ^4.1.0
  mobile_scanner: ^4.0.0  # QR scanning

  # Deep linking
  uni_links: ^0.5.1

  # Utilities
  intl: ^0.19.0
  json_annotation: ^4.8.0
  freezed_annotation: ^2.4.0

dev_dependencies:
  build_runner: ^2.4.0
  json_serializable: ^6.7.0
  freezed: ^2.4.0
  riverpod_generator: ^2.3.0
  flutter_lints: ^3.0.0
```

### Screens & Navigation

```
/                           → Inbox (home)
/signal/:id                 → Signal detail
/approve/:claimId           → Approval screen (via deep link or push)
/entities                   → My agents & machines
/entities/:id               → Entity detail (delegation, revoke)
/settings                   → Settings
/onboarding                 → Welcome
/onboarding/email           → Email verification
/onboarding/phone           → Phone verification (SIMRelay)
/onboarding/worldid         → World ID verification
/send/:entityId             → Send signal to agent (compose)
```

### Screen Specifications

#### 1. Onboarding Flow

**Welcome Screen**
- ATAP logo + tagline: "Your agents, verified."
- "Get Started" button
- No login — keypair generated in device secure enclave
- Human ID derived from public key: `human://h7x9k2m4p3n8j5w2`
- Identity created instantly, before any attestation

**Recovery Setup Screen**
- "Protect your identity with a recovery passphrase"
- Passphrase input (minimum 12 characters)
- Confirm passphrase
- Private key encrypted with Argon2id-derived key, stored on platform
- "Skip for now" option (with warning)
- Explanation: "Your identity lives on this device. The passphrase lets you recover it if you lose your phone."

**Email Screen**
- "Add email attestation (optional, raises trust level)"
- Email input field
- Submit → API sends verification email
- 6-digit code input
- On verification: attestation added, proceed to phone

**Phone Screen (SIMRelay)**
- "Add phone attestation (required for Level 1)"
- Phone number input
- Instructions: "Send an SMS with this code to [SIMRelay number]"
- Display unique code: `ATAP-XXXX`
- Listening state: waiting for SIMRelay webhook confirmation
- On verification: attestation added, trust level → 1, proceed to World ID

**World ID Screen (Optional)**
- "Verify your humanity (raises trust to Level 2)"
- "Verify with World ID" button → opens IDKit WebView
- On verification: attestation added, trust level → 2
- "Skip for now" option

**Completion**
- "You're set up!" with trust level badge
- Entity URI displayed: `human://h7x9k2m4p3n8j5w2`
- Attestations summary (email ✓, phone ✓, World ID ✓/✗)
- "Go to Inbox" button

#### 2. Inbox Screen (Home)

**Layout:**
- Top bar: "ATAP" title + trust level badge + settings icon
- Signal list: reverse-chronological
- Each signal card shows:
  - Sender icon/name (brand logo if from machine with template)
  - Signal type/title
  - Preview of data (first line)
  - Timestamp (relative: "2m ago", "yesterday")
  - Unread indicator (blue dot)
  - Trust badge on sender
- Pull-to-refresh
- Empty state: "No signals yet. Your agents will appear here."

**Signal Card Types:**
- **Approval request**: Orange accent, "Approve" action visible
- **Confirmation**: Green accent, check icon
- **Info/notification**: Blue accent
- **From your agent**: Gray accent, agent icon
- **Encrypted**: Lock icon, "Encrypted signal"

**Real-time:**
- SSE connection maintained in background service
- New signals animate in at top
- Push notification triggers app refresh

#### 3. Signal Detail Screen

**Layout:**
- Full signal data rendered as structured card
- If from a machine with template: branded rendering
- If encrypted: decrypt button → biometric → show content
- Reply button (sends signal back to origin)
- Thread view if part of a thread
- Metadata: timestamp, channel, trust level of sender

#### 4. Approval Screen

**Layout (branded):**
- Machine logo + brand colors (from template)
- Verified domain badge (if verified)
- Title: "Flight Booking Approval" (from template)
- Agent info: who is requesting + trust level
- Display fields rendered from template schema:
  - Route: Frankfurt → Tokyo
  - Dates: June 15–22, 2026
  - Cabin: Business
  - Budget: €3,200
- Scopes being requested: booking:create, payment:execute
- Constraints summary: valid in EU, expires June 2026
- **Approve** button (green, prominent)
- **Decline** button (gray, secondary)
- Biometric auth required before approve executes
- Loading state while delegation is minted
- Success: "Approved! Your agent has been notified."

**Layout (generic, no template):**
- ATAP default branding
- Agent name + description
- Requested scopes as list
- Raw data preview (expandable JSON)
- Same approve/decline flow

#### 5. Entities Screen

**Layout:**
- Tabs: "Agents" | "Machines"
- Each entity tile shows:
  - Name
  - URI
  - Trust level badge
  - Status: active / revoked
  - Created date
- Tap → Entity detail screen

#### 6. Entity Detail Screen

**Layout:**
- Entity info: URI, type, created date
- Trust level with verification details
- Delegation chain visualization (who → via → whom)
- Scope summary
- Expiration date
- "Revoke" button (red, requires biometric confirmation)
- Signal history with this entity

#### 7. Send Signal Screen (Compose)

**Layout:**
- Recipient: pre-filled from entity detail or searchable
- Message type selector: Text / JSON / Custom
- Text input or structured data input
- "Send" button
- Signal is signed with human's Ed25519 key automatically
- Confirmation: "Signal sent to agent://xyz"

### Key Storage

```dart
class CryptoService {
  final FlutterSecureStorage _storage;

  // Generate Ed25519 keypair at onboarding and derive human ID
  Future<({String humanId, String publicKeyBase64})> generateIdentity() async {
    final algorithm = Ed25519();
    final keyPair = await algorithm.newKeyPair();
    final privateBytes = await keyPair.extractPrivateKeyBytes();
    final publicKey = await keyPair.extractPublicKey();
    final publicKeyBytes = Uint8List.fromList(publicKey.bytes);

    // Derive human ID from public key: base32(sha256(pubkey))[:16]
    final hash = sha256.convert(publicKeyBytes);
    final humanId = base32.encode(hash.bytes).toLowerCase().substring(0, 16);

    // Store private key in secure enclave
    await _storage.write(
      key: 'ed25519_private',
      value: base64Encode(privateBytes),
      aOptions: AndroidOptions(encryptedSharedPreferences: true),
      iOptions: IOSOptions(
        accessibility: KeychainAccessibility.when_unlocked_this_device_only,
      ),
    );

    // Store human ID
    await _storage.write(key: 'human_id', value: humanId);

    return (
      humanId: humanId,  // → human://h7x9k2m4p3n8j5w2
      publicKeyBase64: base64Encode(publicKeyBytes),
    );
  }

  // Create encrypted backup for key recovery
  Future<Map<String, dynamic>> createRecoveryBackup(String passphrase) async {
    final privateKeyBase64 = await _storage.read(key: 'ed25519_private');
    final privateKeyBytes = base64Decode(privateKeyBase64!);

    // Derive encryption key from passphrase using Argon2id
    final salt = generateRandomBytes(16);
    final encryptionKey = await argon2id(
      passphrase, salt,
      memory: 65536, iterations: 3, parallelism: 4, hashLength: 32,
    );

    // Encrypt private key with XSalsa20-Poly1305
    final nonce = generateRandomBytes(24);
    final encrypted = secretBox(privateKeyBytes, nonce, encryptionKey);

    return {
      'kdf': 'argon2id',
      'kdf_params': {'memory': 65536, 'iterations': 3, 'parallelism': 4},
      'salt': base64Encode(salt),
      'encrypted_private_key': base64Encode(encrypted),
      'nonce': base64Encode(nonce),
      'created_at': DateTime.now().toIso8601String(),
    };
  }

  // Sign a signal
  Future<Uint8List> sign(Uint8List data) async {
    final privateKeyBase64 = await _storage.read(key: 'ed25519_private');
    // ... reconstruct key and sign
  }
}
```

### Push Notification Handling

```dart
class PushService {
  Future<void> initialize() async {
    // Request permission
    await FirebaseMessaging.instance.requestPermission();

    // Get token and register with platform
    final token = await FirebaseMessaging.instance.getToken();
    await _apiService.updatePushToken(token);

    // Handle foreground messages
    FirebaseMessaging.onMessage.listen((message) {
      _handleSignalNotification(message);
    });

    // Handle background/terminated taps
    FirebaseMessaging.onMessageOpenedApp.listen((message) {
      _navigateToSignal(message.data['signal_id']);
    });
  }

  void _handleSignalNotification(RemoteMessage message) {
    // Parse signal preview from notification data
    // Update inbox state via Riverpod
    // Show local notification if app is in foreground
  }
}
```

### SSE Background Service

```dart
class SSEService {
  EventSource? _eventSource;
  String? _lastEventId;

  Future<void> connect(String entityId, String token) async {
    final url = 'https://api.atap.app/v1/inbox/$entityId/stream';
    final headers = {
      'Authorization': 'Bearer $token',
      if (_lastEventId != null) 'Last-Event-ID': _lastEventId!,
    };

    _eventSource = EventSource(Uri.parse(url), headers: headers);

    _eventSource!.listen((event) {
      if (event.event == 'signal') {
        _lastEventId = event.id;
        final signal = Signal.fromJson(jsonDecode(event.data!));
        _onSignalReceived(signal);
      }
    });
  }

  void _onSignalReceived(Signal signal) {
    // Update Riverpod state
    // Trigger local notification if needed
  }
}
```

### Deep Linking

```dart
// go_router configuration
final router = GoRouter(
  routes: [
    GoRoute(path: '/', builder: (_, __) => InboxScreen()),
    GoRoute(
      path: '/claim/:claimId',
      builder: (_, state) => ApprovalScreen(
        claimId: state.pathParameters['claimId']!,
      ),
    ),
    GoRoute(
      path: '/approve/:claimId',  // Universal link from claim URL
      redirect: (_, state) => '/claim/${state.pathParameters['claimId']}',
    ),
  ],
);

// Universal links configuration:
// iOS: apple-app-site-association at atap.app/.well-known/
// Android: assetlinks.json at atap.app/.well-known/
```

### App Theme

```dart
class ATAPTheme {
  // Colors
  static const primary = Color(0xFF3B82F6);    // Blue
  static const primaryDark = Color(0xFF1E40AF);
  static const success = Color(0xFF059669);     // Green
  static const warning = Color(0xFFD97706);     // Orange
  static const error = Color(0xFFDC2626);       // Red
  static const surface = Color(0xFFFFFFFF);
  static const background = Color(0xFFF8FAFC);
  static const textPrimary = Color(0xFF1A1A1A);
  static const textSecondary = Color(0xFF6B7280);

  // Typography
  static const headingLarge = TextStyle(
    fontSize: 28, fontWeight: FontWeight.w700, color: textPrimary,
  );
  static const headingMedium = TextStyle(
    fontSize: 22, fontWeight: FontWeight.w600, color: textPrimary,
  );
  static const body = TextStyle(
    fontSize: 16, fontWeight: FontWeight.w400, color: textPrimary,
  );
  static const caption = TextStyle(
    fontSize: 13, fontWeight: FontWeight.w400, color: textSecondary,
  );

  // Spacing
  static const spacingXS = 4.0;
  static const spacingS = 8.0;
  static const spacingM = 16.0;
  static const spacingL = 24.0;
  static const spacingXL = 32.0;

  // Borders
  static const radiusS = 8.0;
  static const radiusM = 12.0;
  static const radiusL = 16.0;
}
```

---

## 16. Branded Approval Templates

### Template Schema

```json
{
  "machine": "machine://lufthansa",
  "template_id": "tpl_flight_booking",
  "version": 1,

  "brand": {
    "name": "Lufthansa",
    "logo_url": "https://lufthansa.com/atap/logo.svg",
    "colors": {
      "primary": "#05164d",
      "accent": "#ffc72c",
      "background": "#ffffff"
    },
    "verified_domain": "lufthansa.com"
  },

  "approval_schema": {
    "type": "flight_booking",
    "title": "Flight Booking Approval",
    "description": "Your agent wants to book a flight",
    "display_fields": [
      { "key": "route", "label": "Route", "type": "text", "format": "{{origin}} → {{destination}}" },
      { "key": "depart_date", "label": "Departure", "type": "date" },
      { "key": "return_date", "label": "Return", "type": "date" },
      { "key": "passengers", "label": "Passengers", "type": "list" },
      { "key": "cabin", "label": "Cabin", "type": "text" },
      { "key": "price", "label": "Total Price", "type": "currency" }
    ],
    "required_trust_level": 2,
    "scopes_requested": ["booking:create", "payment:execute"]
  }
}
```

### Field Types

| Type | Rendering | Example |
|------|-----------|---------|
| `text` | Plain text, supports `{{var}}` interpolation | "Frankfurt → Tokyo" |
| `date` | Localized date display | "June 15, 2026" |
| `date_range` | Start → End dates | "June 15 – June 22" |
| `currency` | Amount + currency symbol | "€2,840.00" |
| `list` | Bulleted list of items | "• Sven M. (Adult)" |
| `number` | Formatted number | "3,500" |
| `boolean` | Check/cross icon | ✓ / ✗ |
| `image` | Thumbnail image | Product photo |

### Domain Verification

To get the "Verified" badge, machines must prove domain ownership:

```
# DNS TXT record:
_atap-verify.lufthansa.com IN TXT "atap-verification=tpl_flight_booking"
```

Platform checks this during template registration and displays a badge in the mobile app.

---

## 17. Federation & Key Discovery

### DNS TXT Records

```
{entity-id}._atep.{domain} IN TXT "v=atap1; k=ed25519; p={base64-pubkey}; kid={key-id}"
```

### Well-Known Endpoint

```
GET https://{domain}/.well-known/atap.json

{
  "atap_discovery": "1",
  "entities": [
    {
      "uri": "machine://simrelay-prod",
      "public_key": {
        "algorithm": "ed25519",
        "key_id": "key_simrelay_01",
        "public": "base64-key"
      },
      "revocation_url": "https://simrelay.com/.well-known/atap-revocations.json"
    }
  ]
}
```

### Revocation Lists

```
GET https://{domain}/.well-known/atap-revocations.json

{
  "entity": "human://h7x9k2m4",
  "revocations": [
    { "delegation_id": "del_7f8a9b", "revoked_at": "2026-03-11T10:00:00Z", "reason": "compromised" }
  ],
  "published_at": "2026-03-11T10:01:00Z",
  "signature": { "key_id": "key_sven_01", "sig": "base64-sig" }
}
```

### Discovery Priority

1. Local cache (if fresh)
2. `.well-known/atap.json` (if domain known)
3. DNS TXT record
4. Registry lookup (fallback)

---

## 18. World ID Integration

### Flow

1. Human taps "Verify with World ID" in mobile app
2. App opens IDKit v4 WebView
3. User scans QR with World App (or uses in-app verification)
4. IDKit returns ZK proof
5. App sends proof to platform backend: `POST /v1/attestations/worldid`
6. Backend verifies with World ID API: `POST https://developer.world.org/api/v4/verify/{rp_id}`
7. On success: World ID attestation added, trust level recalculated → 2

### Backend

```go
func (h *AttestationHandler) WorldID(c *fiber.Ctx) error {
    var proof WorldIDProof
    c.BodyParser(&proof)

    // Forward to World ID verification API
    resp, err := h.worldIDClient.Verify(proof)
    if err != nil || !resp.Success {
        return c.Status(400).JSON(problem("worldid_verification_failed"))
    }

    // Add World ID attestation to entity
    entity := c.Locals("entity").(*Entity)
    h.store.AddAttestation(entity.ID, "world_id", map[string]interface{}{
        "proof_type":         "orb",
        "uniqueness":         "verified",
        "identity_disclosed": false,
        "verified_at":        time.Now().UTC().Format(time.RFC3339),
    })

    // Recalculate trust level
    h.store.RecalculateTrustLevel(entity.ID)

    return c.JSON(fiber.Map{"trust_level": 2, "verified": true})
}
```

---

## 19. SIMRelay Integration

### Reverse SMS Attestation Flow

1. User enters phone number in mobile app
2. App requests attestation: `POST /v1/attestations/phone/start`
3. Platform generates unique code: `ATAP-XXXX`
4. Platform registers expected SMS with SIMRelay
5. App displays: "Send SMS with code ATAP-XXXX to +49..."
6. User sends SMS (costs nothing — reverse SMS model)
7. SIMRelay receives SMS, matches code
8. SIMRelay webhook hits platform: `POST /v1/webhooks/simrelay`
9. Platform verifies code, adds phone attestation to entity
10. App receives confirmation via SSE/push, trust level recalculated

### Backend

```go
func (h *AttestationHandler) PhoneStart(c *fiber.Ctx) error {
    var req struct{ Phone string `json:"phone"` }
    c.BodyParser(&req)

    code := generateClaimCode()  // "ATAP-7X9K"
    entity := c.Locals("entity").(*Entity)

    // Register with SIMRelay
    h.simrelay.RegisterExpectedSMS(req.Phone, code, entity.ID)

    // Store pending attestation
    h.store.CreatePendingAttestation(entity.ID, "phone", req.Phone, code)

    return c.JSON(fiber.Map{
        "code": code,
        "send_to": h.config.SIMRelayNumber,
        "instructions": "Send an SMS with this code to the number above",
    })
}

func (h *WebhookHandler) SIMRelay(c *fiber.Ctx) error {
    var webhook SIMRelayWebhook
    c.BodyParser(&webhook)

    // Verify SIMRelay signature
    if !h.simrelay.VerifySignature(c) { return c.Status(401) }

    // Match code to pending attestation
    pending := h.store.GetPendingAttestationByCode(webhook.Body)
    if pending == nil { return c.Status(404) }

    // Add phone attestation to entity
    h.store.AddAttestation(pending.EntityID, "phone", map[string]interface{}{
        "number":      webhook.From,
        "verified_at": time.Now().UTC().Format(time.RFC3339),
        "method":      "reverse_sms",
    })

    // Recalculate trust level
    h.store.RecalculateTrustLevel(pending.EntityID)

    // Send confirmation signal to entity's inbox
    h.delivery.SendSystemSignal(pending.EntityID, Signal{
        Data: map[string]interface{}{
            "type":  "attestation_verified",
            "attestation": "phone",
            "phone": webhook.From,
        },
    })

    return c.Status(200)
}
```

---

## 20. Infrastructure & Deployment

### Docker Compose (Development)

```yaml
version: "3.9"

services:
  platform:
    build: ./platform
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://atap:atep@postgres:5432/atap?sslmode=disable
      REDIS_URL: redis://redis:6379
      SIMRELAY_API_KEY: ${SIMRELAY_API_KEY}
      WORLDID_APP_ID: ${WORLDID_APP_ID}
      WORLDID_RP_ID: ${WORLDID_RP_ID}
      WORLDID_RP_SIGNING_KEY: ${WORLDID_RP_SIGNING_KEY}
      FCM_CREDENTIALS: ${FCM_CREDENTIALS}
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: atap
      POSTGRES_PASSWORD: atap
      POSTGRES_DB: atap
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  pgdata:
```

### Production (Hetzner + Coolify)

```
Hetzner Cloud
├── VPS 1: Platform (Go service)
│   ├── Caddy (reverse proxy, auto HTTPS)
│   ├── atap-platform (Go binary)
│   └── Redis
├── VPS 2: Database
│   ├── PostgreSQL 16
│   └── Automated backups → Hetzner Object Storage
└── WireGuard VPN between all nodes
```

### Domains & DNS

```
atap.dev         → Landing page / docs (static, Cloudflare Pages or Hetzner)
atap.app         → Mobile app landing + web dashboard
api.atap.app     → Platform API
```

### Environment Variables

```env
# Database
DATABASE_URL=postgres://user:pass@host:5432/atap?sslmode=require

# Redis
REDIS_URL=redis://host:6379

# SIMRelay
SIMRELAY_API_URL=https://api.simrelay.com
SIMRELAY_API_KEY=sr_xxx
SIMRELAY_WEBHOOK_SECRET=whsec_xxx
SIMRELAY_NUMBER=+49xxx

# World ID
WORLDID_APP_ID=app_xxx
WORLDID_RP_ID=rp_xxx
WORLDID_RP_SIGNING_KEY=sk_xxx

# Firebase (Push)
FCM_CREDENTIALS_JSON=base64-encoded-service-account

# Platform
PLATFORM_ED25519_PRIVATE_KEY=base64-encoded-key
PLATFORM_DOMAIN=atap.app
JWT_SECRET=xxx
```

---

## 21. Testing Strategy

### Unit Tests

```
platform/internal/crypto/*_test.go       — Ed25519 sign/verify, X25519 encrypt/decrypt
platform/internal/verify/*_test.go       — Delegation chain verification
platform/internal/models/*_test.go       — Model validation
```

### Integration Tests

```
platform/test/api_test.go               — Full API endpoint tests
platform/test/sse_test.go               — SSE delivery end-to-end
platform/test/webhook_test.go           — Webhook delivery + retry
platform/test/claim_flow_test.go        — Full claim flow
platform/test/delegation_flow_test.go   — Create, verify, revoke
```

### End-to-End Tests

```
# Test: Agent registers → gets inbox → receives signal via SSE
# Test: Agent requests claim → human approves → delegation minted
# Test: Signal with signature → verify → accept/reject
# Test: Revoke delegation → cascading invalidation
# Test: Webhook inbound → channel → inbox → SSE outbound
```

### Mobile Tests

```
mobile/test/unit/            — Model serialization, crypto operations
mobile/test/widget/          — Screen rendering, approval card
mobile/test/integration/     — API calls, SSE connection
```

### Load Testing

```
# k6 or vegeta
# Target: 1000 concurrent SSE connections
# Target: 10,000 signals/second throughput
# Target: <100ms P99 SSE delivery latency
```

---

## 22. SDK Specifications

### Python SDK

```python
import atap

# Register agent
agent = await atap.register(name="travel-booker")
# Returns: Entity with uri, token, keypair

# Send signal
await agent.send(
    target="machine://simrelay-prod",
    data={"type": "sms_request", "phone": "+49..."}
)

# Listen for signals (SSE)
async for signal in agent.listen():
    print(f"From: {signal.route.origin}")
    print(f"Data: {signal.data}")

# Poll inbox
signals = await agent.poll(limit=10)

# Request claim
claim = await agent.request_claim(
    scopes=["sms:read", "sms:receive"],
    description="Travel booking assistant"
)
print(f"Ask your human to visit: {claim.claim_url}")

# MCP tool server
from atep.mcp import ATAPToolServer
server = ATAPToolServer(agent)
# Exposes: check_inbox, send_signal, get_delegation tools
```

### JavaScript SDK

```typescript
import { ATAP } from 'atap';

// Register agent
const agent = await ATAP.register({ name: 'travel-booker' });

// Send signal
await agent.send('machine://simrelay-prod', {
  type: 'sms_request',
  phone: '+49...',
});

// Listen for signals (SSE)
agent.listen((signal) => {
  console.log(`From: ${signal.route.origin}`);
  console.log(`Data:`, signal.data);
});

// Request claim
const claim = await agent.requestClaim({
  scopes: ['sms:read', 'sms:receive'],
  description: 'Travel booking assistant',
});
console.log(`Claim URL: ${claim.claimUrl}`);

// MCP tool
import { createATAPMCPServer } from 'atap/mcp';
const mcpServer = createATAPMCPServer(agent);
```

### Go SDK

```go
import "github.com/atap-dev/atap-go"

// Register agent
agent, err := atap.Register(ctx, atap.RegisterOpts{
    Name: "travel-booker",
})

// Send signal
err = agent.Send(ctx, "machine://simrelay-prod", atap.SignalData{
    "type":  "sms_request",
    "phone": "+49...",
})

// Listen for signals (SSE)
ch, err := agent.Listen(ctx)
for signal := range ch {
    fmt.Printf("From: %s\n", signal.Route.Origin)
    fmt.Printf("Data: %v\n", signal.Data)
}

// Request claim
claim, err := agent.RequestClaim(ctx, atap.ClaimOpts{
    Scopes:      []string{"sms:read", "sms:receive"},
    Description: "Travel booking assistant",
})
fmt.Printf("Claim URL: %s\n", claim.ClaimURL)
```

---

## CLAUDE.md Instructions

```markdown
# ATAP Platform — Claude Code Instructions

## What This Is
ATAP (Agent Trust and Authority Protocol) is an open protocol for verifiable
delegation of trust between AI agents, machines, humans, and organizations.

## Key Architecture Decisions
- Backend: Go with Fiber framework
- Database: PostgreSQL with JSONB
- Real-time: Redis pub/sub → SSE delivery
- Mobile: Flutter (iOS + Android)
- Crypto: Ed25519 signing, X25519 encryption (NaCl/libsodium)
- SDKs: Python, JavaScript/TypeScript, Go

## Build Order
Always follow the phase order. Do not jump ahead.
Phase 1 must work end-to-end before starting Phase 2.

## Code Style
- Go: standard library conventions, gofmt, no frameworks beyond Fiber
- Error handling: always wrap with context, use RFC 7807 for API errors
- Logging: zerolog, structured JSON
- Tests: table-driven tests, test files next to source
- SQL: use migrations, never raw DDL in application code

## Important Patterns
- Signal IDs: "sig_" + ULID
- Entity IDs (agents/machines): lowercase alphanumeric + hyphens, 4-64 chars, assigned by registry
- Human IDs: derived from public key: lowercase(base32(sha256(ed25519_pubkey))[:16])
- Email/phone are ATTESTATIONS, not identifiers — stored in attestations JSONB, never used as entity ID
- Tokens: "atap_" + 32 bytes base64url, stored as SHA-256 hash
- Canonical JSON: sorted keys, no whitespace (for signatures)
- SSE: Last-Event-ID for reconnection, 30s heartbeat

## Security Rules
- Private keys NEVER leave the device that generated them
- Human keys stored in mobile secure enclave only
- Human identity is derived from public key — self-sovereign, no external dependency
- Key recovery via Argon2id-encrypted backup (passphrase required)
- Platform CANNOT read encrypted signal bodies
- All delegation documents are self-verifying
- Tokens are stored hashed, never plaintext
- Removing an attestation (GDPR erasure) MUST NOT break delegation chains

## Domain Model
- atap.dev = protocol spec, SDKs, docs
- atap.app = hosted platform, mobile app, API
- api.atap.app = backend API
```

---

*This document is the single source of truth for building ATAP. Update it as decisions change.*
