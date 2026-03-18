# Roadmap: ATAP

## Milestones

- v1.0 v1.0-rc1 — Phases 1-4 (shipped 2026-03-17)
- v1.1 Tech Debt — Phases 5-6 (in progress)

## Phases

<details>
<summary>v1.0 v1.0-rc1 (Phases 1-4) — SHIPPED 2026-03-17</summary>

- [x] Phase 1: Identity and Auth Foundation (4/4 plans) — completed 2026-03-13
- [x] Phase 2: DIDComm Messaging (3/3 plans) — completed 2026-03-13
- [x] Phase 3: Approval Engine (2/2 plans) — completed 2026-03-16
- [x] Phase 4: Credentials and Mobile (5/5 plans) — completed 2026-03-17

See: milestones/v1.0-ROADMAP.md for full details

</details>

### v1.1 Tech Debt (In Progress)

**Milestone Goal:** Close all accumulated tech debt from v1.0 — rate limiting, Adaptive Cards rendering, token refresh, settings screen.

- [ ] **Phase 5: API Hardening** - Enforce IP-based rate limiting on all API endpoints
- [ ] **Phase 6: Mobile Polish** - Replace legacy rendering, activate token refresh, build settings screen

## Phase Details

### Phase 5: API Hardening
**Goal**: API endpoints protect against abuse via IP-based rate limiting
**Depends on**: Phase 4
**Requirements**: API-07
**Success Criteria** (what must be TRUE):
  1. Repeated requests from the same IP beyond the configured threshold receive 429 Too Many Requests responses
  2. Requests within the rate limit threshold are processed normally with no latency impact
  3. Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, Retry-After) are present on responses
**Plans:** 2 plans

Plans:
- [ ] 05-01-PLAN.md — Rate limit middleware: migration, store method, middleware implementation, wiring
- [ ] 05-02-PLAN.md — Comprehensive tests and full suite regression verification

### Phase 6: Mobile Polish
**Goal**: Mobile app renders approvals with Adaptive Cards, refreshes tokens automatically, and provides a functional settings screen
**Depends on**: Phase 5
**Requirements**: MOB-07, MOB-08, MOB-09
**Success Criteria** (what must be TRUE):
  1. Approval cards in the mobile inbox display using the Adaptive Cards format with correct field rendering (replacing legacy brand/display format)
  2. When an access token expires, the app silently obtains a new one using the stored refresh token without prompting the user to log in again
  3. The settings screen is navigable and shows functional controls for notification preferences and account management
  4. Changing a notification preference in settings persists across app restarts
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Identity and Auth Foundation | v1.0 | 4/4 | Complete | 2026-03-13 |
| 2. DIDComm Messaging | v1.0 | 3/3 | Complete | 2026-03-13 |
| 3. Approval Engine | v1.0 | 2/2 | Complete | 2026-03-16 |
| 4. Credentials and Mobile | v1.0 | 5/5 | Complete | 2026-03-17 |
| 5. API Hardening | v1.1 | 0/2 | Planned | - |
| 6. Mobile Polish | v1.1 | 0/? | Not started | - |
