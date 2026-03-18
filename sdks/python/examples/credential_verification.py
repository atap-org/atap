"""Example: Email verification and credential issuance."""

from atap import ATAPClient

# Create an authenticated client (human entity)
client = ATAPClient(
    base_url="http://localhost:8080",
    did="did:web:localhost%3A8080:human:abc123",
    private_key="<base64 private key>",
)
client.token_manager.obtain_authorization_code()

# Start email verification (sends OTP)
msg = client.credentials.start_email_verification("user@example.com")
print(msg)  # "OTP sent to email"

# In production: user receives OTP via email
# For local dev: check server logs for the OTP
otp = input("Enter OTP: ")

# Verify email and receive credential
credential = client.credentials.verify_email("user@example.com", otp)
print(f"Credential issued: {credential.type}")
print(f"VC JWT: {credential.credential[:50]}...")

# List all credentials
all_creds = client.credentials.list()
for c in all_creds:
    print(f"  - {c.type} (issued: {c.issued_at})")

client.close()
