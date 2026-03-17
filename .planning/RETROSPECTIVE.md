# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — v1.0-rc1

**Shipped:** 2026-03-17
**Phases:** 4 | **Plans:** 14 | **Timeline:** 7 days

### What Was Built
- Complete DID identity system with `did:web`, key rotation, and DID Document resolution
- DIDComm v2.1 messaging with ECDH-1PU authenticated encryption and server mediation
- Approval engine with revocation infrastructure, Adaptive Card templates, JWS multi-signatures
- W3C Verifiable Credentials: issuance, trust levels, SD-JWT, Bitstring Status List, crypto-shredding
- Flutter mobile app with DIDComm inbox, approval card rendering, biometric signing
- Organization delegate routing with fan-out and first-response-wins

### What Worked
- Coarse 4-phase granularity kept momentum high — no planning overhead between sub-phases
- Inline ECDH-1PU implementation avoided immature library dependencies
- Spec v1.0-rc1 rework mid-milestone (Phase 3) was handled cleanly with replanning
- Milestone audit caught 5 cross-phase integration breaks before they became production bugs
- Table-driven Go tests with store contracts made rapid iteration safe

### What Was Inefficient
- Phase 3 was planned twice: once with server-side approval storage, then replanned after spec rework to server-stateless model
- REQUIREMENTS.md traceability table fell out of sync (14 items marked "Needs rework" that were actually completed)
- Phase 4 plan 04-05 was a gap-closure plan added after audit — suggests Phase 4 scope wasn't fully analyzed upfront
- Mobile template rendering was built against legacy format, not Adaptive Cards (tech debt carried forward)

### Patterns Established
- Go store interface pattern: each domain gets `*Store` interface, Handler composes multiple store interfaces
- DIDComm as universal transport: all entity-to-entity communication (approvals, revocations, credentials) routed via DIDComm
- Server-derived keys via HKDF from Ed25519 seed — no additional secrets management
- Atomic state transitions via SQL WHERE clauses (no app-level mutexes)
- Per-entity AES-256-GCM encryption keys for crypto-shredding support

### Key Lessons
1. Spec alignment mid-build is expensive but manageable if the phase boundary is clean — Phase 3 replan was smooth because Phase 2 (messaging) was stable
2. Gap-closure plans should be anticipated: run audit earlier (after 80% completion, not 100%)
3. Mobile and backend should share a contract (Adaptive Cards format) from day one — divergent formats create tech debt
4. Inline crypto (ECDH-1PU, ConcatKDF) was the right call — go-jose and didcomm-go didn't support our needs

### Cost Observations
- Model mix: ~70% opus, ~20% sonnet, ~10% haiku (balanced profile)
- 14 plans across 4 phases in 7 days
- Notable: Phase 02 plans averaged 5.5min each — cleanest phase due to clear spec and no rework

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Timeline | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | 7 days | 4 | First milestone — established patterns |

### Top Lessons (Verified Across Milestones)

1. Coarse phase granularity (4 phases for full protocol stack) keeps velocity high
2. Inline crypto beats immature libraries when spec compliance is critical
3. Run milestone audit at 80%, not 100%, to catch integration gaps earlier
