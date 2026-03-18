# ATAP PHP SDK

PHP SDK for the [ATAP (Agent Trust and Authority Protocol)](https://atap.dev) platform. Provides verifiable delegation of trust between AI agents, machines, humans, and organizations.

## Requirements

- PHP 8.1+
- ext-sodium (built-in since PHP 7.2)
- Composer

## Installation

```bash
composer require atap/atap-sdk
```

## Quick Start

```php
use Atap\Sdk\ATAPClient;

// Register a new agent
$client = new ATAPClient([
    'base_url' => 'http://localhost:8080',
]);

$entity = $client->entities()->register('agent', 'My AI Agent');
echo "Agent DID: {$entity->did}\n";
echo "Client Secret: {$entity->clientSecret}\n";

// Create an authenticated client
$authedClient = new ATAPClient([
    'base_url' => 'http://localhost:8080',
    'did' => $entity->did,
    'private_key' => $entity->privateKey,
    'client_secret' => $entity->clientSecret,
]);

// Use the API
$health = $authedClient->discovery()->health();
```

## Configuration

```php
$client = new ATAPClient([
    'base_url' => 'http://localhost:8080',     // ATAP server URL
    'did' => 'did:web:...',                     // Entity DID
    'private_key' => '<base64>',                // Base64-encoded Ed25519 seed (32 bytes) or full key (64 bytes)
    'client_secret' => 'atap_...',              // Client secret for client_credentials grant
    'scopes' => ['atap:inbox', 'atap:send'],    // OAuth scopes (default: all)
    'platform_domain' => 'example.com',         // Override platform domain for DPoP
    'timeout' => 30.0,                          // HTTP timeout in seconds
]);
```

## API Reference

### Entities

```php
// Register a new entity
$entity = $client->entities()->register('agent', 'My Agent', $publicKey, $principalDid);

// Get entity by ID
$entity = $client->entities()->get('entity_id');

// Delete entity (requires auth)
$client->entities()->delete('entity_id');

// Rotate key (requires auth)
$keyVersion = $client->entities()->rotateKey('entity_id', $newPublicKeyB64);
```

### Approvals

```php
use Atap\Sdk\Model\ApprovalSubject;

// Create approval
$subject = new ApprovalSubject('data_access', 'Access user data', payload: ['scope' => 'read']);
$approval = $client->approvals()->create($fromDid, $toDid, $subject);

// Respond to approval
$approval = $client->approvals()->respond('apr_...', $jwsSignature);

// List approvals
$approvals = $client->approvals()->list();

// Revoke approval
$approval = $client->approvals()->revoke('apr_...');
```

### Revocations

```php
// Submit revocation
$revocation = $client->revocations()->submit('apr_...', $signature, $validUntil);

// List revocations (public)
$list = $client->revocations()->list('did:web:...');
```

### DIDComm

```php
// Send message (public)
$result = $client->didcomm()->send($jweBytes);

// Read inbox (requires auth)
$inbox = $client->didcomm()->inbox(limit: 50);
```

### Credentials

```php
// Email verification
$client->credentials()->startEmail('user@example.com');
$cred = $client->credentials()->verifyEmail('user@example.com', '123456');

// Phone verification
$client->credentials()->startPhone('+1234567890');
$cred = $client->credentials()->verifyPhone('+1234567890', '654321');

// Personhood attestation
$cred = $client->credentials()->submitPersonhood($providerToken);

// List credentials
$creds = $client->credentials()->list();

// Public status list
$statusList = $client->credentials()->statusList('1');
```

### Discovery

```php
// Server discovery
$doc = $client->discovery()->discover();

// Resolve DID document
$didDoc = $client->discovery()->resolveDid('agent', 'entity_id');

// Server DID
$serverDid = $client->discovery()->serverDid();

// Health check
$health = $client->discovery()->health();
```

## Error Handling

All API errors throw typed exceptions:

```php
use Atap\Sdk\Exception\ATAPException;
use Atap\Sdk\Exception\ATAPAuthException;
use Atap\Sdk\Exception\ATAPNotFoundException;
use Atap\Sdk\Exception\ATAPConflictException;
use Atap\Sdk\Exception\ATAPRateLimitException;
use Atap\Sdk\Exception\ATAPProblemException;

try {
    $entity = $client->entities()->get('nonexistent');
} catch (ATAPNotFoundException $e) {
    echo "Not found: {$e->getMessage()}\n";
    echo "Problem detail: {$e->problem?->detail}\n";
} catch (ATAPAuthException $e) {
    echo "Auth error ({$e->statusCode}): {$e->getMessage()}\n";
} catch (ATAPRateLimitException $e) {
    echo "Rate limited, try again later\n";
} catch (ATAPException $e) {
    echo "ATAP error: {$e->getMessage()}\n";
}
```

## Cryptography

The SDK uses Ed25519 (via ext-sodium) for all cryptographic operations:

```php
use Atap\Sdk\Crypto\Ed25519;
use Atap\Sdk\Crypto\DPoPProof;
use Atap\Sdk\Crypto\JWKThumbprint;

// Generate keypair
$keys = Ed25519::generateKeypair();
$publicKeyB64 = base64_encode($keys['publicKey']);

// Sign data
$signature = Ed25519::sign('data', $keys['secretKey']);
$valid = Ed25519::verify($signature, 'data', $keys['publicKey']);

// JWK thumbprint
$thumbprint = JWKThumbprint::compute($keys['publicKey']);

// DPoP proof
$proof = DPoPProof::create($keys['secretKey'], 'POST', 'https://example.com/token');
```

## License

Apache-2.0
