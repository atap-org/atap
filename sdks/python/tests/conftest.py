"""Shared fixtures for ATAP SDK tests."""

import base64
import json
from typing import Any, Dict

import httpx
import nacl.signing
import pytest
import respx

from atap.client import ATAPClient
from atap.crypto import b64url_encode


@pytest.fixture
def signing_key() -> nacl.signing.SigningKey:
    """Deterministic Ed25519 signing key for reproducible tests."""
    seed = b"\x01" * 32
    return nacl.signing.SigningKey(seed)


@pytest.fixture
def verify_key(signing_key: nacl.signing.SigningKey) -> nacl.signing.VerifyKey:
    return signing_key.verify_key


@pytest.fixture
def private_key_b64(signing_key: nacl.signing.SigningKey) -> str:
    return base64.b64encode(bytes(signing_key)).decode()


@pytest.fixture
def public_key_b64(verify_key: nacl.signing.VerifyKey) -> str:
    return base64.b64encode(bytes(verify_key)).decode()


@pytest.fixture
def test_did() -> str:
    return "did:web:localhost%3A8080:agent:test123"


@pytest.fixture
def test_secret() -> str:
    return "atap_dGVzdHNlY3JldDEyMzQ1Njc4OTAxMjM0NTY3ODkw"


@pytest.fixture
def mock_token_response() -> Dict[str, Any]:
    return {
        "access_token": "eyJhbGciOiJFZERTQSJ9.test.sig",
        "token_type": "DPoP",
        "expires_in": 3600,
        "scope": "atap:inbox atap:send atap:revoke atap:manage",
        "refresh_token": "eyJhbGciOiJFZERTQSJ9.refresh.sig",
    }


@pytest.fixture
def mock_entity_response() -> Dict[str, Any]:
    return {
        "id": "01abc123def456",
        "did": "did:web:localhost%3A8080:agent:01abc123def456",
        "type": "agent",
        "name": "test-agent",
        "key_id": "key_agn_abc123",
        "client_secret": "atap_dGVzdHNlY3JldDEyMzQ1Njc4OTAxMjM0NTY3ODkw",
        "private_key": base64.b64encode(b"\x01" * 32).decode(),
    }


@pytest.fixture
def mock_approval_response() -> Dict[str, Any]:
    return {
        "id": "apr_01abc123def456",
        "state": "requested",
        "created_at": "2026-03-18T10:00:00Z",
        "from": "did:web:localhost%3A8080:agent:requester",
        "to": "did:web:localhost%3A8080:agent:approver",
        "subject": {
            "type": "com.example.payment",
            "label": "Payment of $100",
            "payload": {"amount": 100, "currency": "USD"},
        },
    }


@pytest.fixture
def mock_revocation_response() -> Dict[str, Any]:
    return {
        "id": "rev_01abc123def456",
        "approval_id": "apr_01abc123def456",
        "approver_did": "did:web:localhost%3A8080:agent:test123",
        "revoked_at": "2026-03-18T10:00:00Z",
        "expires_at": "2026-03-18T11:00:00Z",
    }


@pytest.fixture
def respx_mock():
    """Activate respx mock for httpx."""
    with respx.mock(base_url="http://localhost:8080") as mock:
        yield mock


@pytest.fixture
def authed_client(
    respx_mock: respx.MockRouter,
    private_key_b64: str,
    test_did: str,
    test_secret: str,
    mock_token_response: Dict[str, Any],
) -> ATAPClient:
    """Client with mocked token endpoint for authenticated requests."""
    respx_mock.post("/v1/oauth/token").mock(
        return_value=httpx.Response(200, json=mock_token_response)
    )
    return ATAPClient(
        base_url="http://localhost:8080",
        did=test_did,
        private_key=private_key_b64,
        client_secret=test_secret,
    )


@pytest.fixture
def public_client() -> ATAPClient:
    """Client for public (unauthenticated) endpoints only."""
    return ATAPClient(base_url="http://localhost:8080")
