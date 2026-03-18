"""Discovery and DID resolution operations."""

from __future__ import annotations

from typing import Any, Dict, List

from atap.models import DIDDocument, DiscoveryDocument, VerificationMethod


class DiscoveryAPI:
    """Server discovery and DID document resolution."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def discover(self) -> DiscoveryDocument:
        """Fetch the server discovery document from /.well-known/atap.json."""
        data = self._client._http.request("GET", "/.well-known/atap.json")
        return DiscoveryDocument(
            domain=data.get("domain", ""),
            api_base=data.get("api_base", ""),
            didcomm_endpoint=data.get("didcomm_endpoint", ""),
            claim_types=data.get("claim_types", []),
            max_approval_ttl=data.get("max_approval_ttl", ""),
            trust_level=data.get("trust_level", 0),
            oauth=data.get("oauth"),
        )

    def resolve_did(self, entity_type: str, entity_id: str) -> DIDDocument:
        """Resolve an entity's DID Document.

        Args:
            entity_type: Entity type (agent, machine, human, org).
            entity_id: Entity ID.
        """
        data = self._client._http.request("GET", f"/{entity_type}/{entity_id}/did.json")
        return _parse_did_document(data)

    def server_did(self) -> DIDDocument:
        """Fetch the server's DID Document."""
        data = self._client._http.request("GET", "/server/platform/did.json")
        return _parse_did_document(data)

    def health(self) -> Dict[str, Any]:
        """Check server health."""
        return self._client._http.request("GET", "/v1/health")


def _parse_did_document(data: Dict[str, Any]) -> DIDDocument:
    vms = [
        VerificationMethod(
            id=vm.get("id", ""),
            type=vm.get("type", ""),
            controller=vm.get("controller", ""),
            public_key_multibase=vm.get("publicKeyMultibase", ""),
        )
        for vm in data.get("verificationMethod", [])
    ]
    return DIDDocument(
        id=data.get("id", ""),
        context=data.get("@context", []),
        verification_method=vms,
        authentication=data.get("authentication", []),
        assertion_method=data.get("assertionMethod", []),
        key_agreement=data.get("keyAgreement", []),
        service=data.get("service", []),
        atap_type=data.get("atap:type", ""),
        atap_principal=data.get("atap:principal", ""),
    )
