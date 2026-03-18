# ATAP Go SDK

Go client library for the [ATAP (Agent Trust and Authority Protocol)](https://atap.dev) platform. Zero external dependencies -- uses only the Go standard library for Ed25519 signing, SHA-256 hashing, and HTTP.

## Installation

```bash
go get github.com/8upio/atap/sdks/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    atap "github.com/8upio/atap/sdks/go"
)

func main() {
    // Create an unauthenticated client for registration.
    client, err := atap.NewClient(
        atap.WithBaseURL("http://localhost:8080"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Register a new agent.
    entity, err := client.Entities.Register(context.Background(), "agent", &atap.RegisterOptions{
        Name: "my-agent",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("DID: %s\n", entity.DID)
    fmt.Printf("Secret: %s\n", entity.ClientSecret)

    // Create an authenticated client.
    authed, err := atap.NewClient(
        atap.WithBaseURL("http://localhost:8080"),
        atap.WithDID(entity.DID),
        atap.WithPrivateKey(entity.PrivateKey),
        atap.WithClientSecret(entity.ClientSecret),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer authed.Close()
}
```

## API Reference

### Client Creation

```go
client, err := atap.NewClient(
    atap.WithBaseURL("http://localhost:8080"),        // ATAP server URL
    atap.WithDID("did:web:example.com:agent:abc"),    // Entity DID
    atap.WithPrivateKey("<base64-ed25519-seed>"),      // Ed25519 private key
    atap.WithSigningKey(privKey),                      // Or pass ed25519.PrivateKey directly
    atap.WithClientSecret("atap_..."),                 // For agent/machine OAuth
    atap.WithScopes([]string{"atap:inbox"}),           // OAuth scopes
    atap.WithPlatformDomain("example.com"),            // DPoP htu domain
    atap.WithTimeout(30 * time.Second),                // HTTP timeout
)
```

### Entities

```go
// Register (public)
entity, err := client.Entities.Register(ctx, "agent", &atap.RegisterOptions{
    Name:      "my-agent",
    PublicKey: "<base64-pubkey>",  // optional
})

// Get (public)
entity, err := client.Entities.Get(ctx, "entity-id")

// Delete (authenticated)
err := client.Entities.Delete(ctx, "entity-id")

// Rotate key (authenticated)
keyVersion, err := client.Entities.RotateKey(ctx, "entity-id", "<new-base64-pubkey>")
```

### Approvals

```go
// Create
approval, err := client.Approvals.Create(ctx, fromDID, toDID, atap.ApprovalSubject{
    Type:  "data_access",
    Label: "Access user data",
}, "")

// Respond
approval, err := client.Approvals.Respond(ctx, "apr_123", "jws-signature")

// List
approvals, err := client.Approvals.List(ctx)

// Revoke
approval, err := client.Approvals.Revoke(ctx, "apr_123")
```

### Revocations

```go
// Submit
revocation, err := client.Revocations.Submit(ctx, "apr_123", "jws-signature", "")

// List (public)
list, err := client.Revocations.List(ctx, "did:web:example.com:human:h1")
```

### DIDComm

```go
// Send (public)
result, err := client.DIDComm.Send(ctx, jweBytes)

// Inbox (authenticated)
inbox, err := client.DIDComm.Inbox(ctx, 50)
```

### Credentials

```go
// Email verification
msg, err := client.Credentials.StartEmailVerification(ctx, "user@example.com")
cred, err := client.Credentials.VerifyEmail(ctx, "user@example.com", "123456")

// Phone verification
msg, err := client.Credentials.StartPhoneVerification(ctx, "+15551234567")
cred, err := client.Credentials.VerifyPhone(ctx, "+15551234567", "654321")

// Personhood
cred, err := client.Credentials.SubmitPersonhood(ctx, "provider-token")

// List (authenticated)
creds, err := client.Credentials.List(ctx)

// Status list (public)
data, err := client.Credentials.StatusList(ctx, "1")
```

### Discovery

```go
// Server discovery
doc, err := client.Discovery.Discover(ctx)

// Resolve DID document
didDoc, err := client.Discovery.ResolveDID(ctx, "agent", "entity-id")

// Server DID
serverDoc, err := client.Discovery.ServerDID(ctx)

// Health check
health, err := client.Discovery.Health(ctx)
```

## Error Handling

All errors follow RFC 7807 Problem Details. Use `errors.As` to check error types:

```go
import "errors"

entity, err := client.Entities.Get(ctx, "nonexistent")
if err != nil {
    var notFound *atap.ATAPNotFoundError
    if errors.As(err, &notFound) {
        fmt.Println("Entity not found")
        if notFound.Problem != nil {
            fmt.Println(notFound.Problem.Detail)
        }
    }

    var authErr *atap.ATAPAuthError
    if errors.As(err, &authErr) {
        fmt.Println("Authentication failed:", authErr.StatusCode)
    }

    var conflict *atap.ATAPConflictError
    if errors.As(err, &conflict) {
        fmt.Println("Conflict:", conflict.Message)
    }

    var rateLimit *atap.ATAPRateLimitError
    if errors.As(err, &rateLimit) {
        fmt.Println("Rate limited, try again later")
    }
}
```

## Configuration

All API methods accept `context.Context` as the first parameter for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

entity, err := client.Entities.Get(ctx, "id")
```

## Cryptographic Utilities

The SDK exposes low-level crypto functions:

```go
// Generate Ed25519 keypair
pub, priv, err := atap.GenerateKeypair()

// Load key from base64 (32-byte seed or 64-byte full key)
privKey, err := atap.LoadSigningKey(base64String)

// JWK Thumbprint (RFC 7638)
thumbprint := atap.JWKThumbprint(pubKey)

// DPoP Proof JWT (RFC 9449)
proof := atap.MakeDPoPProof(privKey, "POST", "https://example.com/token", accessToken)

// PKCE (S256)
verifier, challenge, err := atap.GeneratePKCE()

// Extract domain from DID
domain, err := atap.DomainFromDID("did:web:example.com%3A443:agent:abc")
```

## Examples

See the `examples/` directory:

- `examples/register/` - Register an agent and retrieve it
- `examples/approval-flow/` - Create and manage approvals
- `examples/credentials/` - Email verification and credential management
