---
phase: 01-identity-and-auth-foundation
plan: 02
subsystem: did-identity-layer
tags: [did:web, entity-registration, key-rotation, did-document, ed25519]
dependency_graph:
  requires: ["01-01"]
  provides: ["entity-crud-endpoints", "did-document-resolution", "key-version-store"]
  affects: ["01-03"]
tech_stack:
  added:
    - "mr-tron/base58: multibase encoding for Ed25519VerificationKey2020"
    - "golang.org/x/crypto/bcrypt: client secret hashing for agent/machine credentials"
  patterns:
    - "did:web DID construction: did:web:{domain}:{type}:{id}"
    - "Multibase encoding: 'z' prefix + base58btc(ed25519_pubkey)"
    - "TDD: RED tests written first, GREEN implementation second"
    - "application/did+ld+json Content-Type for DID Document endpoints (not application/json)"
    - "bcrypt for client_secret_hash, secret returned once at registration"
key_files:
  created:
    - platform/internal/crypto/did.go
    - platform/internal/crypto/did_test.go
    - platform/internal/api/entities.go
    - platform/internal/api/entities_test.go
    - platform/internal/api/did.go
    - platform/internal/api/did_test.go
    - platform/internal/store/key_versions.go
    - platform/internal/store/key_versions_test.go
  modified:
    - platform/internal/api/api.go
    - platform/internal/api/discovery_test.go
    - platform/internal/models/models.go
    - platform/cmd/server/main.go
decisions:
  - "agent type requires principal_did at registration (enforced in CreateEntity handler)"
  - "Human entity IDs are derived from public key hash (DeriveHumanID) -- deterministic, not ULID"
  - "Agent/machine types receive client_secret (atap_ prefix, 32 bytes base64url) returned once at registration"
  - "Key rotation uses database transaction (pgx.BeginTxFunc) to atomically expire old key and insert new"
  - "DID Document endpoint uses manual JSON marshaling + c.Set(Content-Type) to avoid Fiber's application/json default"
  - "ResolveDID validates type matches entity.Type to prevent cross-type resolution (returns 404 on mismatch)"
  - "newTestHandlerWithStores helper added to discovery_test.go to share mock setup across entity/did tests"
metrics:
  duration: "9 min"
  completed: "2026-03-13"
  tasks_completed: 3
  files_created: 8
  files_modified: 4
---

# Phase 1 Plan 2: DID Identity Layer Summary

DID crypto helpers (BuildDID, EncodePublicKeyMultibase, BuildDIDDocument), entity CRUD endpoints, DID Document resolution at standard did:web path, and key rotation with version history.

## Tasks Completed

### Task 1: DID crypto helpers and entity registration endpoint

**Status:** Complete

Created `platform/internal/crypto/did.go` with three functions:
- `BuildDID(domain, entityType, entityID string) string` -- `did:web:{domain}:{type}:{id}`
- `EncodePublicKeyMultibase(pub ed25519.PublicKey) string` -- `"z" + base58btc(pub)` per Ed25519VerificationKey2020
- `BuildDIDDocument(entity, keyVersions, domain) *models.DIDDocument` -- full W3C DID Document with 3-element @context, all key versions in verificationMethod, only active key in authentication/assertionMethod

Created `platform/internal/store/key_versions.go` with `CreateKeyVersion`, `GetActiveKeyVersion`, `GetKeyVersions`, `RotateKey` (transactional).

Created `platform/internal/api/entities.go` handling `POST /v1/entities`, `GET /v1/entities/{id}`, `DELETE /v1/entities/{id}`, `POST /v1/entities/{id}/keys/rotate`.

Updated `api.go` to add `KeyVersionStore` interface, updated `NewHandler` signature (es, kvs, rdb, platformKey, cfg, log), and `SetupRoutes` with entity routes and DID resolution route.

Updated `models.go`: `CreateEntityRequest.PrincipalDID`, `CreateEntityResponse.ClientSecret`.

**Commit:** eb4e749

### Task 2: DID Document resolution endpoint

**Status:** Complete

Created `platform/internal/api/did.go` with `ResolveDID` handler:
- Validates entity type is one of 4 valid types (returns 404 otherwise)
- Looks up entity, validates type matches URL parameter (prevents cross-type resolution)
- Fetches all key versions via `GetKeyVersions`
- Calls `BuildDIDDocument` then manually marshals JSON and sets `Content-Type: application/did+ld+json`

Created `platform/internal/api/did_test.go` with full table-driven test coverage including rotated key history, type mismatch 404, and content-type verification.

**Commit:** e073e34

### Task 3: Key rotation store and endpoint

**Status:** Complete

Key rotation implementation confirmed complete from Task 1. Added:
- `platform/internal/store/key_versions_test.go`: behavioral contract tests for all 4 store methods (no DB required via in-memory mock)
- `TestRotateKey` in `entities_test.go`: POST /v1/entities/{id}/keys/rotate endpoint tests

**Commit:** db27306

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] discovery_test.go redeclared newTestHandler function**
- **Found during:** Task 1 compilation
- **Issue:** `entities_test.go` defined `newTestHandler(es, kvs)` clashing with existing `discovery_test.go`'s `newTestHandler(t *testing.T)`
- **Fix:** Added `newTestHandlerWithStores(es, kvs, cfg)` helper in `discovery_test.go`, entities tests use that
- **Files modified:** `platform/internal/api/discovery_test.go`, `platform/internal/api/entities_test.go`
- **Commit:** eb4e749

**2. [Rule 1 - Bug] linter reverted api.go changes**
- **Found during:** Task 1 implementation
- **Issue:** The editor environment reverted api.go to the previous version after my Write tool call
- **Fix:** Re-wrote api.go with full content using Write tool
- **Files modified:** `platform/internal/api/api.go`

## Verification Results

```
go test ./internal/crypto/... -v    PASS (13 tests including 5 new DID tests)
go test ./internal/api/... -v       PASS (18 tests including entity CRUD + DID resolution)
go test ./internal/store/... -v     PASS (5 tests including key rotation contracts)
go build ./...                      PASS (compiles cleanly)
```

## Self-Check

Files created/verified:
- [x] platform/internal/crypto/did.go
- [x] platform/internal/crypto/did_test.go
- [x] platform/internal/api/entities.go
- [x] platform/internal/api/entities_test.go
- [x] platform/internal/api/did.go
- [x] platform/internal/api/did_test.go
- [x] platform/internal/store/key_versions.go
- [x] platform/internal/store/key_versions_test.go

Commits verified:
- [x] eb4e749 feat(01-02): DID crypto helpers, entity CRUD endpoints, key version store
- [x] e073e34 feat(01-02): DID Document resolution endpoint with full test coverage
- [x] db27306 feat(01-02): key rotation tests - store contract and API endpoint
