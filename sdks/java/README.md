# ATAP Java SDK

Java SDK for the [Agent Trust and Authority Protocol (ATAP)](https://atap.dev) platform.

## Installation

### Maven

```xml
<dependency>
    <groupId>dev.atap</groupId>
    <artifactId>atap-sdk</artifactId>
    <version>0.1.0</version>
</dependency>
```

### Gradle

```groovy
implementation 'dev.atap:atap-sdk:0.1.0'
```

## Quick Start

```java
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.model.Entity;

// Generate a keypair
Ed25519KeyPair keyPair = Ed25519KeyPair.generate();

// Register an agent
ATAPClient client = ATAPClient.builder()
    .baseUrl("http://localhost:8080")
    .build();

Entity entity = client.entities.register("agent", "My Agent",
    keyPair.getPublicKeyBase64(), null);

System.out.println("DID: " + entity.getDid());
System.out.println("Secret: " + entity.getClientSecret());

// Create an authenticated client
ATAPClient authed = ATAPClient.builder()
    .baseUrl("http://localhost:8080")
    .did(entity.getDid())
    .keyPair(keyPair)
    .clientSecret(entity.getClientSecret())
    .build();
```

## API Reference

### ATAPClient

The main entry point, configured via the builder pattern:

```java
ATAPClient client = ATAPClient.builder()
    .baseUrl("http://localhost:8080")       // ATAP server URL
    .did("did:web:example:agent:abc")       // Entity DID
    .privateKey("<base64 Ed25519 seed>")    // Or use .keyPair(kp)
    .clientSecret("atap_...")               // For client_credentials
    .platformDomain("example.com")          // Optional, extracted from DID
    .timeout(Duration.ofSeconds(30))        // HTTP timeout
    .scopes(List.of("atap:inbox"))          // OAuth scopes
    .build();
```

### Entities

```java
// Register
Entity e = client.entities.register("agent", "Name", publicKeyB64, principalDid);

// Get
Entity e = client.entities.get("entity-id");

// Delete
client.entities.delete("entity-id");

// Rotate key
KeyVersion kv = client.entities.rotateKey("entity-id", newPublicKeyB64);
```

### Approvals

```java
// Create
ApprovalSubject subject = new ApprovalSubject("data_access", "Label", false, payload);
Approval a = client.approvals.create(fromDid, toDid, subject);

// Respond
Approval a = client.approvals.respond("apr_id", "jws_signature");

// List
List<Approval> list = client.approvals.list();

// Revoke
client.approvals.revoke("apr_id");
```

### Revocations

```java
// Submit
Revocation r = client.revocations.submit("apr_id", "signature");

// List (public endpoint)
RevocationList rl = client.revocations.list("did:web:example:agent:a");
```

### DIDComm

```java
// Send message
Map<String, Object> result = client.didcomm.send(jweBytes);

// Read inbox
DIDCommInbox inbox = client.didcomm.inbox(50);
```

### Credentials

```java
// Email verification
client.credentials.startEmail("user@example.com");
Credential c = client.credentials.verifyEmail("user@example.com", "123456");

// Phone verification
client.credentials.startPhone("+15551234567");
Credential c = client.credentials.verifyPhone("+15551234567", "654321");

// Personhood
Credential c = client.credentials.submitPersonhood();

// List
List<Credential> creds = client.credentials.list();

// Status list (public)
Map<String, Object> status = client.credentials.statusList("1");
```

### Discovery

```java
// Server discovery
DiscoveryDocument doc = client.discovery.discover();

// Resolve DID
DIDDocument did = client.discovery.resolveDid("agent", "entity-id");

// Server DID
DIDDocument serverDid = client.discovery.serverDid();

// Health check
Map<String, Object> health = client.discovery.health();
```

## Error Handling

The SDK throws typed exceptions for different error conditions:

```java
try {
    client.entities.get("nonexistent");
} catch (ATAPNotFoundException e) {
    System.out.println("Not found: " + e.getMessage());
    ProblemDetail problem = e.getProblem(); // RFC 7807
} catch (ATAPAuthException e) {
    System.out.println("Auth error: " + e.getStatusCode());
} catch (ATAPConflictException e) {
    // 409
} catch (ATAPRateLimitException e) {
    // 429
} catch (ATAPProblemException e) {
    // Other problem detail responses
} catch (ATAPException e) {
    // Base exception
}
```

## Configuration

| Builder Method | Default | Description |
|---|---|---|
| `baseUrl` | `http://localhost:8080` | ATAP server HTTP URL |
| `did` | `""` | Entity DID (OAuth client_id) |
| `privateKey` | `null` | Base64 Ed25519 seed (32 or 64 bytes) |
| `keyPair` | `null` | Pre-loaded Ed25519KeyPair |
| `clientSecret` | `null` | OAuth client secret |
| `scopes` | all scopes | OAuth scope list |
| `platformDomain` | from DID | Domain for DPoP htu |
| `timeout` | 30 seconds | HTTP request timeout |

## Examples

See the `examples/` directory for complete working examples:

- `RegisterAgent.java` -- Agent registration flow
- `ApprovalFlow.java` -- Creating and managing approvals
- `CredentialVerification.java` -- Email/phone verification

## Requirements

- Java 11+
- BouncyCastle (bcprov-jdk18on) for Ed25519
- Jackson for JSON serialization

## Building

```bash
mvn clean test        # Run tests
mvn verify            # Run tests + coverage check (80% minimum)
mvn package           # Build JAR
```
