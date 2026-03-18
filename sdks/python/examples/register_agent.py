"""Example: Register an agent and make authenticated API calls."""

from atap import ATAPClient

# Step 1: Register a new agent (no auth required)
client = ATAPClient(base_url="http://localhost:8080")

entity = client.entities.register("agent", name="my-travel-agent")
print(f"Entity ID:  {entity.id}")
print(f"DID:        {entity.did}")
print(f"Secret:     {entity.client_secret}")
print(f"Private Key:{entity.private_key}")

# Step 2: Create an authenticated client
authed = ATAPClient(
    base_url="http://localhost:8080",
    did=entity.did,
    private_key=entity.private_key,
    client_secret=entity.client_secret,
)

# Step 3: List approvals (requires atap:inbox scope)
approvals = authed.approvals.list()
print(f"\nPending approvals: {len(approvals)}")

# Step 4: Check DIDComm inbox
inbox = authed.didcomm.inbox()
print(f"Messages in inbox: {inbox.count}")

# Clean up
authed.close()
client.close()
