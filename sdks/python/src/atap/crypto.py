"""Cryptographic operations for the ATAP SDK.

Handles Ed25519 key generation, DPoP proof creation (RFC 9449),
JWK thumbprint computation (RFC 7638), and PKCE S256 challenges.
"""

from __future__ import annotations

import base64
import hashlib
import json
import secrets
import time
import uuid
from typing import Optional, Tuple

import nacl.signing


def b64url_encode(data: bytes) -> str:
    """Base64url encode without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def b64url_decode(s: str) -> bytes:
    """Base64url decode with padding restoration."""
    padding = 4 - len(s) % 4
    if padding != 4:
        s += "=" * padding
    return base64.urlsafe_b64decode(s)


def generate_keypair() -> Tuple[nacl.signing.SigningKey, nacl.signing.VerifyKey]:
    """Generate a new Ed25519 keypair."""
    sk = nacl.signing.SigningKey.generate()
    return sk, sk.verify_key


def load_signing_key(private_key_b64: str) -> nacl.signing.SigningKey:
    """Load Ed25519 signing key from base64. Accepts 32-byte seed or 64-byte full key."""
    raw = base64.b64decode(private_key_b64)
    if len(raw) == 64:
        return nacl.signing.SigningKey(raw[:32])
    elif len(raw) == 32:
        return nacl.signing.SigningKey(raw)
    else:
        raise ValueError(f"Invalid private key length: {len(raw)} bytes (expected 32 or 64)")


def public_key_b64(sk: nacl.signing.SigningKey) -> str:
    """Get base64-encoded public key from a signing key."""
    return base64.b64encode(bytes(sk.verify_key)).decode()


def jwk_thumbprint(verify_key: nacl.signing.VerifyKey) -> str:
    """Compute JWK thumbprint (RFC 7638) for an Ed25519 public key."""
    x = b64url_encode(bytes(verify_key))
    canonical = json.dumps(
        {"crv": "Ed25519", "kty": "OKP", "x": x},
        separators=(",", ":"),
        sort_keys=True,
    )
    digest = hashlib.sha256(canonical.encode()).digest()
    return b64url_encode(digest)


def make_dpop_proof(
    signing_key: nacl.signing.SigningKey,
    method: str,
    url: str,
    access_token: Optional[str] = None,
) -> str:
    """Create a DPoP proof JWT (RFC 9449).

    Args:
        signing_key: Ed25519 signing key.
        method: HTTP method (GET, POST, etc.).
        url: Full URL for the htu claim (must use https://{platform_domain}/path).
        access_token: If provided, includes ath (access token hash) claim.

    Returns:
        Compact JWS string (header.payload.signature).
    """
    verify_key = signing_key.verify_key
    x = b64url_encode(bytes(verify_key))

    header = {
        "typ": "dpop+jwt",
        "alg": "EdDSA",
        "jwk": {"kty": "OKP", "crv": "Ed25519", "x": x},
    }

    payload: dict = {
        "jti": str(uuid.uuid4()),
        "htm": method,
        "htu": url,
        "iat": int(time.time()),
    }

    if access_token:
        ath = hashlib.sha256(access_token.encode("ascii")).digest()
        payload["ath"] = b64url_encode(ath)

    header_b64 = b64url_encode(json.dumps(header, separators=(",", ":")).encode())
    payload_b64 = b64url_encode(json.dumps(payload, separators=(",", ":")).encode())
    signing_input = f"{header_b64}.{payload_b64}".encode()

    signed = signing_key.sign(signing_input)
    sig_b64 = b64url_encode(signed.signature)

    return f"{header_b64}.{payload_b64}.{sig_b64}"


def generate_pkce() -> Tuple[str, str]:
    """Generate PKCE code verifier and S256 challenge.

    Returns:
        Tuple of (code_verifier, code_challenge).
    """
    verifier = b64url_encode(secrets.token_bytes(32))
    challenge = b64url_encode(hashlib.sha256(verifier.encode()).digest())
    return verifier, challenge


def domain_from_did(did: str) -> str:
    """Extract platform domain from a DID.

    did:web:localhost%3A8080:agent:abc -> localhost:8080
    """
    parts = did.split(":")
    if len(parts) < 3:
        raise ValueError(f"Invalid DID format: {did}")
    domain = parts[2]
    domain = domain.replace("%3A", ":")
    return domain
