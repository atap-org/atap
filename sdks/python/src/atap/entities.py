"""Entity management operations."""

from __future__ import annotations

from typing import Any, Dict, Optional

from atap.models import Entity, KeyVersion


class EntityAPI:
    """Entity registration, retrieval, deletion, and key rotation."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def register(
        self,
        entity_type: str,
        *,
        name: str = "",
        public_key: Optional[str] = None,
        principal_did: Optional[str] = None,
    ) -> Entity:
        """Register a new entity.

        Args:
            entity_type: One of "agent", "machine", "human", "org".
            name: Optional display name.
            public_key: Optional base64-encoded Ed25519 public key.
                If omitted, the server generates a keypair.
            principal_did: Optional DID for agent-to-principal binding.

        Returns:
            Entity with id, did, type, name, key_id.
            For agent/machine: also includes client_secret (returned once).
            If server-generated key: also includes private_key (returned once).
        """
        body: Dict[str, str] = {"type": entity_type}
        if name:
            body["name"] = name
        if public_key:
            body["public_key"] = public_key
        if principal_did:
            body["principal_did"] = principal_did

        data = self._client._http.request("POST", "/v1/entities", json_body=body)
        return _parse_entity(data)

    def get(self, entity_id: str) -> Entity:
        """Get public entity info by ID."""
        data = self._client._http.request("GET", f"/v1/entities/{entity_id}")
        return _parse_entity(data)

    def delete(self, entity_id: str) -> None:
        """Delete an entity (crypto-shred). Requires atap:manage scope."""
        self._client._authed_request("DELETE", f"/v1/entities/{entity_id}")

    def rotate_key(self, entity_id: str, public_key: str) -> KeyVersion:
        """Rotate an entity's Ed25519 public key. Requires atap:manage scope.

        Args:
            entity_id: The entity ID.
            public_key: Base64-encoded new Ed25519 public key.

        Returns:
            New KeyVersion with id, key_index, valid_from.
        """
        data = self._client._authed_request(
            "POST",
            f"/v1/entities/{entity_id}/keys/rotate",
            json_body={"public_key": public_key},
        )
        return KeyVersion(
            id=data.get("id", ""),
            entity_id=data.get("entity_id", ""),
            key_index=data.get("key_index", 0),
            valid_from=data.get("valid_from", ""),
            valid_until=data.get("valid_until"),
            created_at=data.get("created_at", ""),
        )


def _parse_entity(data: Dict[str, Any]) -> Entity:
    return Entity(
        id=data.get("id", ""),
        type=data.get("type", ""),
        did=data.get("did", ""),
        principal_did=data.get("principal_did", ""),
        name=data.get("name", ""),
        key_id=data.get("key_id", ""),
        public_key=data.get("public_key", ""),
        trust_level=data.get("trust_level", 0),
        registry=data.get("registry", ""),
        created_at=data.get("created_at", ""),
        updated_at=data.get("updated_at", ""),
        client_secret=data.get("client_secret"),
        private_key=data.get("private_key"),
    )
