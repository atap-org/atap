"""HTTP client for the ATAP SDK with DPoP authentication."""

from __future__ import annotations

import json
from typing import Any, Dict, Optional
from urllib.parse import urlencode

import httpx

from atap.crypto import make_dpop_proof
from atap.errors import (
    ATAPAuthError,
    ATAPConflictError,
    ATAPError,
    ATAPNotFoundError,
    ATAPProblemError,
    ATAPRateLimitError,
)
from atap.models import ProblemDetail


class HTTPClient:
    """Low-level HTTP client with DPoP proof injection and error handling."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        timeout: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._client = httpx.Client(base_url=self._base_url, timeout=timeout)

    def close(self) -> None:
        """Close the underlying HTTP client."""
        self._client.close()

    def request(
        self,
        method: str,
        path: str,
        *,
        json_body: Optional[Dict[str, Any]] = None,
        headers: Optional[Dict[str, str]] = None,
        params: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        """Make an HTTP request and return parsed JSON response."""
        h = dict(headers or {})
        if json_body is not None:
            h.setdefault("Content-Type", "application/json")

        response = self._client.request(
            method,
            path,
            json=json_body,
            headers=h,
            params=params,
        )
        return self._handle_response(response)

    def authenticated_request(
        self,
        method: str,
        path: str,
        *,
        signing_key: Any,
        access_token: str,
        platform_domain: str,
        json_body: Optional[Dict[str, Any]] = None,
        raw_body: Optional[bytes] = None,
        content_type: Optional[str] = None,
        params: Optional[Dict[str, str]] = None,
    ) -> Dict[str, Any]:
        """Make a DPoP-authenticated HTTP request."""
        url = f"https://{platform_domain}{path}"
        dpop_proof = make_dpop_proof(signing_key, method, url, access_token=access_token)

        headers: Dict[str, str] = {
            "Authorization": f"DPoP {access_token}",
            "DPoP": dpop_proof,
        }

        if content_type:
            headers["Content-Type"] = content_type

        if raw_body is not None:
            response = self._client.request(
                method,
                path,
                content=raw_body,
                headers=headers,
                params=params,
            )
        else:
            response = self._client.request(
                method,
                path,
                json=json_body,
                headers=headers,
                params=params,
            )

        return self._handle_response(response)

    def post_form(
        self,
        path: str,
        form_data: Dict[str, str],
        *,
        dpop_proof: Optional[str] = None,
    ) -> Dict[str, Any]:
        """POST form-encoded data (for OAuth token endpoint)."""
        headers: Dict[str, str] = {
            "Content-Type": "application/x-www-form-urlencoded",
        }
        if dpop_proof:
            headers["DPoP"] = dpop_proof

        response = self._client.post(
            path,
            content=urlencode(form_data).encode(),
            headers=headers,
        )
        return self._handle_response(response)

    def get_redirect(
        self,
        path: str,
        *,
        params: Optional[Dict[str, str]] = None,
        dpop_proof: Optional[str] = None,
    ) -> str:
        """GET request expecting a 302 redirect, returns the Location URL."""
        headers: Dict[str, str] = {}
        if dpop_proof:
            headers["DPoP"] = dpop_proof

        response = self._client.get(
            path,
            params=params,
            headers=headers,
            follow_redirects=False,
        )

        if response.status_code != 302:
            self._handle_response(response)
            raise ATAPError(
                f"Expected 302 redirect, got {response.status_code}",
                response.status_code,
            )

        location = response.headers.get("location", "")
        if not location:
            raise ATAPError("302 redirect with no Location header")
        return location

    def _handle_response(self, response: httpx.Response) -> Dict[str, Any]:
        """Parse response, raising typed errors for non-2xx status codes."""
        if response.status_code == 204:
            return {}

        try:
            data = response.json()
        except (json.JSONDecodeError, ValueError):
            if 200 <= response.status_code < 300:
                return {}
            raise ATAPError(
                f"HTTP {response.status_code}: {response.text}",
                response.status_code,
            )

        if 200 <= response.status_code < 300:
            return data

        # Parse RFC 7807 Problem Detail
        problem = None
        if "type" in data and "status" in data:
            problem = ProblemDetail(
                type=data.get("type", ""),
                title=data.get("title", ""),
                status=data.get("status", response.status_code),
                detail=data.get("detail"),
                instance=data.get("instance"),
            )

        status = response.status_code

        if status == 401 or status == 403:
            msg = problem.detail if problem else data.get("detail", "Authentication failed")
            raise ATAPAuthError(str(msg), status, problem)
        elif status == 404:
            msg = problem.detail if problem else "Not found"
            raise ATAPNotFoundError(str(msg), problem)
        elif status == 409:
            msg = problem.detail if problem else "Conflict"
            raise ATAPConflictError(str(msg), problem)
        elif status == 429:
            msg = problem.detail if problem else "Rate limit exceeded"
            raise ATAPRateLimitError(str(msg), problem)
        elif problem:
            raise ATAPProblemError(problem)
        else:
            raise ATAPError(
                f"HTTP {status}: {data.get('detail', data.get('message', str(data)))}",
                status,
            )
