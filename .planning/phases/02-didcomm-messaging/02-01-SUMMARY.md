---
phase: 02-didcomm-messaging
plan: "01"
subsystem: didcomm
tags: [didcomm, jwe, ecdh-1pu, a256cbc-hs512, crypto, message-types]
dependency_graph:
  requires: []
  provides: [didcomm-envelope-encrypt, didcomm-envelope-decrypt, didcomm-message-types, atap-protocol-constants]
  affects: [platform/internal/didcomm]
tech_stack:
  added: []
  patterns:
    - "ECDH-1PU+A256KW key agreement with tag-in-KDF (Ze||Zs||tag before ConcatKDF)"
    - "A256CBC-HS512 content encryption (AES-CBC + HMAC-SHA512 truncated to 256 bits)"
    - "RFC 3394 AES Key Wrap/Unwrap for CEK wrapping"
    - "NIST SP 800-56C ConcatKDF with SHA-512 for wrapping key derivation"
    - "DIDComm v2.1 JWE JSON serialization with OKP ephemeral public key"
    - "TDD: RED (failing tests) → GREEN (implementation) per task"
key_files:
  created:
    - platform/internal/didcomm/message.go
    - platform/internal/didcomm/message_test.go
    - platform/internal/didcomm/envelope.go
    - platform/internal/didcomm/envelope_test.go
  modified: []
decisions:
  - "Used crypto/ecdh.X25519() (stdlib) for all X25519 ECDH — go-jose v4 does not support X25519 (confirmed Pitfall 6)"
  - "Implemented ConcatKDF inline (~25 lines) rather than golang-crypto/concatkdf (v0.x, low-maintenance)"
  - "tag-in-KDF: ciphertext tag appended to Z = Ze||Zs||tag BEFORE ConcatKDF, per ECDH-1PU draft v4 Pitfall 2"
  - "apv = base64url(sha256(recipientKID)) per DIDComm v2.1 spec for single-recipient JWE"
  - "A256CBC-HS512 key split: key[0:32]=MAC key, key[32:64]=AES enc key, HMAC-SHA512 truncated to 256 bits"
metrics:
  duration_minutes: 4
  completed_date: "2026-03-13"
  tasks_completed: 2
  files_created: 4
  files_modified: 0
---

# Phase 2 Plan 01: DIDComm Crypto Envelope and Message Types Summary

**One-liner:** ECDH-1PU+A256KW/A256CBC-HS512 JWE authcrypt with tag-in-KDF plus DIDComm v2.1 plaintext message types and 8 ATAP protocol type constants.

## What Was Built

### Task 1: DIDComm Message Types and ATAP Protocol Constants

`platform/internal/didcomm/message.go` — new package with:

- `PlaintextMessage` struct matching DIDComm v2.1 spec JSON field names (`id`, `type`, `from`, `to`, `created_time`, `expires_time`, `thid`, `pthid`, `body`, `attachments`)
- `Attachment` and `AttachmentData` types
- Three DIDComm content type constants (`ContentTypePlain`, `ContentTypeSigned`, `ContentTypeEncrypted`)
- Eight ATAP protocol type constants under `https://atap.dev/protocols/`:
  - Approval lifecycle: `TypeApprovalRequest`, `TypeApprovalResponse`, `TypeApprovalRevoke`, `TypeApprovalStatus`, `TypeApprovalRejected`
  - System: `TypePing`, `TypePong`, `TypeProblemReport`
- `NewMessage()` helper generating `msg_` + ULID IDs with `crypto/rand` entropy and current Unix timestamp

### Task 2: ECDH-1PU+A256KW / A256CBC-HS512 JWE Envelope

`platform/internal/didcomm/envelope.go` — full JWE authcrypt implementation:

- `GenerateX25519KeyPair()` — `crypto/ecdh.X25519().GenerateKey()`
- `Encrypt(plaintext, senderPriv, senderPub, recipientPub, senderKID, recipientKID)` — full ECDH-1PU authcrypt
- `Decrypt(jwe, recipientPriv, senderPub)` — full JWE decryption with authentication
- Internal helpers: `concatKDF`, `aesKeyWrap`, `aesKeyUnwrap`, `encryptA256CBCHS512`, `decryptA256CBCHS512`, `pkcs7Pad`, `pkcs7Unpad`

**Critical implementation detail (tag-in-KDF):** Per IETF draft-madden-jose-ecdh-1pu-04, the ciphertext authentication tag must be included in `Z = Ze || Zs || tag` BEFORE running ConcatKDF for the wrapping key. This reverses the naive JWE construction order: encrypt content first → extract tag → then wrap CEK.

## Tests

All 11 tests pass:
- `TestPlaintextMessageMarshal` — correct JSON field names and body structure
- `TestPlaintextMessageOmitsEmptyOptionals` — optional fields absent when zero
- `TestATAPProtocolTypeConstants` — all 8 constants start with `https://atap.dev/protocols/`
- `TestContentTypeConstants` — exact DIDComm spec values
- `TestNewMessage` — msg_ ULID prefix, from/to/body, created_time in expected range
- `TestNewMessageUniqueIDs` — 100 unique IDs generated
- `TestEnvelopeRoundTrip` — `Decrypt(Encrypt(msg)) == msg`
- `TestEnvelopeJWEStructure` — alg/enc/skid/epk in protected header, recipient kid correct
- `TestEnvelopeWrongRecipientKey` — returns error
- `TestEnvelopeWrongSenderKey` — returns error (authentication failure via tag-in-KDF)
- `TestEnvelopeMultiplePlaintexts` — empty, 1 byte, JSON, 4KB all round-trip correctly
- `TestGenerateX25519KeyPair` — 32-byte keys, uniqueness

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 RED | `5b3aa26` | test(02-01): failing tests for message types |
| Task 1 GREEN | `7877096` | feat(02-01): message types and ATAP protocol constants |
| Task 2 RED | `4d8668a` | test(02-01): failing tests for JWE envelope |
| Task 2 GREEN | `ed684b6` | feat(02-01): ECDH-1PU+A256KW / A256CBC-HS512 JWE envelope |

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED
