# Milestones

## v1.0 — v1.0-rc1 (Shipped: 2026-03-17)

**Phases completed:** 4 phases, 14 plans
**Timeline:** 7 days (2026-03-11 → 2026-03-17)
**Codebase:** 17,004 Go LOC + 3,058 Dart LOC | 173 commits

**Key accomplishments:**
1. Stripped legacy signal pipeline, rebuilt domain model with DID/approval/VC schema
2. DID identity layer — `did:web` registration, DID Document resolution, key rotation, OAuth 2.1 + DPoP auth
3. DIDComm v2.1 messaging — ECDH-1PU authenticated encryption, server mediator, message queue + inbox
4. Approval engine — revocation infrastructure, Adaptive Card templates, DIDComm-based approval lifecycle
5. W3C Verifiable Credentials — issuance engine, trust levels, SD-JWT, Bitstring Status List, crypto-shredding
6. Flutter mobile app — DIDComm inbox, approval card rendering, biometric signing, credential management

### Known Tech Debt
- TODO: add IP-based rate limiting (Phase 1)
- Settings screen is placeholder (Phase 4)
- Mobile template rendering uses legacy format, not Adaptive Cards (Phase 4)
- Refresh token stored but unused (Phase 4)

---

