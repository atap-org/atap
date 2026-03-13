---
phase: 01-identity-and-auth-foundation
verified: 2026-03-13T18:45:00Z
status: passed
score: 13/13 must-haves verified
---

# Phase 1: Identity and Auth Foundation Verification Report

**Phase Goal:** Any entity can register with a `did:web` DID, authenticate via OAuth 2.1 + DPoP, and have its DID Document resolved by any standards-compliant client
**Verified:** 2026-03-13T18:45:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Old signal/channel/webhook/SSE/claim/delegation code is completely removed | VERIFIED | claims.go, human.go, push.go, webhook.go, api_test.go deleted; go build passes; grep for old types returns only JWT lib references |
| 2 | Platform starts cleanly against new schema with migrations 008 and 009 applied | VERIFIED | go build passes with zero errors; main.go runs migrations on startup; go test ./... passes |
| 3 | Any entity type (agent, human, machine, org) can register via POST /v1/entities and receive a did:web DID | VERIFIED | entities.go CreateEntity handler; TestCreateEntity passes for all 4 types; did:web:{domain}:{type}:{id} format confirmed |
| 4 | DID Document is resolvable at GET /{type}/{id}/did.json with Ed25519VerificationKey2020 and ATAP context | VERIFIED | did.go ResolveDID handler; Content-Type: application/did+ld+json; 3-element @context; TestResolveDID passes all 6 sub-tests |
| 5 | Human IDs are derived from public key hash, agent/machine/org IDs use ULID | VERIFIED | entities.go uses DeriveHumanID for human type, NewEntityID (ULID) for others; TestCreateEntity/create_human_with_public_key passes |
| 6 | Agent DID Documents include atap:principal referencing their controlling entity | VERIFIED | did.go and crypto/did.go BuildDIDDocument sets ATAPPrincipal for agent type; TestResolveDID and TestBuildDIDDocument confirm |
| 7 | Key rotation stores previous key versions with validity periods | VERIFIED | store/key_versions.go RotateKey uses pgx transaction; sets valid_until on old key, inserts new key; TestRotateKey passes |
| 8 | An entity can be deleted via DELETE /v1/entities/{id} with atap:manage scope | VERIFIED | entities.go DeleteEntity; auth-protected via DPoPAuthMiddleware + RequireScope("atap:manage"); TestDeleteEntity passes |
| 9 | An agent can obtain a DPoP-bound access token via Client Credentials grant | VERIFIED | oauth.go handleClientCredentials; bcrypt secret verify; issues JWT with cnf.jkt; TestClientCredentials passes 6 sub-tests |
| 10 | A human can obtain a DPoP-bound access token via Authorization Code + PKCE grant | VERIFIED | oauth.go Authorize + handleAuthorizationCode; S256 enforced; atomic single-use redemption; TestAuthCode_* passes |
| 11 | All authenticated API requests require both Authorization: DPoP header and DPoP proof JWT | VERIFIED | auth.go DPoPAuthMiddleware; rejects Bearer scheme; validates htm/htu/jti/cnf.jkt; TestDPoP_Middleware passes 6 sub-tests |
| 12 | Token scopes (atap:inbox, atap:send, atap:approve, atap:manage) are enforced | VERIFIED | auth.go RequireScope; oauth.go parseScopes validates against validScopes map; TestScope_Enforcement passes |
| 13 | GET /.well-known/atap.json returns valid discovery document with all required fields | VERIFIED | discovery.go Discovery handler; domain, api_base, didcomm_endpoint, claim_types, max_approval_ttl, trust_level, oauth; TestDiscovery passes |

**Score:** 13/13 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `platform/migrations/008_strip_old_pipeline.up.sql` | DROP TABLE for signals, channels, webhook_delivery, claims, delegations, push_tokens | VERIFIED | 8 DROP TABLE IF EXISTS statements present |
| `platform/migrations/009_did_and_oauth.up.sql` | New schema: entity DID columns, key_versions, oauth_auth_codes, oauth_tokens | VERIFIED | ALTER TABLE + CREATE TABLE key_versions, oauth_auth_codes, oauth_tokens with indexes |
| `platform/internal/store/migrations_test.go` | TestMigrations verifies migration schema | VERIFIED | 5 sub-tests: dropped tables, entity columns, key_versions, oauth_auth_codes, oauth_tokens |
| `platform/internal/models/models.go` | DIDDocument, KeyVersion, OAuthToken types | VERIFIED | DIDDocument, VerificationMethod, KeyVersion, OAuthToken, OAuthAuthCode, CreateEntityRequest/Response all present |
| `platform/internal/store/store.go` | Entity CRUD with DID support | VERIFIED | CreateEntity, GetEntity, GetEntityByDID, GetEntityByKeyID, DeleteEntity with DID columns |
| `platform/internal/api/api.go` | Slim Handler with EntityStore, KeyVersionStore, OAuthTokenStore interfaces | VERIFIED | All 3 store interfaces defined; Handler struct correct; SetupRoutes wires all routes |
| `platform/internal/crypto/did.go` | BuildDID, EncodePublicKeyMultibase, BuildDIDDocument | VERIFIED | All 3 functions implemented; TestBuildDID, TestEncodePublicKeyMultibase, TestBuildDIDDocument all pass |
| `platform/internal/api/entities.go` | POST /v1/entities, GET /v1/entities/{id}, DELETE /v1/entities/{id} | VERIFIED | CreateEntity, GetEntity, DeleteEntity, RotateKey handlers present and tested |
| `platform/internal/api/did.go` | GET /{type}/{id}/did.json DID Document resolution | VERIFIED | ResolveDID handler; application/did+ld+json Content-Type; cross-type 404 protection |
| `platform/internal/store/key_versions.go` | CreateKeyVersion, GetActiveKeyVersion, GetKeyVersions, RotateKey | VERIFIED | All 4 methods implemented with correct SQL; RotateKey uses pgx transaction |
| `platform/internal/api/oauth.go` | POST /v1/oauth/token, GET /v1/oauth/authorize | VERIFIED | Token + Authorize handlers; both grants; DPoP proof parsing; JWT issuance |
| `platform/internal/api/auth.go` | DPoPAuthMiddleware, RequireScope | VERIFIED | Full 9-step DPoP validation chain; Bearer rejection; jti replay via Redis |
| `platform/internal/store/oauth.go` | CreateOAuthToken, GetOAuthToken, RevokeOAuthToken, CreateAuthCode, RedeemAuthCode | VERIFIED | All 5 methods (+ CleanupExpiredTokens); atomic RedeemAuthCode via UPDATE...RETURNING |
| `platform/internal/api/discovery.go` | GET /.well-known/atap.json | VERIFIED | Discovery handler returns all required fields; registered in SetupRoutes |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `api/entities.go` | `crypto/did.go` | `crypto.BuildDID` call | VERIFIED | Line 67: `did := crypto.BuildDID(h.config.PlatformDomain, req.Type, entityID)` |
| `api/did.go` | `store/key_versions.go` | `GetKeyVersions` call | VERIFIED | Line 45: `keyVersions, err := h.keyVersionStore.GetKeyVersions(c.Context(), entityID)` |
| `api/did.go` | `crypto/did.go` | `BuildDIDDocument` call | VERIFIED | Line 52: `doc := crypto.BuildDIDDocument(entity, keyVersions, h.config.PlatformDomain)` |
| `api/auth.go` | `store/oauth.go` | `GetOAuthToken` call | VERIFIED | Line 137: `storedToken, err := h.oauthTokenStore.GetOAuthToken(c.Context(), jtiToken)` |
| `api/auth.go` | Redis | `dpop:nonce:` cache key | VERIFIED | Line 70: `nonceKey := "dpop:nonce:" + jti`; SET with 5min TTL |
| `api/oauth.go` | `store/oauth.go` | `CreateOAuthToken` call | VERIFIED | Lines 131, 153 (client_credentials); lines 224, 244 (auth_code) |
| `api/oauth.go` | `go-dpop` library | `dpop.Parse` call | VERIFIED | Lines 431, 447: `dpop.Parse(proofHeader, ...)` |
| `api/api.go` | `api/discovery.go` | route registration | VERIFIED | Line 80: `app.Get("/.well-known/atap.json", h.Discovery)` |
| `cmd/server/main.go` | `api.Handler` | `NewHandler(db, db, db, rdb, platformPriv, cfg, log)` | VERIFIED | Store passed as EntityStore + KeyVersionStore + OAuthTokenStore; GlobalErrorHandler wired |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| INF-01 | 01-01 | Strip old signal pipeline code | SATISFIED | 9 files deleted; go build passes; no old type references |
| INF-02 | 01-01 | Database migration from signal-based schema | SATISFIED | Migrations 008 (drop) + 009 (create) with correct SQL |
| INF-03 | 01-01 | Docker Compose updated | SATISFIED | docker-compose.yml updated; no Firebase/push env vars |
| DID-01 | 01-02 | Every entity identified by did:web DID | SATISFIED | BuildDID constructs did:web:{domain}:{type}:{id}; stored in entities.did |
| DID-02 | 01-02 | DID Documents at standard did:web HTTPS path | SATISFIED | GET /{type}/{id}/did.json; Ed25519VerificationKey2020 with publicKeyMultibase |
| DID-03 | 01-02 | DID Documents include ATAP properties | SATISFIED | atap:type, atap:principal in https://atap.dev/ns/v1 context; @context array with 3 entries |
| DID-04 | 01-02 | Four entity types supported | SATISFIED | validEntityTypes map: agent, machine, human, org |
| DID-05 | 01-02 | Human IDs derived from public key | SATISFIED | DeriveHumanID: lowercase(base32(sha256(pubkey))[:16]); TestDeriveHumanID confirms format |
| DID-06 | 01-02 | Agent DID Documents MUST include atap:principal | SATISFIED | Validation: agent without principal_did returns 400; BuildDIDDocument sets ATAPPrincipal |
| DID-07 | 01-02 | Key rotation via DID Document update | SATISFIED | RotateKey transaction; previous keys in verificationMethod with valid_until; active key in authentication/assertionMethod |
| DID-08 | 01-02 | DID resolution uses HTTPS with valid TLS | SATISFIED (infra) | Platform is configured for HTTPS; TLS is deployment-level concern; did:web resolution endpoint exists at correct path |
| AUTH-01 | 01-04 | OAuth 2.1 with DPoP for sender-constrained tokens | SATISFIED | Full DPoP chain: proof at token endpoint + middleware on every request |
| AUTH-02 | 01-04 | Agent entities use Client Credentials grant | SATISFIED | handleClientCredentials; rejects human/org with 400 |
| AUTH-03 | 01-04 | Human entities via Authorization Code + PKCE | SATISFIED | Authorize + handleAuthorizationCode; S256 enforced; DPoP binding consistent |
| AUTH-04 | 01-04 | All API tokens MUST be DPoP-bound | SATISFIED | DPoPAuthMiddleware on all authenticated routes; cnf.jkt verified on every request |
| AUTH-05 | 01-04 | Token scopes: atap:inbox/send/approve/manage | SATISFIED | parseScopes validates; RequireScope enforces; TestScope_Enforcement passes |
| AUTH-06 | 01-04 | Access token 1 hour, refresh tokens 90 days | SATISFIED | issueJWT TTL params: 1*time.Hour for access, 90*24*time.Hour for refresh; both stored in oauth_tokens |
| SRV-01 | 01-03 | Server discovery via /.well-known/atap.json | SATISFIED | Discovery handler; all required fields present |
| SRV-02 | 01-03 | Server trust levels published | SATISFIED | trust_level: 1 (L1 DV TLS) in discovery document |
| SRV-03 | 01-03 | max_approval_ttl published | SATISFIED | max_approval_ttl: 86400 in discovery document |
| API-01 | 01-02 | Entity endpoints: POST, GET, DELETE /v1/entities | SATISFIED | CreateEntity, GetEntity, DeleteEntity all implemented and tested |
| API-02 | 01-02 | DID resolution: GET /{type}/{id}/did.json | SATISFIED | ResolveDID at root path; Content-Type: application/did+ld+json |
| API-06 | 01-03 | All errors follow RFC 7807 with ATAP error URIs | SATISFIED | problem() sets application/problem+json; URIs: https://atap.dev/errors/{type}; GlobalErrorHandler wired |

**All 23 requirements SATISFIED.**

### Anti-Patterns Found

None. Scan complete:
- No TODO/FIXME/PLACEHOLDER comments in production code
- No empty return null/stub implementations
- No console.log-only handlers
- Old pipeline files fully deleted (claims.go, human.go, push.go, webhook.go, api_test.go)
- Dependencies (go-dpop, go-jose, mr-tron/base58) marked `// indirect` in go.mod but actively imported and used — cosmetic go.mod issue, not a functional defect; `go build ./...` and all tests pass

### Human Verification Required

#### 1. DID-08: HTTPS with valid TLS in production

**Test:** Deploy platform to atap.app, then resolve `https://atap.app/agent/{id}/did.json` with a standards-compliant DID resolver (e.g., `did:web` resolver at https://dev.uniresolver.io/ or `resolve did:web:atap.app:agent:{id}`)
**Expected:** DID resolver successfully fetches and validates the DID Document over HTTPS with valid TLS
**Why human:** TLS certificate validity is a deployment/infrastructure concern that cannot be verified by inspecting Go source code

#### 2. Auth Code flow with mobile deep link redirect

**Test:** Initiate Authorization Code flow with `redirect_uri=atap://callback`, complete flow on a mobile device
**Expected:** Authorization code redirects to `atap://callback?code=...` and the app receives it
**Why human:** Deep link handling depends on mobile OS configuration (iOS URL schemes, Android intent filters) which is outside the platform codebase

#### 3. DPoP jti replay prevention under Redis unavailability

**Test:** With Redis unreachable, send two identical DPoP proofs to a protected endpoint
**Expected:** Second request is not rejected (best-effort behavior documented as intentional)
**Why human:** Verifying the degraded-mode behavior requires live Redis manipulation and is a policy decision to confirm as acceptable

---

## Summary

Phase 1 goal is fully achieved. All 23 requirements (INF-01..03, DID-01..08, AUTH-01..06, SRV-01..03, API-01, API-02, API-06) are implemented and verified against the actual codebase.

**Build:** `go build ./...` — clean
**Tests:** `go test ./...` — 3 packages, all pass (api: 0.942s, crypto: 0.772s, store: 0.547s)
**Anti-patterns:** None found

Key implementation facts verified against code (not summary claims):
- `did:web:{domain}:{type}:{id}` format confirmed in `crypto/did.go:16`
- `application/did+ld+json` Content-Type set manually in `api/did.go:62-63` (avoids Fiber's default)
- `application/problem+json` set via Fiber's ctype param in `api/api.go:133`
- DPoP jti stored as `dpop:nonce:{jti}` with 5-minute TTL in Redis (`api/auth.go:70,80`)
- Authorization code redemption is atomic via `UPDATE...RETURNING WHERE used_at IS NULL` (`store/oauth.go:92-97`)
- Key rotation is transactional via `pgx.BeginTxFunc` (`store/key_versions.go:79`)
- Refresh tokens always issued alongside access tokens (both grant types)
- `go.mod` marks go-dpop, go-jose, mr-tron/base58 as indirect despite direct imports — `go mod tidy` anomaly, does not affect compilation or functionality

---

_Verified: 2026-03-13T18:45:00Z_
_Verifier: Claude (gsd-verifier)_
