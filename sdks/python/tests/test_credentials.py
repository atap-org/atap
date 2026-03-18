"""Tests for credential operations."""

import httpx
import pytest
import respx

from atap.client import ATAPClient
from atap.errors import ATAPRateLimitError


class TestEmailVerification:
    def test_start_email(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/email/start").mock(
            return_value=httpx.Response(200, json={"message": "OTP sent to email"})
        )
        msg = authed_client.credentials.start_email_verification("test@example.com")
        assert "OTP" in msg

    def test_verify_email(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/email/verify").mock(
            return_value=httpx.Response(201, json={
                "id": "crd_abc123",
                "type": "ATAPEmailVerification",
                "credential": "eyJ...",
                "issued_at": "2026-03-18T10:00:00Z",
            })
        )
        cred = authed_client.credentials.verify_email("test@example.com", "123456")
        assert cred.type == "ATAPEmailVerification"
        assert cred.credential.startswith("eyJ")

    def test_email_rate_limit(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/email/start").mock(
            return_value=httpx.Response(429, json={
                "type": "https://atap.dev/errors/rate-limit-exceeded",
                "title": "Too Many Requests",
                "status": 429,
                "detail": "Rate limit exceeded",
            })
        )
        with pytest.raises(ATAPRateLimitError):
            authed_client.credentials.start_email_verification("test@example.com")


class TestPhoneVerification:
    def test_start_phone(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/phone/start").mock(
            return_value=httpx.Response(200, json={"message": "OTP sent to phone"})
        )
        msg = authed_client.credentials.start_phone_verification("+1234567890")
        assert "OTP" in msg

    def test_verify_phone(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/phone/verify").mock(
            return_value=httpx.Response(201, json={
                "id": "crd_phone123",
                "type": "ATAPPhoneVerification",
                "credential": "eyJ...",
                "issued_at": "2026-03-18T10:00:00Z",
            })
        )
        cred = authed_client.credentials.verify_phone("+1234567890", "654321")
        assert cred.type == "ATAPPhoneVerification"


class TestPersonhood:
    def test_submit_personhood(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.post("/v1/credentials/personhood").mock(
            return_value=httpx.Response(201, json={
                "id": "crd_person123",
                "type": "ATAPPersonhood",
                "credential": "eyJ...",
                "issued_at": "2026-03-18T10:00:00Z",
            })
        )
        cred = authed_client.credentials.submit_personhood()
        assert cred.type == "ATAPPersonhood"


class TestListCredentials:
    def test_list(
        self,
        respx_mock: respx.MockRouter,
        authed_client: ATAPClient,
    ) -> None:
        respx_mock.get("/v1/credentials").mock(
            return_value=httpx.Response(200, json=[
                {
                    "id": "crd_1",
                    "type": "ATAPEmailVerification",
                    "credential": "eyJ...",
                    "issued_at": "2026-03-18T10:00:00Z",
                },
                {
                    "id": "crd_2",
                    "type": "ATAPPhoneVerification",
                    "credential": "eyJ...",
                    "issued_at": "2026-03-18T11:00:00Z",
                },
            ])
        )
        creds = authed_client.credentials.list()
        assert len(creds) == 2
        assert creds[0].type == "ATAPEmailVerification"
        assert creds[1].type == "ATAPPhoneVerification"


class TestStatusList:
    def test_status_list(self, respx_mock: respx.MockRouter) -> None:
        respx_mock.get("/v1/credentials/status/1").mock(
            return_value=httpx.Response(200, json={
                "@context": ["https://www.w3.org/ns/credentials/v2"],
                "type": ["VerifiableCredential", "BitstringStatusListCredential"],
            })
        )
        client = ATAPClient(base_url="http://localhost:8080")
        result = client.credentials.status_list("1")
        assert "BitstringStatusListCredential" in result["type"]
