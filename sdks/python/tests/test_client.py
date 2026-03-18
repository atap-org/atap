"""Tests for the main ATAPClient."""

import httpx
import nacl.signing
import pytest
import respx

from atap.client import ATAPClient
from atap.errors import ATAPError


class TestClientInit:
    def test_minimal_client(self) -> None:
        client = ATAPClient(base_url="http://localhost:8080")
        assert client.entities is not None
        assert client.approvals is not None
        client.close()

    def test_client_with_credentials(
        self, private_key_b64: str, test_did: str, test_secret: str
    ) -> None:
        client = ATAPClient(
            base_url="http://localhost:8080",
            did=test_did,
            private_key=private_key_b64,
            client_secret=test_secret,
        )
        assert client.token_manager is not None
        client.close()

    def test_client_with_signing_key(
        self, signing_key: nacl.signing.SigningKey, test_did: str
    ) -> None:
        client = ATAPClient(
            base_url="http://localhost:8080",
            did=test_did,
            signing_key=signing_key,
        )
        assert client.token_manager is not None
        client.close()

    def test_token_manager_not_initialized(self) -> None:
        client = ATAPClient(base_url="http://localhost:8080")
        with pytest.raises(ValueError, match="Token manager not initialized"):
            _ = client.token_manager

    def test_authed_request_without_credentials(self) -> None:
        client = ATAPClient(base_url="http://localhost:8080")
        with pytest.raises(ValueError, match="Authentication not configured"):
            client._authed_request("GET", "/v1/approvals")

    def test_context_manager(self) -> None:
        with ATAPClient(base_url="http://localhost:8080") as client:
            assert client.entities is not None

    def test_custom_platform_domain(
        self, private_key_b64: str, test_did: str
    ) -> None:
        client = ATAPClient(
            base_url="http://localhost:8080",
            did=test_did,
            private_key=private_key_b64,
            platform_domain="api.atap.app",
        )
        assert client._platform_domain == "api.atap.app"
        client.close()

    def test_custom_scopes(
        self, private_key_b64: str, test_did: str, test_secret: str
    ) -> None:
        client = ATAPClient(
            base_url="http://localhost:8080",
            did=test_did,
            private_key=private_key_b64,
            client_secret=test_secret,
            scopes=["atap:inbox"],
        )
        assert client.token_manager._scopes == ["atap:inbox"]
        client.close()


class TestClientIntegration:
    """Integration-style tests: register -> authenticate -> API call."""

    def test_register_then_call(self, respx_mock: respx.MockRouter) -> None:
        # Step 1: Register
        respx_mock.post("/v1/entities").mock(
            return_value=httpx.Response(201, json={
                "id": "test123",
                "did": "did:web:localhost%3A8080:agent:test123",
                "type": "agent",
                "name": "test",
                "key_id": "key_agn_123",
                "client_secret": "atap_test_secret",
                "private_key": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
            })
        )

        client = ATAPClient(base_url="http://localhost:8080")
        entity = client.entities.register("agent", name="test")

        # Step 2: Create authenticated client
        respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json={
                "access_token": "test_jwt",
                "token_type": "DPoP",
                "expires_in": 3600,
                "scope": "atap:inbox atap:send atap:revoke atap:manage",
            })
        )
        respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(200, json=[])
        )

        authed = ATAPClient(
            base_url="http://localhost:8080",
            did=entity.did,
            private_key=entity.private_key,
            client_secret=entity.client_secret,
        )

        # Step 3: List approvals
        approvals = authed.approvals.list()
        assert approvals == []
