# Requirements: ATAP v1.1 Tech Debt

**Defined:** 2026-03-17
**Core Value:** Any party can cryptographically verify who authorized an AI agent, what it may do, and under what constraints — offline, without callback to an authorization server.

## v1.1 Requirements

### API Hardening

- [ ] **API-07**: API endpoints enforce IP-based rate limiting per client

### Mobile Polish

- [ ] **MOB-07**: Approval cards render via Adaptive Cards format (replace legacy brand/display rendering)
- [ ] **MOB-08**: OAuth token refresh using stored refresh token when access token expires
- [ ] **MOB-09**: Settings screen with functional controls (notification preferences, account management)

## Future Requirements

Deferred to v2.0+:

- **FED-01**: Cross-server DIDComm relay (federation between ATAP servers)
- **FED-02**: `did:webs` support for server-independent key authority
- **EXT-01**: GNAP (RFC 9635) extension for multi-signature approval
- **EXT-02**: ATAPApproval-as-VC wrapping for OpenID4VP presentation
- **EXT-03**: eIDAS 2.0 credential import via OpenID4VP
- **SEC-01**: Formal verification of approval flow (Tamarin Prover)
- **SEC-02**: Post-quantum migration via composite JWS (EdDSA+ML-DSA-65)

## Out of Scope

| Feature | Reason |
|---------|--------|
| New protocol features | v1.1 is tech debt only |
| Federation | Deferred to v2.0 |
| New credential types | Not tech debt |
| Client SDKs | API not stable yet |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| API-07 | Phase 5 | Pending |
| MOB-07 | Phase 6 | Pending |
| MOB-08 | Phase 6 | Pending |
| MOB-09 | Phase 6 | Pending |

**Coverage:**
- v1.1 requirements: 4 total
- Mapped to phases: 4
- Unmapped: 0

---
*Requirements defined: 2026-03-17 | Traceability updated: 2026-03-17*
