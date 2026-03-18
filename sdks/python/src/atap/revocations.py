"""Revocation operations."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from atap.models import Revocation, RevocationList


class RevocationAPI:
    """Submit and query revocations."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def submit(
        self,
        approval_id: str,
        signature: str,
        *,
        valid_until: Optional[str] = None,
    ) -> Revocation:
        """Submit a revocation. Requires atap:revoke scope.

        Args:
            approval_id: The approval ID to revoke (apr_...).
            signature: JWS signature.
            valid_until: Optional RFC3339 expiry (defaults to revoked_at + 60min).
        """
        body: Dict[str, Any] = {
            "approval_id": approval_id,
            "signature": signature,
        }
        if valid_until:
            body["valid_until"] = valid_until

        data = self._client._authed_request("POST", "/v1/revocations", json_body=body)
        return _parse_revocation(data)

    def list(self, entity_did: str) -> RevocationList:
        """Query active revocations for an entity (public endpoint).

        Args:
            entity_did: The approver DID to query.
        """
        data = self._client._http.request(
            "GET", "/v1/revocations", params={"entity": entity_did}
        )
        revocations = [_parse_revocation(r) for r in data.get("revocations", [])]
        return RevocationList(
            entity=data.get("entity", entity_did),
            revocations=revocations,
            checked_at=data.get("checked_at", ""),
        )


def _parse_revocation(data: Dict[str, Any]) -> Revocation:
    return Revocation(
        id=data.get("id", ""),
        approval_id=data.get("approval_id", ""),
        approver_did=data.get("approver_did", ""),
        revoked_at=data.get("revoked_at", ""),
        expires_at=data.get("expires_at", ""),
    )
