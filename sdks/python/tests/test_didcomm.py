"""Tests for DIDComm operations."""

import json

import httpx
import pytest
import respx

from atap.client import ATAPClient


class TestSend:
    def test_send_didcomm(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.post("/v1/didcomm").mock(
            return_value=httpx.Response(202, json={
                "id": "msg_abc123",
                "status": "queued",
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        jwe = json.dumps({"protected": "test", "recipients": []}).encode()
        result = client.didcomm.send(jwe)
        assert result["id"] == "msg_abc123"
        assert result["status"] == "queued"


class TestInbox:
    def test_get_inbox(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.get("/v1/didcomm/inbox").mock(
            return_value=httpx.Response(200, json={
                "messages": [
                    {
                        "id": "msg_abc",
                        "sender_did": "did:web:test:agent:sender",
                        "message_type": "test",
                        "payload": "base64encodedpayload",
                        "created_at": "2026-03-18T10:00:00Z",
                    }
                ],
                "count": 1,
            })
        )
        inbox = authed_client.didcomm.inbox()
        assert inbox.count == 1
        assert len(inbox.messages) == 1
        assert inbox.messages[0].id == "msg_abc"
        assert inbox.messages[0].sender_did == "did:web:test:agent:sender"

    def test_get_inbox_empty(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.get("/v1/didcomm/inbox").mock(
            return_value=httpx.Response(200, json={"messages": [], "count": 0})
        )
        inbox = authed_client.didcomm.inbox()
        assert inbox.count == 0
        assert len(inbox.messages) == 0

    def test_inbox_limit(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        route = respx_mock.get("/v1/didcomm/inbox").mock(
            return_value=httpx.Response(200, json={"messages": [], "count": 0})
        )
        authed_client.didcomm.inbox(limit=10)
        assert "limit=10" in str(route.calls[0].request.url)
