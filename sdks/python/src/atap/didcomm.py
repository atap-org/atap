"""DIDComm messaging operations."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from atap.models import DIDCommInbox, DIDCommMessage


class DIDCommAPI:
    """Send and receive DIDComm messages."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def send(self, jwe_bytes: bytes) -> Dict[str, Any]:
        """Send a DIDComm message (JWE envelope). Public endpoint.

        Args:
            jwe_bytes: Raw JWE bytes (application/didcomm-encrypted+json).

        Returns:
            Dict with id and status ("queued").
        """
        data = self._client._http.request(
            "POST",
            "/v1/didcomm",
            headers={
                "Content-Type": "application/didcomm-encrypted+json",
            },
            json_body=None,
        )
        # Use raw request since we need to send raw bytes
        return self._send_raw(jwe_bytes)

    def _send_raw(self, jwe_bytes: bytes) -> Dict[str, Any]:
        """Send raw JWE bytes to the DIDComm endpoint."""
        import httpx

        response = self._client._http._client.post(
            "/v1/didcomm",
            content=jwe_bytes,
            headers={"Content-Type": "application/didcomm-encrypted+json"},
        )
        if response.status_code == 202 or response.status_code == 200:
            return response.json()
        # Let the error handler deal with it
        return self._client._http._handle_response(response)

    def inbox(self, *, limit: int = 50) -> DIDCommInbox:
        """Retrieve pending DIDComm messages. Requires atap:inbox scope.

        Args:
            limit: Max messages to return (default 50, max 100).
        """
        params = {"limit": str(min(limit, 100))}
        data = self._client._authed_request("GET", "/v1/didcomm/inbox", params=params)
        messages = [
            DIDCommMessage(
                id=m.get("id", ""),
                sender_did=m.get("sender_did", ""),
                message_type=m.get("message_type", ""),
                payload=m.get("payload", ""),
                created_at=m.get("created_at", ""),
            )
            for m in data.get("messages", [])
        ]
        return DIDCommInbox(messages=messages, count=data.get("count", len(messages)))
