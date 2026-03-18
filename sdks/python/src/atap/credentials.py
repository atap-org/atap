"""Credential (W3C Verifiable Credentials) operations."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

from atap.models import Credential


class CredentialAPI:
    """Email/phone/personhood verification and credential management."""

    def __init__(self, client: Any) -> None:
        self._client = client

    def start_email_verification(self, email: str) -> str:
        """Initiate email verification (OTP). Requires atap:manage scope.

        Args:
            email: Email address to verify.

        Returns:
            Status message.
        """
        data = self._client._authed_request(
            "POST", "/v1/credentials/email/start", json_body={"email": email}
        )
        return data.get("message", "OTP sent")

    def verify_email(self, email: str, otp: str) -> Credential:
        """Verify email with OTP, issuing ATAPEmailVerification VC. Requires atap:manage scope.

        Args:
            email: Email address.
            otp: The OTP code.

        Returns:
            Credential with VC JWT.
        """
        data = self._client._authed_request(
            "POST",
            "/v1/credentials/email/verify",
            json_body={"email": email, "otp": otp},
        )
        return _parse_credential(data)

    def start_phone_verification(self, phone: str) -> str:
        """Initiate phone verification (OTP). Requires atap:manage scope.

        Args:
            phone: Phone number (E.164 format).

        Returns:
            Status message.
        """
        data = self._client._authed_request(
            "POST", "/v1/credentials/phone/start", json_body={"phone": phone}
        )
        return data.get("message", "OTP sent")

    def verify_phone(self, phone: str, otp: str) -> Credential:
        """Verify phone with OTP, issuing ATAPPhoneVerification VC. Requires atap:manage scope.

        Args:
            phone: Phone number.
            otp: The OTP code.

        Returns:
            Credential with VC JWT.
        """
        data = self._client._authed_request(
            "POST",
            "/v1/credentials/phone/verify",
            json_body={"phone": phone, "otp": otp},
        )
        return _parse_credential(data)

    def submit_personhood(self, *, provider_token: Optional[str] = None) -> Credential:
        """Submit personhood attestation, issuing ATAPPersonhood VC. Requires atap:manage scope.

        Args:
            provider_token: Optional provider token.

        Returns:
            Credential with VC JWT.
        """
        body: Dict[str, Any] = {}
        if provider_token:
            body["provider_token"] = provider_token

        data = self._client._authed_request(
            "POST", "/v1/credentials/personhood", json_body=body
        )
        return _parse_credential(data)

    def list(self) -> List[Credential]:
        """List credentials for the authenticated entity. Requires atap:manage scope."""
        data = self._client._authed_request("GET", "/v1/credentials")
        if isinstance(data, list):
            return [_parse_credential(c) for c in data]
        return [_parse_credential(c) for c in data.get("credentials", [])]

    def status_list(self, list_id: str = "1") -> Dict[str, Any]:
        """Get W3C Bitstring Status List VC (public endpoint).

        Args:
            list_id: Status list ID (default "1").
        """
        return self._client._http.request("GET", f"/v1/credentials/status/{list_id}")


def _parse_credential(data: Dict[str, Any]) -> Credential:
    return Credential(
        id=data.get("id", ""),
        type=data.get("type", ""),
        credential=data.get("credential", ""),
        issued_at=data.get("issued_at", ""),
        revoked_at=data.get("revoked_at"),
    )
