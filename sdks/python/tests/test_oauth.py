"""Tests for OAuth token management."""

import json
import time
from typing import Any, Dict
from unittest.mock import MagicMock, patch

import httpx
import nacl.signing
import pytest
import respx

from atap.crypto import b64url_decode
from atap.http_client import HTTPClient
from atap.oauth import TokenManager


@pytest.fixture
def http_client(respx_mock: respx.MockRouter) -> HTTPClient:
    return HTTPClient(base_url="http://localhost:8080")


@pytest.fixture
def token_manager(
    http_client: HTTPClient,
    signing_key: nacl.signing.SigningKey,
    test_did: str,
    test_secret: str,
) -> TokenManager:
    return TokenManager(
        http_client=http_client,
        signing_key=signing_key,
        did=test_did,
        client_secret=test_secret,
        scopes=["atap:inbox", "atap:send"],
    )


class TestTokenManagerClientCredentials:
    def test_obtain_token(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
    ) -> None:
        respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token = token_manager.get_access_token()
        assert token == mock_token_response["access_token"]

    def test_token_cached(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token_manager.get_access_token()
        token_manager.get_access_token()
        assert route.call_count == 1

    def test_token_refresh_when_expired(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
    ) -> None:
        respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token_manager.get_access_token()
        # Simulate token expiry
        token_manager._token_obtained_at = time.time() - 3600
        token_manager.get_access_token()

    def test_dpop_proof_included(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token_manager.get_access_token()
        request = route.calls[0].request
        # httpx lowercases headers
        assert "dpop" in dict(request.headers)

    def test_form_data_correct(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
        test_did: str,
        test_secret: str,
    ) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token_manager.get_access_token()
        request = route.calls[0].request
        body = request.content.decode()
        assert "grant_type=client_credentials" in body
        assert "client_id=did" in body

    def test_no_secret_raises(
        self,
        http_client: HTTPClient,
        signing_key: nacl.signing.SigningKey,
        test_did: str,
    ) -> None:
        tm = TokenManager(
            http_client=http_client,
            signing_key=signing_key,
            did=test_did,
            client_secret=None,
        )
        with pytest.raises(ValueError, match="client_secret is required"):
            tm.get_access_token()


class TestTokenManagerInvalidate:
    def test_invalidate_clears_token(
        self,
        respx_mock: respx.MockRouter,
        token_manager: TokenManager,
        mock_token_response: Dict[str, Any],
    ) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )
        token_manager.get_access_token()
        token_manager.invalidate()
        token_manager.get_access_token()
        assert route.call_count == 2


class TestTokenManagerAuthorizationCode:
    def test_obtain_authorization_code(
        self,
        respx_mock: respx.MockRouter,
        http_client: HTTPClient,
        signing_key: nacl.signing.SigningKey,
        mock_token_response: Dict[str, Any],
    ) -> None:
        did = "did:web:localhost%3A8080:human:test123"
        tm = TokenManager(
            http_client=http_client,
            signing_key=signing_key,
            did=did,
        )

        # Mock authorize endpoint (302 redirect with code)
        respx_mock.get("/v1/oauth/authorize").mock(
            return_value=httpx.Response(
                302,
                headers={"location": "atap://callback?code=test_auth_code&state="},
            )
        )
        # Mock token endpoint
        respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json=mock_token_response)
        )

        token = tm.obtain_authorization_code()
        assert token.access_token == mock_token_response["access_token"]
