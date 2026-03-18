"""Tests for revocation operations."""

from typing import Any, Dict

import httpx
import pytest
import respx

from atap.client import ATAPClient


class TestSubmit:
    def test_submit_revocation(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        mock_revocation_response: Dict[str, Any],
    ) -> None:
        respx_mock.post("/v1/revocations").mock(
            return_value=httpx.Response(201, json=mock_revocation_response)
        )
        rev = authed_client.revocations.submit("apr_01abc123def456", "jws_sig")
        assert rev.id == "rev_01abc123def456"
        assert rev.approval_id == "apr_01abc123def456"

    def test_submit_with_valid_until(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        mock_revocation_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/revocations").mock(
            return_value=httpx.Response(201, json=mock_revocation_response)
        )
        authed_client.revocations.submit(
            "apr_123", "sig", valid_until="2026-03-19T10:00:00Z"
        )
        import json
        body = json.loads(route.calls[0].request.content)
        assert body["valid_until"] == "2026-03-19T10:00:00Z"


class TestList:
    def test_list_revocations(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/revocations").mock(
            return_value=httpx.Response(200, json={
                "entity": "did:web:localhost%3A8080:agent:test",
                "revocations": [
                    {
                        "id": "rev_abc",
                        "approval_id": "apr_123",
                        "approver_did": "did:web:localhost%3A8080:agent:test",
                        "revoked_at": "2026-03-18T10:00:00Z",
                        "expires_at": "2026-03-18T11:00:00Z",
                    }
                ],
                "checked_at": "2026-03-18T10:05:00Z",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        result = client.revocations.list("did:web:localhost%3A8080:agent:test")
        assert result.entity == "did:web:localhost%3A8080:agent:test"
        assert len(result.revocations) == 1
        assert result.revocations[0].id == "rev_abc"

    def test_list_empty(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/revocations").mock(
            return_value=httpx.Response(200, json={
                "entity": "did:web:test",
                "revocations": [],
                "checked_at": "2026-03-18T10:05:00Z",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        result = client.revocations.list("did:web:test")
        assert len(result.revocations) == 0
