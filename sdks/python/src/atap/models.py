"""Domain models for the ATAP SDK."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


@dataclass
class ProblemDetail:
    """RFC 7807 Problem Details error response."""

    type: str
    title: str
    status: int
    detail: Optional[str] = None
    instance: Optional[str] = None


@dataclass
class Entity:
    """An ATAP entity (agent, machine, human, or org)."""

    id: str
    type: str
    did: str = ""
    principal_did: str = ""
    name: str = ""
    key_id: str = ""
    public_key: str = ""
    trust_level: int = 0
    registry: str = ""
    created_at: str = ""
    updated_at: str = ""
    # Only returned at registration, not stored
    client_secret: Optional[str] = None
    private_key: Optional[str] = None


@dataclass
class KeyVersion:
    """A versioned public key for an entity."""

    id: str
    entity_id: str = ""
    key_index: int = 0
    valid_from: str = ""
    valid_until: Optional[str] = None
    created_at: str = ""


@dataclass
class ApprovalSubject:
    """The purpose and payload of an approval."""

    type: str
    label: str
    reversible: bool = False
    payload: Optional[Dict[str, Any]] = None


@dataclass
class Approval:
    """A multi-signature approval document."""

    id: str
    state: str = ""
    created_at: str = ""
    valid_until: Optional[str] = None
    from_did: str = ""
    to_did: str = ""
    via: str = ""
    parent: str = ""
    subject: Optional[ApprovalSubject] = None
    template_url: str = ""
    signatures: Dict[str, str] = field(default_factory=dict)
    responded_at: Optional[str] = None
    fan_out: Optional[int] = None


@dataclass
class Revocation:
    """A revocation entry for a previously-granted approval."""

    id: str
    approval_id: str
    approver_did: str = ""
    revoked_at: str = ""
    expires_at: str = ""


@dataclass
class RevocationList:
    """A list of active revocations for an entity."""

    entity: str
    revocations: List[Revocation] = field(default_factory=list)
    checked_at: str = ""


@dataclass
class DIDCommMessage:
    """A DIDComm message from the inbox."""

    id: str
    sender_did: str = ""
    message_type: str = ""
    payload: str = ""
    created_at: str = ""


@dataclass
class DIDCommInbox:
    """DIDComm inbox response."""

    messages: List[DIDCommMessage] = field(default_factory=list)
    count: int = 0


@dataclass
class Credential:
    """A W3C Verifiable Credential."""

    id: str = ""
    type: str = ""
    credential: str = ""
    issued_at: str = ""
    revoked_at: Optional[str] = None


@dataclass
class OAuthToken:
    """An OAuth 2.1 token response."""

    access_token: str
    token_type: str = "DPoP"
    expires_in: int = 3600
    scope: str = ""
    refresh_token: Optional[str] = None


@dataclass
class DiscoveryDocument:
    """Server discovery document from /.well-known/atap.json."""

    domain: str = ""
    api_base: str = ""
    didcomm_endpoint: str = ""
    claim_types: List[str] = field(default_factory=list)
    max_approval_ttl: str = ""
    trust_level: int = 0
    oauth: Optional[Dict[str, Any]] = None


@dataclass
class VerificationMethod:
    """A verification method in a DID Document."""

    id: str
    type: str = ""
    controller: str = ""
    public_key_multibase: str = ""


@dataclass
class DIDDocument:
    """A W3C DID Document."""

    id: str
    context: List[str] = field(default_factory=list)
    verification_method: List[VerificationMethod] = field(default_factory=list)
    authentication: List[str] = field(default_factory=list)
    assertion_method: List[str] = field(default_factory=list)
    key_agreement: List[str] = field(default_factory=list)
    service: List[Dict[str, Any]] = field(default_factory=list)
    atap_type: str = ""
    atap_principal: str = ""
