---
phase: 04-credentials-and-mobile
plan: 01
subsystem: database, api, crypto
tags: [verifiable-credentials, w3c-vc-2.0, ed25519, aes-256-gcm, sd-jwt, bitstring-status-list, trustbloc-vc-go, postgres]

# Dependency graph
requires:
  - phase: 03-approval-engine
    provides: entities table, store patterns, models.go base types

provides:
  - Migration 013: entity_enc_keys, credentials, credential_status_lists tables
  - Credential model types (Credential, EncryptionKey, CredentialStatusList) in models.go
  - Store layer: CreateEncKey/GetEncKey/DeleteEncKey, CreateCredential/GetCredentials/RevokeCredential, GetStatusList/UpdateStatusListBit/GetNextStatusIndex
  - credential package: IssueCredential + 6 type wrappers, IssueEmailSDJWT, EncryptCredential/DecryptCredential
  - trust package: DeriveTrustLevel (L0-L3), EffectiveTrust (min)
  - statuslist package: EncodeStatusList/DecodeStatusList (gzip+base64url), SetBit/CheckBit (MSB-first)

affects:
  - 04-02 (API credential endpoints will use this store + credential package)
  - 04-03 (mobile will issue/present credentials from this infrastructure)

# Tech tracking
tech-stack:
  added:
    - github.com/trustbloc/vc-go v1.3.6 (W3C VC 2.0 JWT creation + SD-JWT)
    - github.com/trustbloc/did-go v1.3.5 (transitively, provides utiltime)
    - go.mod replace directive: piprate/json-gold -> trustbloc/json-gold (did-go compatibility)
  patterns:
    - TDD (RED/GREEN/REFACTOR) with in-memory mock stores for all store tests
    - Per-entity AES-256-GCM encryption: nonce prepended, key in separate table for crypto-shredding
    - trustbloc/vc-go ProofCreator via creator.WithJWTAlg(eddsakms.New(), ed25519Signer)
    - jose.Signer adapter struct for MakeSDJWT (separate from jwt.ProofCreator)
    - Bitstring Status List: 16384 bytes = 131072 slots, MSB-first bit addressing

key-files:
  created:
    - platform/migrations/013_credentials.up.sql
    - platform/migrations/013_credentials.down.sql
    - platform/internal/store/credentials.go
    - platform/internal/store/credentials_test.go
    - platform/internal/credential/credential.go
    - platform/internal/credential/credential_test.go
    - platform/internal/credential/trust.go
    - platform/internal/credential/statuslist.go
  modified:
    - platform/internal/models/models.go (added Credential, EncryptionKey, CredentialStatusList, CredentialIDPrefix)
    - platform/go.mod (trustbloc/vc-go, replace directive)
    - platform/go.sum

key-decisions:
  - "Migration numbered 013 (not 012) because 012_revocations.up.sql already exists from Phase 03"
  - "trustbloc/vc-go requires replace directive for piprate/json-gold -> trustbloc/json-gold to resolve did-go ld.MessageDigestAlgorithm build error"
  - "ProofCreator uses WithJWTAlg(eddsakms.New()) — not WithLDProofType — because VC output is JWT not JSON-LD"
  - "jose.Signer adapter (Headers returning EdDSA) required for MakeSDJWT, separate from jwt.ProofCreator"
  - "Store tests use in-memory mock (matching existing project pattern) — no testcontainers needed for unit coverage"
  - "GetNextStatusIndex uses atomic UPDATE RETURNING next_index - 1 for safe concurrent index allocation"

patterns-established:
  - "Credential package lives at platform/internal/credential/ (not credentials/ — matches plan spec)"
  - "IssueCredential takes (entityDID, credType, issuerDID, keyID, pubKey, privKey, subject map, statusIndex, listID) — convenience wrappers add specific subject fields"
  - "AES-256-GCM: gcm.Seal(nonce, nonce, plaintext, nil) — nonce prepended to output, standard Go pattern"
  - "SetBit(bits, idx): bits[idx/8] |= (1 << (7 - uint(idx%8))) — MSB-first per W3C spec"

requirements-completed: [CRD-01, CRD-02, CRD-03, CRD-04, CRD-05, CRD-06, PRV-01]

# Metrics
duration: 11min
completed: 2026-03-16
---

# Phase 4 Plan 01: Verifiable Credentials Foundation Summary

**W3C VC 2.0 JWT issuance for 6 ATAP credential types via trustbloc/vc-go, per-entity AES-256-GCM encryption, Bitstring Status List revocation, SD-JWT selective disclosure, and PostgreSQL store layer with crypto-shredding support**

## Performance

- **Duration:** 11 min
- **Started:** 2026-03-16T13:48:27Z
- **Completed:** 2026-03-16T13:59:30Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments

- Migration 013 creates `entity_enc_keys`, `credentials`, and `credential_status_lists` tables with seed status list (16384-byte bitstring = 131072 credential slots)
- Credential models added to models.go: `Credential`, `EncryptionKey`, `CredentialStatusList`, `CredentialIDPrefix`
- Full store layer: enc key CRUD, credential CRUD, status list bit operations, atomic index allocation
- 6 ATAP VC types issue as W3C VC 2.0 JWTs signed with Ed25519 via trustbloc/vc-go
- SD-JWT selective disclosure for PII credentials (email) using vc.MakeSDJWT
- AES-256-GCM roundtrip correct, wrong-key returns error
- Trust level derivation L0-L3 with highest-wins scan; effective trust = min(entity, server)

## Task Commits

1. **Task 1: DB migration + models + credential store layer** - `53ae12c` (feat — pre-existing commit from previous run)
2. **Task 2: Credential issuance engine, trust level, status list, SD-JWT** - `b51485a` (feat)

## Files Created/Modified

- `platform/migrations/013_credentials.up.sql` - entity_enc_keys, credentials, credential_status_lists tables + seed
- `platform/migrations/013_credentials.down.sql` - drop all three tables
- `platform/internal/models/models.go` - Credential, EncryptionKey, CredentialStatusList types + CredentialIDPrefix
- `platform/internal/store/credentials.go` - 9 store methods for enc keys, credentials, status lists
- `platform/internal/store/credentials_test.go` - 8 test functions, in-memory mock
- `platform/internal/credential/credential.go` - IssueCredential, 6 VC type wrappers, IssueEmailSDJWT, AES-256-GCM helpers
- `platform/internal/credential/credential_test.go` - 23 tests covering all behaviors
- `platform/internal/credential/trust.go` - DeriveTrustLevel, EffectiveTrust
- `platform/internal/credential/statuslist.go` - EncodeStatusList, DecodeStatusList, SetBit, CheckBit
- `platform/go.mod` - trustbloc/vc-go v1.3.6 + replace directive for json-gold

## Decisions Made

- Migration numbered 013 (not 012) because 012_revocations.up.sql already exists from Phase 03.
- trustbloc/vc-go v1.3.6 requires a `replace` directive in go.mod: `piprate/json-gold -> trustbloc/json-gold` because `trustbloc/did-go v1.3.5` references `ld.MessageDigestAlgorithm` only available in the trustbloc fork.
- `ProofCreator` uses `creator.WithJWTAlg(eddsakms.New(), signer)` not `WithLDProofType` — JWT output path does not use JSON-LD descriptors.
- `MakeSDJWT` requires a `jose.Signer` (with `Headers()` method) not a `jwt.ProofCreator` — a separate adapter struct was added.
- Store tests use in-memory mock pattern (matching existing revocations_test.go, key_versions_test.go) — no testcontainers, no DATABASE_URL required.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] go.mod replace directive for trustbloc/did-go json-gold compatibility**
- **Found during:** Task 2 (credential package compilation)
- **Issue:** `trustbloc/did-go v1.3.5` references `ld.MessageDigestAlgorithm` which exists only in `trustbloc/json-gold` fork, not in `piprate/json-gold v0.5.1-*` that the platform's go.mod pulled in directly
- **Fix:** Added `replace github.com/piprate/json-gold ... => github.com/trustbloc/json-gold ...` to go.mod, mirroring the replace directive in did-go's own go.mod
- **Files modified:** platform/go.mod, platform/go.sum
- **Verification:** `go build ./internal/credential/...` succeeds
- **Committed in:** b51485a (Task 2 commit)

**2. [Rule 3 - Blocking] Migration number collision — plan specified 012, used 013**
- **Found during:** Task 1 analysis
- **Issue:** Migration 012_revocations.up.sql already exists (created in Phase 03); plan was written before that migration was added
- **Fix:** Created migration as 013_credentials.up/down.sql
- **Files modified:** platform/migrations/013_credentials.up.sql, platform/migrations/013_credentials.down.sql
- **Verification:** Files exist with correct naming convention
- **Committed in:** 53ae12c (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both auto-fixes were necessary for compilation and correct file naming. No scope creep.

## Issues Encountered

- trustbloc/vc-go SD-JWT `MakeSDJWT` takes `jose.Signer` not `jwt.ProofCreator` — two separate signer adapters needed (one for regular JWTs, one for SD-JWTs). Resolved by implementing both interfaces with thin Ed25519 wrappers.

## Next Phase Readiness

- DB schema and store layer ready for Plan 04-02 (credential API endpoints)
- credential package exports all 6 VC issuers + encrypt/decrypt + trust level derivation
- Status list infrastructure ready for revocation endpoint
- `go.mod` replace directive must be preserved across any future dep updates

---
*Phase: 04-credentials-and-mobile*
*Completed: 2026-03-16*
