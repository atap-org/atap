"""Tests for HTTP client."""

import json
from typing import Any, Dict

import httpx
import nacl.signing
import pytest
import respx

from atap.errors import (
    ATAPAuthError,
    ATAPConflictError,
    ATAPError,
    ATAPNotFoundError,
    ATAPProblemError,
    ATAPRateLimitError,
)
from atap.http_client import HTTPClient
from atap.models import ProblemDetail


@pytest.fixture
def http_client(respx_mock: respx.MockRouter) -> HTTPClient:
    return HTTPClient(base_url="http://localhost:8080")


class TestRequest:
    def test_get_json(self, respx_mock: respx.MockRouter, http_client: HTTPClient) -> None:
        respx_mock.get("/v1/health").mock(
            return_value=httpx.Response(200, json={"status": "ok"})
        )
        result = http_client.request("GET", "/v1/health")
        assert result["status"] == "ok"

    def test_post_json(self, respx_mock: respx.MockRouter, http_client: HTTPClient) -> None:
        respx_mock.post("/v1/entities").mock(
            return_value=httpx.Response(201, json={"id": "test123", "type": "agent"})
        )
        result = http_client.request("POST", "/v1/entities", json_body={"type": "agent"})
        assert result["id"] == "test123"

    def test_204_no_content(self, respx_mock: respx.MockRouter, http_client: HTTPClient) -> None:
        respx_mock.delete("/v1/entities/test123").mock(
            return_value=httpx.Response(204)
        )
        result = http_client.request("DELETE", "/v1/entities/test123")
        assert result == {}

    def test_params_forwarded(self, respx_mock: respx.MockRouter, http_client: HTTPClient) -> None:
        route = respx_mock.get("/v1/revocations").mock(
            return_value=httpx.Response(200, json={"revocations": []})
        )
        http_client.request("GET", "/v1/revocations", params={"entity": "did:web:test"})
        assert "entity=did" in str(route.calls[0].request.url)


class TestAuthenticatedRequest:
    def test_dpop_headers(
        self,
        respx_mock: respx.MockRouter,
        http_client: HTTPClient,
        signing_key: nacl.signing.SigningKey,
    ) -> None:
        route = respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(200, json=[])
        )
        http_client.authenticated_request(
            "GET",
            "/v1/approvals",
            signing_key=signing_key,
            access_token="test_token",
            platform_domain="localhost:8080",
        )
        headers = dict(route.calls[0].request.headers)
        assert "authorization" in headers
        assert headers["authorization"].startswith("DPoP ")
        assert "dpop" in headers


class TestPostForm:
    def test_form_encoded(self, respx_mock: respx.MockRouter, http_client: HTTPClient) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json={"access_token": "test"})
        )
        http_client.post_form("/v1/oauth/token", {"grant_type": "client_credentials"})
        request = route.calls[0].request
        assert b"grant_type=client_credentials" in request.content

    def test_dpop_header_included(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        route = respx_mock.post("/v1/oauth/token").mock(
            return_value=httpx.Response(200, json={"access_token": "test"})
        )
        http_client.post_form("/v1/oauth/token", {}, dpop_proof="test_proof")
        headers = dict(route.calls[0].request.headers)
        assert headers["dpop"] == "test_proof"


class TestGetRedirect:
    def test_302_returns_location(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/oauth/authorize").mock(
            return_value=httpx.Response(302, headers={"location": "https://example.com/callback?code=abc"})
        )
        location = http_client.get_redirect("/v1/oauth/authorize")
        assert "code=abc" in location

    def test_non_302_raises(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/oauth/authorize").mock(
            return_value=httpx.Response(400, json={"type": "bad-request", "title": "Bad", "status": 400})
        )
        with pytest.raises((ATAPError, ATAPProblemError)):
            http_client.get_redirect("/v1/oauth/authorize")


class TestErrorHandling:
    def test_401_raises_auth_error(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(
                401,
                json={
                    "type": "https://atap.dev/errors/unauthorized",
                    "title": "Unauthorized",
                    "status": 401,
                    "detail": "Invalid token",
                },
            )
        )
        with pytest.raises(ATAPAuthError) as exc_info:
            http_client.request("GET", "/v1/approvals")
        assert exc_info.value.status_code == 401
        assert exc_info.value.problem is not None

    def test_403_raises_auth_error(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/approvals").mock(
            return_value=httpx.Response(
                403,
                json={
                    "type": "https://atap.dev/errors/insufficient_scope",
                    "title": "Forbidden",
                    "status": 403,
                    "detail": "Insufficient scope",
                },
            )
        )
        with pytest.raises(ATAPAuthError) as exc_info:
            http_client.request("GET", "/v1/approvals")
        assert exc_info.value.status_code == 403

    def test_404_raises_not_found(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/entities/missing").mock(
            return_value=httpx.Response(
                404,
                json={
                    "type": "https://atap.dev/errors/not-found",
                    "title": "Not Found",
                    "status": 404,
                    "detail": "Entity not found",
                },
            )
        )
        with pytest.raises(ATAPNotFoundError):
            http_client.request("GET", "/v1/entities/missing")

    def test_409_raises_conflict(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.post("/v1/approvals/apr_123/respond").mock(
            return_value=httpx.Response(
                409,
                json={
                    "type": "https://atap.dev/errors/conflict",
                    "title": "Conflict",
                    "status": 409,
                    "detail": "Already responded",
                },
            )
        )
        with pytest.raises(ATAPConflictError):
            http_client.request("POST", "/v1/approvals/apr_123/respond", json_body={})

    def test_429_raises_rate_limit(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.post("/v1/credentials/email/start").mock(
            return_value=httpx.Response(
                429,
                json={
                    "type": "https://atap.dev/errors/rate-limit-exceeded",
                    "title": "Too Many Requests",
                    "status": 429,
                    "detail": "Rate limit exceeded",
                },
            )
        )
        with pytest.raises(ATAPRateLimitError):
            http_client.request("POST", "/v1/credentials/email/start", json_body={})

    def test_500_raises_problem_error(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/health").mock(
            return_value=httpx.Response(
                500,
                json={
                    "type": "https://atap.dev/errors/internal",
                    "title": "Internal Server Error",
                    "status": 500,
                    "detail": "Something went wrong",
                },
            )
        )
        with pytest.raises(ATAPProblemError) as exc_info:
            http_client.request("GET", "/v1/health")
        assert exc_info.value.problem.status == 500

    def test_non_json_error(
        self, respx_mock: respx.MockRouter, http_client: HTTPClient
    ) -> None:
        respx_mock.get("/v1/health").mock(
            return_value=httpx.Response(502, text="Bad Gateway")
        )
        with pytest.raises(ATAPError, match="502"):
            http_client.request("GET", "/v1/health")
