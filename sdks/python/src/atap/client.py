"""Main ATAP client that ties all API modules together."""

from __future__ import annotations

import base64
from typing import Any, Dict, List, Optional

import nacl.signing

from atap.approvals import ApprovalAPI
from atap.credentials import CredentialAPI
from atap.crypto import domain_from_did, load_signing_key, public_key_b64
from atap.didcomm import DIDCommAPI
from atap.discovery import DiscoveryAPI
from atap.entities import EntityAPI
from atap.http_client import HTTPClient
from atap.oauth import TokenManager
from atap.revocations import RevocationAPI


class ATAPClient:
    """High-level client for the ATAP platform.

    Usage for agent/machine (client_credentials):
        client = ATAPClient(
            base_url="http://localhost:8080",
            did="did:web:localhost%3A8080:agent:abc",
            private_key="<base64 Ed25519 seed>",
            client_secret="atap_...",
        )

    Usage for human/org (authorization_code + PKCE):
        client = ATAPClient(
            base_url="http://localhost:8080",
            did="did:web:localhost%3A8080:human:abc",
            private_key="<base64 Ed25519 seed>",
        )
        client.token_manager.obtain_authorization_code()
    """

    def __init__(
        self,
        *,
        base_url: str = "http://localhost:8080",
        did: str = "",
        private_key: Optional[str] = None,
        signing_key: Optional[nacl.signing.SigningKey] = None,
        client_secret: Optional[str] = None,
        scopes: Optional[List[str]] = None,
        platform_domain: Optional[str] = None,
        timeout: float = 30.0,
    ) -> None:
        """Initialize the ATAP client.

        Args:
            base_url: HTTP base URL for the ATAP server.
            did: Entity DID (used as client_id for OAuth).
            private_key: Base64-encoded Ed25519 private key (seed or full key).
            signing_key: Pre-loaded nacl SigningKey (alternative to private_key).
            client_secret: Client secret for agent/machine client_credentials grant.
            scopes: OAuth scopes (defaults to all).
            platform_domain: Domain for DPoP htu construction.
                Defaults to domain extracted from DID.
            timeout: HTTP request timeout in seconds.
        """
        self._http = HTTPClient(base_url=base_url, timeout=timeout)
        self._did = did
        self._platform_domain = platform_domain or (domain_from_did(did) if did else "localhost")

        if signing_key:
            self._signing_key = signing_key
        elif private_key:
            self._signing_key = load_signing_key(private_key)
        else:
            self._signing_key = None  # type: ignore[assignment]

        self._token_manager: Optional[TokenManager] = None
        if self._signing_key and did:
            self._token_manager = TokenManager(
                http_client=self._http,
                signing_key=self._signing_key,
                did=did,
                client_secret=client_secret,
                scopes=scopes,
                platform_domain=self._platform_domain,
            )

        # API modules
        self.entities = EntityAPI(self)
        self.approvals = ApprovalAPI(self)
        self.revocations = RevocationAPI(self)
        self.didcomm = DIDCommAPI(self)
        self.credentials = CredentialAPI(self)
        self.discovery = DiscoveryAPI(self)

    @property
    def token_manager(self) -> TokenManager:
        """Access the token manager for manual token operations."""
        if not self._token_manager:
            raise ValueError("Token manager not initialized. Provide did and private_key.")
        return self._token_manager

    def _authed_request(
        self,
        method: str,
        path: str,
        *,
        json_body: Optional[Dict[str, Any]] = None,
        raw_body: Optional[bytes] = None,
        content_type: Optional[str] = None,
        params: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        """Make an authenticated request using the token manager."""
        if not self._token_manager or not self._signing_key:
            raise ValueError("Authentication not configured. Provide did, private_key, and optionally client_secret.")

        access_token = self._token_manager.get_access_token()
        return self._http.authenticated_request(
            method,
            path,
            signing_key=self._signing_key,
            access_token=access_token,
            platform_domain=self._platform_domain,
            json_body=json_body,
            raw_body=raw_body,
            content_type=content_type,
            params=params,
        )

    def close(self) -> None:
        """Close the HTTP client."""
        self._http.close()

    def __enter__(self) -> "ATAPClient":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()
