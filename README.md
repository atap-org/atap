# ATAP — Agent Trust and Authority Protocol

An open protocol for verifiable delegation of trust between AI agents, machines, humans, and organizations.

**[atap.dev](https://atap.dev)** — Protocol spec, SDKs, docs  
**[atap.app](https://atap.app)** — Hosted platform, mobile app

## The Problem

AI agents have no persistent identity, no way to receive notifications, and no verifiable link to the humans they act for.

## The Solution

ATAP provides every entity — agent, machine, human, or organization — with a cryptographic identity and a verifiable delegation chain. Any party can answer: *who is this agent, who authorized it, and what can it do?*

## Quick Start

```bash
# Start the platform
docker compose up -d

# Register an agent (30 seconds)
curl -X POST http://localhost:8080/v1/register \
  -H "Content-Type: application/json" \
  -d '{"name": "my-first-agent"}'

# Response:
# {
#   "uri": "agent://01jd7x...",
#   "token": "atap_abc123...",
#   "inbox_url": "http://localhost:8080/v1/inbox/01jd7x...",
#   "stream_url": "http://localhost:8080/v1/inbox/01jd7x.../stream"
# }
```

## Entity Model

```
agent://   — AI agents (ephemeral, goal-driven)
machine:// — Persistent applications and services
human://   — Trust anchors (identity derived from public key)
org://     — Organizational umbrellas
```

## Trust Levels

| Level | Verification | Use Cases |
|-------|-------------|-----------|
| 0 | Self-registration | Inbox, basic signals |
| 1 | Email + Phone verified | Service integrations |
| 2 | World ID (ZK proof) | Payments, commerce |
| 3 | eID + Org verification | Regulated transactions |

## Repository Structure

```
platform/    — Go backend (Fiber, PostgreSQL, Redis)
mobile/      — Flutter app (iOS + Android)
sdks/        — Python, JavaScript, Go client SDKs
spec/        — Protocol specification + JSON schemas
web/         — atap.dev static site
docs/        — Documentation
```

## Where ATAP Sits

```
┌─────────────────────────────────┐
│   Applications                  │
├─────────────────────────────────┤
│   AP2 (payments)                │
├─────────────────────────────────┤
│   A2A (agent comms) / MCP       │
├─────────────────────────────────┤
│   ATAP (identity + delegation)  │  ← this layer
├─────────────────────────────────┤
│   Transport (HTTP, SSE)         │
└─────────────────────────────────┘
```

## License

Apache 2.0 — everything is open source. The asset is the trust graph, not the code.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). All contributions welcome.
