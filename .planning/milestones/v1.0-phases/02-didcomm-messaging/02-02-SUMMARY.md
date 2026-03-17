---
phase: 02-didcomm-messaging
plan: 02
subsystem: identity/didcomm
tags: [x25519, didcomm, did-document, key-agreement, migration, server-identity]
dependency_graph:
  requires: [02-01]
  provides: [x25519-entity-keys, didcomm-message-queue-schema, server-did-identity]
  affects: [entity-registration, did-resolution, store-layer]
tech_stack:
  added: [crypto/ecdh, golang.org/x/crypto/hkdf]
  patterns: [X25519KeyAgreementKey2020, DIDCommMessaging-service-endpoint, HKDF-key-derivation]
key_files:
  created:
    - platform/migrations/010_didcomm.up.sql
    - platform/migrations/010_didcomm.down.sql
  modified:
    - platform/internal/models/models.go
    - platform/internal/crypto/did.go
    - platform/internal/crypto/did_test.go
    - platform/internal/api/entities.go
    - platform/internal/api/did.go
    - platform/internal/api/did_test.go
    - platform/internal/store/store.go
    - platform/internal/config/config.go
    - platform/internal/api/api.go
    - platform/cmd/server/main.go
decisions:
  - "[02-02]: Server X25519 key derived deterministically from Ed25519 seed via HKDF — stable across restarts without new env var or DB row"
  - "[02-02]: X25519 verification method appended to verificationMethod array (not a separate array) — single source of truth for DID Document VMs"
  - "[02-02]: Server DID Document uses application/did+json (not +ld+json) — platform identity, not entity identity"
  - "[02-02]: nullableBytes helper added to store for clean NULL handling on optional BYTEA columns"
metrics:
  duration_minutes: 7
  completed_date: "2026-03-13"
  tasks_completed: 3
  files_modified: 10
---

# Phase 02 Plan 02: DIDComm Entity Infrastructure Summary

X25519 key generation at entity registration, DID Document keyAgreement + DIDCommMessaging service endpoint, message queue schema, and deterministic server DIDComm identity via HKDF-derived X25519 key.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Migration 010 — X25519 columns and didcomm_messages table | 23aae87 | migrations/010_didcomm.up.sql, .down.sql |
| 2 | Entity model + DID Document X25519 and DIDComm extension | 703be9c | models.go, did.go, store.go, entities.go, did_test.go |
| 3 | Server DIDComm identity with X25519 keypair | e4726fe | config.go, api.go, did.go, main.go |

## What Was Built

### Migration 010
- Added `x25519_public_key` and `x25519_private_key` BYTEA columns to entities table
- Created `didcomm_messages` table with JWE payload storage, state machine (pending/delivered/expired), expiry tracking
- Indexes on `(recipient_did, state)` and `expires_at WHERE state = 'pending'` for efficient message delivery queries

### Entity X25519 Keys
- Every entity now gets an X25519 keypair generated at registration via `crypto/ecdh`
- Keys persisted to DB alongside Ed25519 key in `CreateEntity`
- All `Get*` store methods updated to load X25519 keys from DB
- `nullableBytes` helper added for clean NULL handling on optional BYTEA columns

### DID Document Extension
- `DIDDocument` struct extended with `KeyAgreement []string` and `Service []DIDService` fields (omitempty)
- New types: `DIDService`, `DIDServiceEndpoint`, `DIDCommMessage`
- `EncodeX25519PublicKeyMultibase` added to crypto package (same "z"+base58btc convention as Ed25519)
- `BuildDIDDocument` conditionally adds X25519 verification method, keyAgreement reference, and DIDCommMessaging service endpoint when entity has an X25519 key
- Backward compatible: entities without X25519 key produce identical DID Documents to before

### Server DIDComm Identity (MSG-03)
- `Config` extended with `PlatformX25519PrivateKey` and `PlatformX25519PublicKey`
- Handler extended with `platformX25519Key *ecdh.PrivateKey`
- `NewHandler` signature updated to accept `platformX25519Key`
- Server X25519 key derived from Ed25519 seed via HKDF (SHA-256, info=`atap-platform-x25519`) at startup — deterministic without separate env var
- New route `GET /server/platform/did.json` returns server DID Document with Ed25519 + X25519 keys and DIDCommMessaging service

## Test Coverage
- `TestBuildDIDDocument_WithX25519`: verifies keyAgreement, X25519KeyAgreementKey2020 VM, DIDCommMessaging service
- `TestEncodeX25519PublicKeyMultibase`: z-prefix, determinism, uniqueness
- `TestBuildDIDDocument/entity_without_X25519_key_has_no_keyAgreement_or_service`: backward compat
- `TestResolveDID/DID_document_with_X25519_key_includes_keyAgreement_and_DIDCommMessaging_service`: HTTP integration test

All 3 existing test packages pass: `internal/api`, `internal/crypto`, `internal/store`.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- FOUND: platform/migrations/010_didcomm.up.sql
- FOUND: platform/migrations/010_didcomm.down.sql
- FOUND: .planning/phases/02-didcomm-messaging/02-02-SUMMARY.md
- FOUND commit 23aae87 (Task 1: migration)
- FOUND commit 703be9c (Task 2: entity/DID extension)
- FOUND commit e4726fe (Task 3: server DIDComm identity)
