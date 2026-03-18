"""ATAP SDK — Python client for the Agent Trust and Authority Protocol."""

from atap.client import ATAPClient
from atap.errors import ATAPError, ATAPAuthError, ATAPNotFoundError, ATAPProblemError
from atap.models import (
    Entity,
    Approval,
    ApprovalSubject,
    Revocation,
    Credential,
    DIDCommMessage,
    DIDDocument,
    ProblemDetail,
    KeyVersion,
    OAuthToken,
    DiscoveryDocument,
)

__version__ = "0.1.0"

__all__ = [
    "ATAPClient",
    "ATAPError",
    "ATAPAuthError",
    "ATAPNotFoundError",
    "ATAPProblemError",
    "Entity",
    "Approval",
    "ApprovalSubject",
    "Revocation",
    "Credential",
    "DIDCommMessage",
    "DIDDocument",
    "ProblemDetail",
    "KeyVersion",
    "OAuthToken",
    "DiscoveryDocument",
]
