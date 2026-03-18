"""Tests for discovery and DID resolution."""

import httpx
import respx

from atap.client import ATAPClient


class TestDiscover:
    def test_discover(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/.well-known/atap.json").mock(
            return_value=httpx.Response(200, json={
                "domain": "atap.dev",
                "api_base": "https://api.atap.app",
                "didcomm_endpoint": "https://api.atap.app/v1/didcomm",
                "claim_types": ["ATAPEmailVerification", "ATAPPhoneVerification"],
                "max_approval_ttl": "P30D",
                "trust_level": 3,
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        doc = client.discovery.discover()
        assert doc.domain == "atap.dev"
        assert doc.api_base == "https://api.atap.app"
        assert len(doc.claim_types) == 2


class TestResolveDID:
    def test_resolve_entity_did(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/agent/test123/did.json").mock(
            return_value=httpx.Response(200, json={
                "@context": ["https://www.w3.org/ns/did/v1"],
                "id": "did:web:localhost%3A8080:agent:test123",
                "verificationMethod": [
                    {
                        "id": "did:web:localhost%3A8080:agent:test123#key-1",
                        "type": "Ed25519VerificationKey2020",
                        "controller": "did:web:localhost%3A8080:agent:test123",
                        "publicKeyMultibase": "z6Mk...",
                    }
                ],
                "authentication": ["did:web:localhost%3A8080:agent:test123#key-1"],
                "assertionMethod": ["did:web:localhost%3A8080:agent:test123#key-1"],
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        doc = client.discovery.resolve_did("agent", "test123")
        assert doc.id == "did:web:localhost%3A8080:agent:test123"
        assert len(doc.verification_method) == 1
        assert doc.verification_method[0].type == "Ed25519VerificationKey2020"


class TestServerDID:
    def test_server_did(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/server/platform/did.json").mock(
            return_value=httpx.Response(200, json={
                "@context": ["https://www.w3.org/ns/did/v1"],
                "id": "did:web:localhost%3A8080:server:platform",
                "verificationMethod": [],
                "authentication": [],
                "assertionMethod": [],
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        doc = client.discovery.server_did()
        assert "server:platform" in doc.id


class TestHealth:
    def test_health(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/health").mock(
            return_value=httpx.Response(200, json={
                "status": "ok",
                "protocol": "ATAP",
                "version": "0.1",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        result = client.discovery.health()
        assert result["status"] == "ok"
