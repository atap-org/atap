"""Approval operations."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from atap.models import Approval, ApprovalSubject


class ApprovalAPI:
    """Create, respond to, list, and revoke approvals."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def create(
        self,
        from_did: str,
        to_did: str,
        subject: ApprovalSubject,
        *,
        via: Optional[str] = None,
    ) -> Approval:
        """Create an approval request. Requires atap:send scope.

        Args:
            from_did: Requester DID.
            to_did: Approver DID (or org DID for fan-out).
            subject: The approval subject with type, label, and payload.
            via: Optional mediating system DID.
        """
        body: Dict[str, Any] = {
            "from": from_did,
            "to": to_did,
            "subject": {
                "type": subject.type,
                "label": subject.label,
                "payload": subject.payload or {},
            },
        }
        if via:
            body["via"] = via

        data = self._client._authed_request("POST", "/v1/approvals", json_body=body)
        return _parse_approval(data)

    def respond(self, approval_id: str, signature: str) -> Approval:
        """Respond to an approval (approve). Requires atap:send scope.

        Args:
            approval_id: The approval ID (apr_...).
            signature: JWS signature from the approver.
        """
        data = self._client._authed_request(
            "POST",
            f"/v1/approvals/{approval_id}/respond",
            json_body={"signature": signature},
        )
        return _parse_approval(data)

    def list(self) -> List[Approval]:
        """List approvals addressed to the authenticated entity. Requires atap:inbox scope."""
        data = self._client._authed_request("GET", "/v1/approvals")
        if isinstance(data, list):
            return [_parse_approval(a) for a in data]
        return [_parse_approval(a) for a in data.get("approvals", data.get("items", []))]

    def revoke(self, approval_id: str) -> Approval:
        """Revoke an approval. Requires atap:revoke scope."""
        data = self._client._authed_request("DELETE", f"/v1/approvals/{approval_id}")
        return _parse_approval(data)


def _parse_approval(data: Dict[str, Any]) -> Approval:
    subject = None
    if "subject" in data:
        s = data["subject"]
        subject = ApprovalSubject(
            type=s.get("type", ""),
            label=s.get("label", ""),
            reversible=s.get("reversible", False),
            payload=s.get("payload"),
        )
    return Approval(
        id=data.get("id", ""),
        state=data.get("state", ""),
        created_at=data.get("created_at", ""),
        valid_until=data.get("valid_until"),
        from_did=data.get("from", ""),
        to_did=data.get("to", ""),
        via=data.get("via", ""),
        parent=data.get("parent", ""),
        subject=subject,
        template_url=data.get("template_url", ""),
        signatures=data.get("signatures", {}),
        responded_at=data.get("responded_at"),
        fan_out=data.get("fan_out"),
    )
