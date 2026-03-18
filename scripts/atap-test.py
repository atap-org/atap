#!/usr/bin/env python3
"""ATAP test helper — register entities, get DPoP tokens, call API endpoints.

Usage:
  # Register an agent (server generates keypair)
  python3 scripts/atap-test.py register --type agent --name "my-agent"

  # Register with your own key
  python3 scripts/atap-test.py register --type agent --name "my-agent" --private-key <base64>

  # Get a DPoP-bound access token
  python3 scripts/atap-test.py token --did <did> --secret <client_secret> --private-key <base64> --scope atap:revoke

  # Call an authenticated endpoint
  python3 scripts/atap-test.py call POST /v1/revocations --did <did> --token <access_token> --private-key <base64> --data '{"approval_id":"apr_test123","signature":"sig"}'

  # Call a public endpoint
  python3 scripts/atap-test.py call GET /v1/revocations?entity=did:web:localhost:agent:abc

  # Full flow: register + token + revoke in one shot
  python3 scripts/atap-test.py demo

Requires: pip install PyNaCl PyJWT
"""

import argparse
import base64
import hashlib
import json
import sys
import time
import urllib.request
import urllib.error
import uuid

try:
    import nacl.signing
    import nacl.encoding
except ImportError:
    print("Missing dependency: pip install PyNaCl", file=sys.stderr)
    sys.exit(1)

try:
    import jwt
except ImportError:
    print("Missing dependency: pip install PyJWT", file=sys.stderr)
    sys.exit(1)


BASE_URL = "http://localhost:8080"


def b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def generate_keypair():
    """Generate Ed25519 keypair, return (signing_key, verify_key, seed_b64, pub_b64)."""
    sk = nacl.signing.SigningKey.generate()
    vk = sk.verify_key
    seed_b64 = base64.b64encode(bytes(sk)).decode()
    pub_b64 = base64.b64encode(bytes(vk)).decode()
    return sk, vk, seed_b64, pub_b64


def load_signing_key(private_key_b64: str) -> nacl.signing.SigningKey:
    """Load Ed25519 signing key from base64. Accepts 32-byte seed or 64-byte full key."""
    raw = base64.b64decode(private_key_b64)
    if len(raw) == 64:
        # Full Ed25519 key (seed + public), take seed
        return nacl.signing.SigningKey(raw[:32])
    elif len(raw) == 32:
        return nacl.signing.SigningKey(raw)
    else:
        raise ValueError(f"Invalid private key length: {len(raw)} bytes (expected 32 or 64)")


def jwk_thumbprint(verify_key: nacl.signing.VerifyKey) -> str:
    """Compute JWK thumbprint (RFC 7638) for an Ed25519 public key."""
    x = b64url_encode(bytes(verify_key))
    # Canonical JWK for thumbprint: sorted keys
    canonical = json.dumps({"crv": "Ed25519", "kty": "OKP", "x": x}, separators=(",", ":"), sort_keys=True)
    digest = hashlib.sha256(canonical.encode()).digest()
    return b64url_encode(digest)


def make_dpop_proof(signing_key: nacl.signing.SigningKey, method: str, url: str, access_token: str = None) -> str:
    """Create a DPoP proof JWT (RFC 9449)."""
    verify_key = signing_key.verify_key
    x = b64url_encode(bytes(verify_key))

    header = {
        "typ": "dpop+jwt",
        "alg": "EdDSA",
        "jwk": {
            "kty": "OKP",
            "crv": "Ed25519",
            "x": x,
        },
    }

    payload = {
        "jti": str(uuid.uuid4()),
        "htm": method,
        "htu": url,
        "iat": int(time.time()),
    }

    if access_token:
        ath = hashlib.sha256(access_token.encode("ascii")).digest()
        payload["ath"] = b64url_encode(ath)

    # Encode header and payload
    header_b64 = b64url_encode(json.dumps(header, separators=(",", ":")).encode())
    payload_b64 = b64url_encode(json.dumps(payload, separators=(",", ":")).encode())
    signing_input = f"{header_b64}.{payload_b64}".encode()

    # Sign with Ed25519
    signed = signing_key.sign(signing_input)
    signature = signed.signature  # 64 bytes
    sig_b64 = b64url_encode(signature)

    return f"{header_b64}.{payload_b64}.{sig_b64}"


def http_request(method: str, path: str, headers: dict = None, data: dict = None) -> dict:
    """Make an HTTP request and return parsed JSON response."""
    url = f"{BASE_URL}{path}"
    body = None

    if data is not None:
        if headers and headers.get("Content-Type") == "application/x-www-form-urlencoded":
            body = urllib.parse.urlencode(data).encode()
        else:
            body = json.dumps(data).encode()
            if headers is None:
                headers = {}
            headers.setdefault("Content-Type", "application/json")

    req = urllib.request.Request(url, data=body, headers=headers or {}, method=method)

    try:
        with urllib.request.urlopen(req) as resp:
            resp_body = resp.read().decode()
            return {
                "status": resp.status,
                "body": json.loads(resp_body) if resp_body else {},
            }
    except urllib.error.HTTPError as e:
        resp_body = e.read().decode()
        try:
            parsed = json.loads(resp_body)
        except json.JSONDecodeError:
            parsed = {"raw": resp_body}
        return {"status": e.code, "body": parsed}


def cmd_register(args):
    """Register an entity."""
    data = {"type": args.type, "name": args.name}

    if args.principal_did:
        data["principal_did"] = args.principal_did

    if args.private_key:
        sk = load_signing_key(args.private_key)
        pub_b64 = base64.b64encode(bytes(sk.verify_key)).decode()
        data["public_key"] = pub_b64

    result = http_request("POST", "/v1/entities", data=data)
    print(json.dumps(result, indent=2))

    if result["status"] == 201:
        print(f"\n--- Save these credentials ---")
        print(f"DID:           {result['body'].get('did')}")
        print(f"ID:            {result['body'].get('id')}")
        if result["body"].get("client_secret"):
            print(f"Client Secret: {result['body']['client_secret']}")
        if result["body"].get("private_key"):
            print(f"Private Key:   {result['body']['private_key']}")


def domain_from_did(did: str) -> str:
    """Extract platform domain from a DID. did:web:localhost%3A8080:agent:... -> localhost:8080"""
    parts = did.split(":")
    domain = parts[2]
    # Per did:web spec, ports are percent-encoded as %3A
    domain = domain.replace("%3A", ":")
    return domain


def cmd_token(args):
    """Get a DPoP-bound access token."""
    sk = load_signing_key(args.private_key)
    domain = domain_from_did(args.did)
    token_url = f"https://{domain}/v1/oauth/token"
    dpop_proof = make_dpop_proof(sk, "POST", token_url)

    import urllib.parse

    form_data = {
        "grant_type": "client_credentials",
        "client_id": args.did,
        "client_secret": args.secret,
    }
    if args.scope:
        form_data["scope"] = args.scope

    body = urllib.parse.urlencode(form_data).encode()

    req = urllib.request.Request(
        f"{BASE_URL}/v1/oauth/token",
        data=body,
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "DPoP": dpop_proof,
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req) as resp:
            result = json.loads(resp.read().decode())
            print(json.dumps({"status": resp.status, "body": result}, indent=2))
            if "access_token" in result:
                print(f"\n--- Access Token ---")
                print(f"Token: {result['access_token']}")
                print(f"Type:  {result.get('token_type', 'DPoP')}")
                print(f"Scope: {result.get('scope', 'all')}")
    except urllib.error.HTTPError as e:
        resp_body = e.read().decode()
        try:
            parsed = json.loads(resp_body)
        except json.JSONDecodeError:
            parsed = {"raw": resp_body}
        print(json.dumps({"status": e.code, "body": parsed}, indent=2))


def cmd_call(args):
    """Call an authenticated API endpoint."""
    headers = {}

    if args.private_key and args.token:
        sk = load_signing_key(args.private_key)
        domain = domain_from_did(args.did) if args.did else "localhost"
        url = f"https://{domain}{args.path.split('?')[0]}"
        dpop_proof = make_dpop_proof(sk, args.method, url, access_token=args.token)
        headers["Authorization"] = f"DPoP {args.token}"
        headers["DPoP"] = dpop_proof

    data = None
    if args.data:
        data = json.loads(args.data)
        headers["Content-Type"] = "application/json"

    result = http_request(args.method, args.path, headers=headers, data=data)
    print(json.dumps(result, indent=2))


def build_fake_jwe(sender_did: str, recipient_did: str) -> bytes:
    """Build a minimal JWE envelope for DIDComm passthrough testing.

    This is NOT real ECDH-1PU encryption — it's a structurally valid JWE that the
    server can route (extract recipient KID, validate domain, queue for delivery).
    The server only decrypts JWEs addressed to did:web:{domain}:server:platform.
    For regular entity-to-entity messages, the server queues the opaque JWE as-is.
    """
    import hashlib

    sender_kid = f"{sender_did}#key-x25519-1"
    recipient_kid = f"{recipient_did}#key-x25519-1"

    # Build a minimal protected header with skid
    header = {
        "alg": "ECDH-1PU+A256KW",
        "enc": "A256CBC-HS512",
        "epk": {"kty": "OKP", "crv": "X25519", "x": b64url_encode(b"\x00" * 32)},
        "apu": b64url_encode(sender_kid.encode()),
        "apv": b64url_encode(hashlib.sha256(recipient_kid.encode()).digest()),
        "skid": sender_kid,
    }
    protected_b64 = b64url_encode(json.dumps(header, separators=(",", ":")).encode())

    jwe = {
        "protected": protected_b64,
        "recipients": [
            {
                "header": {"kid": recipient_kid},
                "encrypted_key": b64url_encode(b"\x00" * 40),
            }
        ],
        "iv": b64url_encode(b"\x00" * 16),
        "ciphertext": b64url_encode(b"\x00" * 32),
        "tag": b64url_encode(b"\x00" * 32),
    }
    return json.dumps(jwe).encode()


def get_human_token(sk, did):
    """Get a DPoP access token for a human entity via Authorization Code + PKCE."""
    import urllib.parse
    import secrets

    domain = domain_from_did(did)

    # Generate PKCE code verifier and challenge
    code_verifier = b64url_encode(secrets.token_bytes(32))
    challenge_hash = hashlib.sha256(code_verifier.encode()).digest()
    code_challenge = b64url_encode(challenge_hash)

    # Step 1: Authorization request (GET, capture 302 redirect)
    authorize_url = f"https://{domain}/v1/oauth/authorize"
    dpop_proof = make_dpop_proof(sk, "GET", authorize_url)

    params = urllib.parse.urlencode({
        "response_type": "code",
        "client_id": did,
        "redirect_uri": "atap://callback",
        "scope": "atap:inbox atap:send atap:revoke atap:manage",
        "code_challenge": code_challenge,
        "code_challenge_method": "S256",
    })

    req = urllib.request.Request(
        f"{BASE_URL}/v1/oauth/authorize?{params}",
        headers={"DPoP": dpop_proof},
        method="GET",
    )

    # Don't follow redirects — capture the 302
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise urllib.error.HTTPError(newurl, code, msg, headers, fp)

    opener = urllib.request.build_opener(NoRedirect)
    try:
        opener.open(req)
        raise RuntimeError("Expected 302 redirect from authorize endpoint")
    except urllib.error.HTTPError as e:
        if e.code != 302:
            raise RuntimeError(f"Authorize failed ({e.code}): {e.read().decode()}")
        location = e.headers.get("Location", "")
        parsed = urllib.parse.urlparse(location)
        qs = urllib.parse.parse_qs(parsed.query)
        auth_code = qs.get("code", [None])[0]
        if not auth_code:
            raise RuntimeError(f"No code in redirect: {location}")

    # Step 2: Token exchange
    token_url = f"https://{domain}/v1/oauth/token"
    dpop_proof2 = make_dpop_proof(sk, "POST", token_url)

    form_data = urllib.parse.urlencode({
        "grant_type": "authorization_code",
        "code": auth_code,
        "redirect_uri": "atap://callback",
        "code_verifier": code_verifier,
    }).encode()

    req2 = urllib.request.Request(
        f"{BASE_URL}/v1/oauth/token",
        data=form_data,
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "DPoP": dpop_proof2,
        },
        method="POST",
    )

    with urllib.request.urlopen(req2) as resp:
        token_resp = json.loads(resp.read().decode())

    return token_resp["access_token"]


def authed_request(sk, did, access_token, method, path, data=None):
    """Make an authenticated API call with DPoP."""
    domain = domain_from_did(did)
    url = f"https://{domain}{path}"
    dpop_proof = make_dpop_proof(sk, method, url, access_token=access_token)

    headers = {
        "Authorization": f"DPoP {access_token}",
        "DPoP": dpop_proof,
    }

    body = None
    if data is not None:
        body = json.dumps(data).encode()
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=body,
        headers=headers,
        method=method,
    )

    try:
        with urllib.request.urlopen(req) as resp:
            resp_body = resp.read().decode()
            return {"status": resp.status, "body": json.loads(resp_body) if resp_body else {}}
    except urllib.error.HTTPError as e:
        resp_body = e.read().decode()
        try:
            parsed = json.loads(resp_body)
        except json.JSONDecodeError:
            parsed = {"raw": resp_body}
        return {"status": e.code, "body": parsed}


def cmd_credential_demo(args):
    """Register a human, get token, start email verification, wait for OTP, verify, list credentials."""
    print("=" * 60)
    print(" ATAP Credential Demo (Human Entity)")
    print("=" * 60)

    # Step 1: Generate keypair and register human
    print("\n[1/5] Generating Ed25519 keypair and registering human...")
    sk, vk, seed_b64, pub_b64 = generate_keypair()
    reg = http_request("POST", "/v1/entities", data={
        "type": "human",
        "public_key": pub_b64,
    })
    if reg["status"] != 201:
        print(f"Registration failed: {json.dumps(reg, indent=2)}")
        return

    did = reg["body"]["did"]
    entity_id = reg["body"]["id"]
    print(f"  ID:  {entity_id}")
    print(f"  DID: {did}")

    # Step 2: Get OAuth token via Authorization Code + PKCE + DPoP
    print("\n[2/5] Getting DPoP access token (Authorization Code + PKCE)...")
    try:
        access_token = get_human_token(sk, did)
        print(f"  Token: {access_token[:40]}...")
    except Exception as e:
        print(f"  Token failed: {e}")
        return

    # Step 3: Start email verification
    email = args.email or "test@example.com"
    print(f"\n[3/5] Starting email verification for {email}...")
    result = authed_request(sk, did, access_token, "POST",
                           "/v1/credentials/email/start", {"email": email})
    if result["status"] != 200:
        print(f"  Failed: {json.dumps(result, indent=2)}")
        return
    print(f"  {result['body'].get('message', 'OTP sent')}")
    print(f"\n  >>> Check server logs for: EMAIL OTP (stub — not sent)")

    # Step 4: Wait for OTP input
    print()
    otp = input("  Enter OTP from server logs: ").strip()
    if not otp:
        print("  No OTP entered, aborting.")
        return

    # Step 5: Verify OTP and get credential
    print(f"\n[4/5] Verifying OTP...")
    verify = authed_request(sk, did, access_token, "POST",
                            "/v1/credentials/email/verify", {"email": email, "otp": otp})
    if verify["status"] == 201:
        print(f"  Credential issued!")
        cred = verify["body"]
        print(f"  Type: {cred.get('type', 'unknown')}")
        print(f"  JWT:  {str(cred.get('credential', ''))[:60]}...")
    else:
        print(f"  Verification failed: {json.dumps(verify, indent=2)}")
        return

    # Step 6: List credentials
    print(f"\n[5/5] Listing credentials...")
    creds = authed_request(sk, did, access_token, "GET", "/v1/credentials")
    if creds["status"] == 200:
        cred_list = creds["body"] if isinstance(creds["body"], list) else creds["body"].get("credentials", [])
        print(f"  Found {len(cred_list)} credential(s)")
        for c in cred_list:
            print(f"    - {c.get('type', '?')} (issued: {c.get('issued_at', '?')})")
    else:
        print(f"  List failed: {json.dumps(creds, indent=2)}")

    print("\n" + "=" * 60)
    print(" Credential demo complete!")
    print("=" * 60)
    print(f"\n  Private key (save for later): {seed_b64}")
    print(f"  DID: {did}")


def cmd_demo(args):
    """Full demo: register agent -> get token -> submit revocation -> list revocations -> DIDComm."""
    print("=" * 60)
    print(" ATAP Demo Flow")
    print("=" * 60)

    # Step 1: Register agent
    print("\n[1/5] Registering agent (server-generated keypair)...")
    reg = http_request("POST", "/v1/entities", data={"type": "agent", "name": "demo-agent"})
    if reg["status"] != 201:
        print(f"Registration failed: {json.dumps(reg, indent=2)}")
        return
    did = reg["body"]["did"]
    secret = reg["body"]["client_secret"]
    private_key = reg["body"]["private_key"]
    print(f"  DID: {did}")
    print(f"  Secret: {secret[:20]}...")
    print(f"  Private key: {private_key[:20]}...")

    # Step 2: Get token
    print("\n[2/5] Getting DPoP access token (scope: atap:revoke)...")
    sk = load_signing_key(private_key)
    import urllib.parse

    # DPoP htu must match what the server expects: https://{PlatformDomain}/v1/oauth/token
    domain = domain_from_did(did)
    token_url = f"https://{domain}/v1/oauth/token"
    dpop_proof = make_dpop_proof(sk, "POST", token_url)

    form_data = urllib.parse.urlencode({
        "grant_type": "client_credentials",
        "client_id": did,
        "client_secret": secret,
        "scope": "atap:revoke",
    }).encode()

    req = urllib.request.Request(
        f"{BASE_URL}/v1/oauth/token",
        data=form_data,
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "DPoP": dpop_proof,
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req) as resp:
            token_resp = json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        print(f"Token request failed ({e.code}): {e.read().decode()}")
        return

    access_token = token_resp["access_token"]
    print(f"  Token: {access_token[:30]}...")

    # Step 3: Submit revocation
    print("\n[3/5] Submitting revocation...")
    revoke_url = f"https://{domain}/v1/revocations"
    dpop_proof2 = make_dpop_proof(sk, "POST", revoke_url, access_token=access_token)

    demo_approval_id = f"apr_demo_{uuid.uuid4().hex[:12]}"
    revoke_data = json.dumps({"approval_id": demo_approval_id, "signature": "demo_sig"}).encode()
    req2 = urllib.request.Request(
        f"{BASE_URL}/v1/revocations",
        data=revoke_data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"DPoP {access_token}",
            "DPoP": dpop_proof2,
        },
        method="POST",
    )

    try:
        with urllib.request.urlopen(req2) as resp:
            revoke_resp = json.loads(resp.read().decode())
            print(f"  Status: {resp.status}")
            print(f"  Revocation: {json.dumps(revoke_resp, indent=2)}")
    except urllib.error.HTTPError as e:
        print(f"  Revocation failed ({e.code}): {e.read().decode()}")
        return

    # Step 4: List revocations
    print(f"\n[4/5] Listing revocations for {did}...")
    encoded_did = urllib.parse.quote(did, safe="")
    list_result = http_request("GET", f"/v1/revocations?entity={encoded_did}")
    print(f"  Status: {list_result['status']}")
    print(f"  Revocations: {json.dumps(list_result['body'], indent=2)}")

    # Step 5: DIDComm message delivery (passthrough)
    print(f"\n[5/5] Sending DIDComm message to {did}...")

    # Register a second agent as the sender
    reg2 = http_request("POST", "/v1/entities", data={"type": "agent", "name": "demo-sender"})
    if reg2["status"] != 201:
        print(f"  Sender registration failed: {json.dumps(reg2, indent=2)}")
    else:
        sender_did = reg2["body"]["did"]
        print(f"  Sender DID: {sender_did}")

        # Build a structurally valid JWE addressed to the first agent
        jwe_bytes = build_fake_jwe(sender_did, did)

        req5 = urllib.request.Request(
            f"{BASE_URL}/v1/didcomm",
            data=jwe_bytes,
            headers={"Content-Type": "application/didcomm-encrypted+json"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(req5) as resp:
                didcomm_resp = json.loads(resp.read().decode())
                print(f"  Status: {resp.status}")
                print(f"  Response: {json.dumps(didcomm_resp, indent=2)}")
        except urllib.error.HTTPError as e:
            print(f"  DIDComm failed ({e.code}): {e.read().decode()}")

    print("\n" + "=" * 60)
    print(" Demo complete!")
    print("=" * 60)


def main():
    global BASE_URL
    parser = argparse.ArgumentParser(description="ATAP test helper")
    parser.add_argument("--base-url", default=BASE_URL, help="Server base URL")
    sub = parser.add_subparsers(dest="command")

    # register
    reg = sub.add_parser("register", help="Register an entity")
    reg.add_argument("--type", required=True, choices=["agent", "machine", "human", "org"])
    reg.add_argument("--name", default="test-entity")
    reg.add_argument("--private-key", help="Base64 Ed25519 private key (optional — server generates if omitted)")
    reg.add_argument("--principal-did", help="Principal DID (optional)")

    # token
    tok = sub.add_parser("token", help="Get a DPoP access token")
    tok.add_argument("--did", required=True, help="Entity DID (client_id)")
    tok.add_argument("--secret", required=True, help="Client secret")
    tok.add_argument("--private-key", required=True, help="Base64 Ed25519 private key (for DPoP proof)")
    tok.add_argument("--scope", default="atap:revoke", help="Requested scope")

    # call
    call = sub.add_parser("call", help="Call an API endpoint")
    call.add_argument("method", choices=["GET", "POST", "PUT", "DELETE"])
    call.add_argument("path", help="API path (e.g. /v1/revocations)")
    call.add_argument("--did", help="Entity DID (used to derive DPoP htu domain)")
    call.add_argument("--token", help="Access token")
    call.add_argument("--private-key", help="Base64 Ed25519 private key (for DPoP proof)")
    call.add_argument("--data", help="JSON body")

    # demo
    sub.add_parser("demo", help="Full demo: register -> token -> revoke -> list")

    # credential-demo
    cred_demo = sub.add_parser("credential-demo", help="Human: register -> token -> email OTP -> credential")
    cred_demo.add_argument("--email", default="test@example.com", help="Email address for verification")

    args = parser.parse_args()

    if args.base_url != BASE_URL:
        BASE_URL = args.base_url

    if args.command == "register":
        cmd_register(args)
    elif args.command == "token":
        cmd_token(args)
    elif args.command == "call":
        cmd_call(args)
    elif args.command == "demo":
        cmd_demo(args)
    elif args.command == "credential-demo":
        cmd_credential_demo(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
