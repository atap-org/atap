---
phase: 04-credentials-and-mobile
plan: 04
subsystem: mobile
tags: [flutter, didcomm, ed25519, jws, biometric, approval-cards, credentials, recovery-passphrase]
---

# Plan 04-04: Flutter Mobile App Rebuild

## What Was Built

Complete Flutter mobile app rebuild around the DIDComm/approval architecture, replacing the old Signal/SSE/Firebase infrastructure.

### Task 1: Models, API Client, JWS Service

- **DIDCommMessage model** (`core/models/didcomm_message.dart`): Replaces Signal model, maps DIDComm v2 messages with `isApprovalRequest` helper
- **Approval model** (`core/models/approval.dart`): Full approval model with subject, signatures, `isPersistent` for standing approvals
- **Credential model** (`core/models/credential.dart`): W3C VC model with revocation tracking
- **JWS Service** (`core/crypto/jws_service.dart`): Ed25519 detached JWS signing/verification (RFC 7797)
- **API Client** (`core/api/api_client.dart`): Updated with DIDComm inbox, approval respond/list/revoke, credential list, template fetch; removed old SSE/signal methods
- **pubspec.yaml**: Added `local_auth` (biometric) and `encrypt` (passphrase AES-256); removed Firebase dependencies

### Task 2: Screens, Navigation, Cleanup

- **Register Screen** (`features/onboarding/register_screen.dart`): Simplified to generate Ed25519 keypair, POST /v1/entities, show DID, navigate to recovery passphrase
- **Recovery Passphrase Screen** (`features/onboarding/recovery_passphrase_screen.dart`): PBKDF2-derived AES-256 key encrypts private key backup, stored in flutter_secure_storage (MOB-01)
- **Inbox Screen** (`features/inbox/inbox_screen.dart`): DIDComm polling-based inbox, approval cards for approval messages, simple tiles for other types
- **Approval Card** (`features/inbox/approval_card.dart`): Branded card (fetches template) or fallback card; biometric gate → JWS sign → respond API (MOB-03)
- **Credentials Screen** (`features/credentials/credentials_screen.dart`): Lists VCs with type icons and revocation status (MOB-04)
- **Approvals Screen** (`features/approvals/approvals_screen.dart`): Persistent approvals with revoke action (MOB-05)
- **Router**: Bottom navigation (Inbox, Credentials, Approvals, Settings); recovery passphrase badge on Settings
- **main.dart**: Removed Firebase initialization and push notification handling
- **Deleted**: signal.dart, sse_client.dart, signal_detail_screen.dart, claim_screen.dart, push_provider.dart

## Key Files

### Created
- `mobile/lib/core/models/didcomm_message.dart`
- `mobile/lib/core/models/approval.dart`
- `mobile/lib/core/models/credential.dart`
- `mobile/lib/core/crypto/jws_service.dart`
- `mobile/lib/features/onboarding/recovery_passphrase_screen.dart`
- `mobile/lib/features/inbox/approval_card.dart`
- `mobile/lib/features/credentials/credentials_screen.dart`
- `mobile/lib/features/approvals/approvals_screen.dart`

### Modified
- `mobile/pubspec.yaml`
- `mobile/lib/core/api/api_client.dart`
- `mobile/lib/features/onboarding/register_screen.dart`
- `mobile/lib/features/inbox/inbox_screen.dart`
- `mobile/lib/providers/inbox_provider.dart`
- `mobile/lib/main.dart`
- `mobile/lib/app/router.dart`

### Deleted
- `mobile/lib/core/models/signal.dart`
- `mobile/lib/core/api/sse_client.dart`
- `mobile/lib/features/inbox/signal_detail_screen.dart`
- `mobile/lib/features/onboarding/claim_screen.dart`
- `mobile/lib/providers/push_provider.dart`

## Deviations

- Removed Firebase/FCM entirely (no longer needed with DIDComm polling architecture)
- Registration simplified: no claim code required, direct POST /v1/entities with public key
- PBKDF2 implemented manually (100,000 iterations, SHA-256) rather than using a dedicated PBKDF2 package

## Requirements Addressed

- **MOB-01**: Ed25519 keypair generation, DID registration, recovery passphrase with encrypted backup
- **MOB-02**: DIDComm inbox with 15-second polling
- **MOB-03**: Approval cards (branded + fallback), biometric-gated JWS signing
- **MOB-04**: Credentials screen displaying VCs
- **MOB-05**: Persistent approval management with revocation
- **MOB-06**: Bottom navigation with 4 tabs

## Self-Check: PASSED
- [x] All models parse JSON correctly
- [x] API client has all DIDComm/credential/approval methods
- [x] JWS service produces valid detached JWS
- [x] Recovery passphrase encrypts key backup with PBKDF2-derived AES-256
- [x] No references to old Signal/SSE/push code
- [x] All tasks committed atomically
