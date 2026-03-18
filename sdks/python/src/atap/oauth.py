"""OAuth 2.1 + DPoP token management for the ATAP SDK."""

from __future__ import annotations

import threading
import time
from typing import List, Optional

import nacl.signing

from atap.crypto import domain_from_did, generate_pkce, make_dpop_proof
from atap.models import OAuthToken


class TokenManager:
    """Manages OAuth 2.1 tokens with DPoP binding and auto-refresh.

    Supports client_credentials (agent/machine) and
    authorization_code+PKCE (human/org) grant types.
    """

    def __init__(
        self,
        *,
        http_client: "HTTPClient",  # type: ignore[name-defined]  # noqa: F821
        signing_key: nacl.signing.SigningKey,
        did: str,
        client_secret: Optional[str] = None,
        scopes: Optional[List[str]] = None,
        platform_domain: Optional[str] = None,
    ) -> None:
        self._http = http_client
        self._signing_key = signing_key
        self._did = did
        self._client_secret = client_secret
        self._scopes = scopes or ["atap:inbox", "atap:send", "atap:revoke", "atap:manage"]
        self._platform_domain = platform_domain or domain_from_did(did)
        self._token: Optional[OAuthToken] = None
        self._token_obtained_at: float = 0
        self._lock = threading.Lock()

    @property
    def _token_url(self) -> str:
        return f"https://{self._platform_domain}/v1/oauth/token"

    def get_access_token(self) -> str:
        """Get a valid access token, refreshing if needed."""
        with self._lock:
            if self._token and not self._is_expired():
                return self._token.access_token
            if self._token and self._token.refresh_token:
                return self._refresh().access_token
            return self._obtain().access_token

    def _is_expired(self) -> bool:
        if not self._token:
            return True
        elapsed = time.time() - self._token_obtained_at
        # Refresh 60 seconds before expiry
        return elapsed >= (self._token.expires_in - 60)

    def _obtain(self) -> OAuthToken:
        """Obtain a new token via client_credentials grant."""
        if not self._client_secret:
            raise ValueError(
                "client_secret is required for client_credentials grant. "
                "For human/org entities, use obtain_authorization_code() instead."
            )

        dpop_proof = make_dpop_proof(self._signing_key, "POST", self._token_url)

        form_data = {
            "grant_type": "client_credentials",
            "client_id": self._did,
            "client_secret": self._client_secret,
            "scope": " ".join(self._scopes),
        }

        data = self._http.post_form("/v1/oauth/token", form_data, dpop_proof=dpop_proof)
        self._token = OAuthToken(
            access_token=data["access_token"],
            token_type=data.get("token_type", "DPoP"),
            expires_in=data.get("expires_in", 3600),
            scope=data.get("scope", ""),
            refresh_token=data.get("refresh_token"),
        )
        self._token_obtained_at = time.time()
        return self._token

    def _refresh(self) -> OAuthToken:
        """Refresh an expired token using the refresh token."""
        if not self._token or not self._token.refresh_token:
            return self._obtain()

        dpop_proof = make_dpop_proof(self._signing_key, "POST", self._token_url)

        form_data = {
            "grant_type": "refresh_token",
            "refresh_token": self._token.refresh_token,
        }

        data = self._http.post_form("/v1/oauth/token", form_data, dpop_proof=dpop_proof)
        self._token = OAuthToken(
            access_token=data["access_token"],
            token_type=data.get("token_type", "DPoP"),
            expires_in=data.get("expires_in", 3600),
            scope=data.get("scope", ""),
            refresh_token=data.get("refresh_token", self._token.refresh_token),
        )
        self._token_obtained_at = time.time()
        return self._token

    def obtain_authorization_code(
        self,
        redirect_uri: str = "atap://callback",
    ) -> OAuthToken:
        """Obtain a token via authorization_code + PKCE flow (for human/org).

        This performs both the authorize and token exchange steps.
        """
        verifier, challenge = generate_pkce()
        authorize_url = f"https://{self._platform_domain}/v1/oauth/authorize"

        dpop_proof = make_dpop_proof(self._signing_key, "GET", authorize_url)

        params = {
            "response_type": "code",
            "client_id": self._did,
            "redirect_uri": redirect_uri,
            "scope": " ".join(self._scopes),
            "code_challenge": challenge,
            "code_challenge_method": "S256",
        }

        redirect_location = self._http.get_redirect(
            "/v1/oauth/authorize", params=params, dpop_proof=dpop_proof
        )

        # Extract code from redirect URL
        from urllib.parse import parse_qs, urlparse

        parsed = urlparse(redirect_location)
        qs = parse_qs(parsed.query)
        code = qs.get("code", [None])[0]
        if not code:
            raise ValueError(f"No authorization code in redirect: {redirect_location}")

        # Exchange code for token
        dpop_proof2 = make_dpop_proof(self._signing_key, "POST", self._token_url)
        form_data = {
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": redirect_uri,
            "code_verifier": verifier,
        }

        with self._lock:
            data = self._http.post_form("/v1/oauth/token", form_data, dpop_proof=dpop_proof2)
            self._token = OAuthToken(
                access_token=data["access_token"],
                token_type=data.get("token_type", "DPoP"),
                expires_in=data.get("expires_in", 3600),
                scope=data.get("scope", ""),
                refresh_token=data.get("refresh_token"),
            )
            self._token_obtained_at = time.time()
            return self._token

    def invalidate(self) -> None:
        """Clear cached token, forcing re-authentication on next request."""
        with self._lock:
            self._token = None
            self._token_obtained_at = 0
