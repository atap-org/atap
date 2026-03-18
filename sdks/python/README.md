# ATAP Python SDK

Python client for the [ATAP](https://atap.dev) (Agent Trust and Authority Protocol) platform.

## Installation

```bash
pip install atap-sdk
```

## Quick Start

```python
from atap import ATAPClient

# Register a new agent
client = ATAPClient(base_url="http://localhost:8080")
entity = client.entities.register("agent", name="my-agent")

# Create an authenticated client
authed = ATAPClient(
    base_url="http://localhost:8080",
    did=entity.did,
    private_key=entity.private_key,
    client_secret=entity.client_secret,
)

# List approvals
approvals = authed.approvals.list()

# Check DIDComm inbox
inbox = authed.didcomm.inbox()
```

## Authentication

The SDK handles OAuth 2.1 + DPoP authentication automatically.

**Agent/Machine** (client_credentials grant):
```python
client = ATAPClient(
    base_url="http://localhost:8080",
    did="did:web:localhost%3A8080:agent:abc",
    private_key="<base64 Ed25519 seed>",
    client_secret="atap_...",
)
# Tokens are obtained and refreshed automatically
```

**Human/Org** (authorization_code + PKCE):
```python
client = ATAPClient(
    base_url="http://localhost:8080",
    did="did:web:localhost%3A8080:human:abc",
    private_key="<base64 Ed25519 seed>",
)
client.token_manager.obtain_authorization_code()
```

## API Reference

### Entities

```python
# Register
entity = client.entities.register("agent", name="my-agent")
entity = client.entities.register("human", public_key="<base64 pubkey>")

# Get
entity = client.entities.get("entity_id")

# Delete (requires atap:manage)
client.entities.delete("entity_id")

# Rotate key (requires atap:manage)
kv = client.entities.rotate_key("entity_id", "<base64 new pubkey>")
```

### Approvals

```python
from atap import ApprovalSubject

# Create (requires atap:send)
approval = client.approvals.create(
    from_did="did:web:...:agent:requester",
    to_did="did:web:...:human:approver",
    subject=ApprovalSubject(
        type="com.example.payment",
        label="Payment of $100",
        payload={"amount": 100},
    ),
)

# Respond (requires atap:send)
result = client.approvals.respond("apr_...", signature="<JWS>")

# List (requires atap:inbox)
approvals = client.approvals.list()

# Revoke (requires atap:revoke)
client.approvals.revoke("apr_...")
```

### Revocations

```python
# Submit (requires atap:revoke)
rev = client.revocations.submit("apr_...", signature="<JWS>")

# List (public)
result = client.revocations.list("did:web:...:agent:abc")
```

### DIDComm

```python
# Send message (public)
client.didcomm.send(jwe_bytes)

# Check inbox (requires atap:inbox)
inbox = client.didcomm.inbox(limit=50)
for msg in inbox.messages:
    print(f"From: {msg.sender_did}, Type: {msg.message_type}")
```

### Credentials

```python
# Email verification (requires atap:manage)
client.credentials.start_email_verification("user@example.com")
cred = client.credentials.verify_email("user@example.com", "123456")

# Phone verification (requires atap:manage)
client.credentials.start_phone_verification("+1234567890")
cred = client.credentials.verify_phone("+1234567890", "654321")

# Personhood (requires atap:manage)
cred = client.credentials.submit_personhood()

# List credentials (requires atap:manage)
creds = client.credentials.list()

# Status list (public)
status = client.credentials.status_list("1")
```

### Discovery

```python
# Server discovery
doc = client.discovery.discover()

# DID resolution
did_doc = client.discovery.resolve_did("agent", "entity_id")

# Server DID
server_did = client.discovery.server_did()

# Health check
health = client.discovery.health()
```

## Error Handling

```python
from atap import ATAPError, ATAPProblemError
from atap.errors import ATAPAuthError, ATAPNotFoundError, ATAPRateLimitError

try:
    entity = client.entities.get("missing")
except ATAPNotFoundError as e:
    print(f"Not found: {e}")
except ATAPAuthError as e:
    print(f"Auth error ({e.status_code}): {e}")
except ATAPRateLimitError as e:
    print(f"Rate limited: {e}")
except ATAPProblemError as e:
    print(f"API error: {e.problem.title} - {e.problem.detail}")
except ATAPError as e:
    print(f"Error: {e}")
```

## Configuration

```python
client = ATAPClient(
    base_url="http://localhost:8080",      # HTTP target
    did="did:web:...",                      # Entity DID
    private_key="<base64>",                 # Ed25519 private key
    client_secret="atap_...",               # For agent/machine auth
    scopes=["atap:inbox", "atap:send"],     # OAuth scopes
    platform_domain="api.atap.app",         # Domain for DPoP htu
    timeout=30.0,                           # Request timeout (seconds)
)
```

## Examples

See the [examples/](examples/) directory:
- [register_agent.py](examples/register_agent.py) — Register and authenticate
- [approval_flow.py](examples/approval_flow.py) — Create and respond to approvals
- [credential_verification.py](examples/credential_verification.py) — Email verification

## Development

```bash
pip install -e ".[dev]"
pytest --cov=atap
ruff check src/
```

## License

Apache-2.0
