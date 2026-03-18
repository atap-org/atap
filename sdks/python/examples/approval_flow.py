"""Example: Create and respond to an approval request."""

from atap import ATAPClient, ApprovalSubject

# Requester: create an approval
requester = ATAPClient(
    base_url="http://localhost:8080",
    did="did:web:localhost%3A8080:agent:requester",
    private_key="<base64 private key>",
    client_secret="atap_...",
)

approval = requester.approvals.create(
    from_did="did:web:localhost%3A8080:agent:requester",
    to_did="did:web:localhost%3A8080:human:approver",
    subject=ApprovalSubject(
        type="com.example.payment",
        label="Payment of $100 to Acme Corp",
        payload={"amount": 100, "currency": "USD", "recipient": "Acme Corp"},
    ),
)
print(f"Approval created: {approval.id} (state: {approval.state})")

# Approver: respond to the approval
approver = ATAPClient(
    base_url="http://localhost:8080",
    did="did:web:localhost%3A8080:human:approver",
    private_key="<base64 private key>",
)
# For human entities, use authorization_code flow:
approver.token_manager.obtain_authorization_code()

# List pending approvals
pending = approver.approvals.list()
for a in pending:
    print(f"  [{a.state}] {a.id}: {a.subject.label if a.subject else 'N/A'}")

# Approve the first one
if pending:
    result = approver.approvals.respond(pending[0].id, signature="<JWS signature>")
    print(f"Responded: {result.id} -> {result.state}")

requester.close()
approver.close()
