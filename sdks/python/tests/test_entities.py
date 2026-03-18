"""Tests for entity operations."""

from typing import Any, Dict

import httpx
import pytest
import respx

from atap.client import ATAPClient
from atap.errors import ATAPNotFoundError


class TestRegister:
    def test_register_agent(
        self,
        respx_mock: respx.MockRouter,
        mock_entity_response: Dict[str, Any],
    ) -> None:
        respx_mock.post("/v1/entities").mock(
            return_value=httpx.Response(201, json=mock_entity_response)
        )
        client = ATAPClient(base_url="http://localhost:8080")
        entity = client.entities.register("agent", name="test-agent")
        assert entity.id == "01abc123def456"
        assert entity.type == "agent"
        assert entity.did.startswith("did:web:")
        assert entity.client_secret is not None

    def test_register_with_public_key(
        self,
        respx_mock: respx.MockRouter,
        public_key_b64: str,
        mock_entity_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/entities").mock(
            return_value=httpx.Response(201, json=mock_entity_response)
        )
        client = ATAPClient(base_url="http://localhost:8080")
        client.entities.register("human", public_key=public_key_b64)
        request_body = route.calls[0].request.content
        import json
        body = json.loads(request_body)
        assert body["public_key"] == public_key_b64

    def test_register_with_principal_did(
        self,
        respx_mock: respx.MockRouter,
        mock_entity_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/entities").mock(
            return_value=httpx.Response(201, json=mock_entity_response)
        )
        client = ATAPClient(base_url="http://localhost:8080")
        client.entities.register("agent", name="my-agent", principal_did="did:web:example.com:human:abc")
        import json
        body = json.loads(route.calls[0].request.content)
        assert body["principal_did"] == "did:web:example.com:human:abc"


class TestGet:
    def test_get_entity(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/entities/test123").mock(
            return_value=httpx.Response(200, json={
                "id": "test123",
                "type": "agent",
                "did": "did:web:localhost%3A8080:agent:test123",
                "trust_level": 0,
                "registry": "localhost:8080",
                "created_at": "2026-03-18T10:00:00Z",
                "updated_at": "2026-03-18T10:00:00Z",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        entity = client.entities.get("test123")
        assert entity.id == "test123"
        assert entity.type == "agent"

    def test_get_entity_not_found(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/entities/missing").mock(
            return_value=httpx.Response(404, json={
                "type": "https://atap.dev/errors/not-found",
                "title": "Not Found",
                "status": 404,
                "detail": "Entity not found",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        with pytest.raises(ATAPNotFoundError):
            client.entities.get("missing")


class TestDelete:
    def test_delete_entity(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.delete("/v1/entities/test123").mock(
            return_value=httpx.Response(204)
        )
        authed_client.entities.delete("test123")


class TestRotateKey:
    def test_rotate_key(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        public_key_b64: str,
    ) -> None:
        respx_mock.post("/v1/entities/test123/keys/rotate").mock(
            return_value=httpx.Response(200, json={
                "id": "key_agn_new123",
                "entity_id": "test123",
                "key_index": 2,
                "valid_from": "2026-03-18T10:00:00Z",
            })
        )
        kv = authed_client.entities.rotate_key("test123", public_key_b64)
        assert kv.id == "key_agn_new123"
        assert kv.key_index == 2
