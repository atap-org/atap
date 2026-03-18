"""Tests for approval operations."""

from typing import Any, Dict

import httpx
import pytest
import respx

from atap.client import ATAPClient
from atap.errors import ATAPConflictError
from atap.models import ApprovalSubject


class TestCreate:
    def test_create_approval(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        mock_approval_response: Dict[str, Any],
    ) -> None:
        respx_mock.post("/v1/approvals").mock(
            return_value=httpx.Response(202, json=mock_approval_response)
        )
        approval = authed_client.approvals.create(
            from_did="did:web:localhost%3A8080:agent:requester",
            to_did="did:web:localhost%3A8080:agent:approver",
            subject=ApprovalSubject(
                type="com.example.payment",
                label="Payment of $100",
                payload={"amount": 100, "currency": "USD"},
            ),
        )
        assert approval.id == "apr_01abc123def456"
        assert approval.state == "requested"
        assert approval.subject is not None
        assert approval.subject.type == "com.example.payment"

    def test_create_approval_with_via(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        mock_approval_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/approvals").mock(
            return_value=httpx.Response(202, json=mock_approval_response)
        )
        authed_client.approvals.create(
            from_did="did:web:test:agent:a",
            to_did="did:web:test:agent:b",
            subject=ApprovalSubject(type="test", label="test"),
            via="did:web:test:machine:mediator",
        )
        import json
        body = json.loads(route.calls[0].request.content)
        assert body["via"] == "did:web:test:machine:mediator"


class TestRespond:
    def test_respond_approval(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/approvals/apr_123/respond").mock(
            return_value=httpx.Response(200, json={
                "id": "apr_123",
                "state": "approved",
            })
        )
        approval = authed_client.approvals.respond("apr_123", "jws_signature_here")
        assert approval.state == "approved"

    def test_respond_conflict(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/approvals/apr_123/respond").mock(
            return_value=httpx.Response(409, json={
                "type": "https://atap.dev/errors/conflict",
                "title": "Conflict",
                "status": 409,
                "detail": "Already responded",
            })
        )
        with pytest.raises(ATAPConflictError):
            authed_client.approvals.respond("apr_123", "sig")


class TestList:
    def test_list_approvals(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
        mock_approval_response: Dict[str, Any],
    ) -> None:
        respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(200, json=[mock_approval_response])
        )
        approvals = authed_client.approvals.list()
        assert len(approvals) == 1
        assert approvals[0].id == "apr_01abc123def456"

    def test_list_empty(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(200, json=[])
        )
        approvals = authed_client.approvals.list()
        assert len(approvals) == 0


class TestRevoke:
    def test_revoke_approval(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.delete("/v1/approvals/apr_123").mock(
            return_value=httpx.Response(200, json={
                "id": "apr_123",
                "state": "revoked",
            })
        )
        approval = authed_client.approvals.revoke("apr_123")
        assert approval.state == "revoked"
