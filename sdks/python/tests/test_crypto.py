"""Tests for crypto module."""

import base64
import hashlib
import json

import nacl.signing
import pytest

from atap.crypto import (
    b64url_decode,
    b64url_encode,
    domain_from_did,
    generate_keypair,
    generate_pkce,
    jwk_thumbprint,
    load_signing_key,
    make_dpop_proof,
    public_key_b64,
)


class TestBase64url:
    def test_encode_decode_roundtrip(self) -> None:
        data = b"hello world"
        encoded = b64url_encode(data)
        assert b64url_decode(encoded) == data

    def test_no_padding(self) -> None:
        data = b"test"
        encoded = b64url_encode(data)
        assert "=" not in encoded

    def test_url_safe_chars(self) -> None:
        # Bytes that would produce + and / in standard base64
        data = b"\xfb\xff\xfe"
        encoded = b64url_encode(data)
        assert "+" not in encoded
        assert "/" not in encoded

    def test_empty_input(self) -> None:
        assert b64url_encode(b"") == ""
        assert b64url_decode("") == b""


class TestKeyGeneration:
    def test_generate_keypair(self) -> None:
        sk, vk = generate_keypair()
        assert isinstance(sk, nacl.signing.SigningKey)
        assert isinstance(vk, nacl.signing.VerifyKey)
        assert len(bytes(vk)) == 32

    def test_load_signing_key_32_bytes(self) -> None:
        seed = b"\x01" * 32
        b64 = base64.b64encode(seed).decode()
        sk = load_signing_key(b64)
        assert bytes(sk)[:32] == seed

    def test_load_signing_key_64_bytes(self) -> None:
        seed = b"\x01" * 32
        sk_original = nacl.signing.SigningKey(seed)
        full_key = bytes(sk_original) + bytes(sk_original.verify_key)
        b64 = base64.b64encode(full_key).decode()
        sk = load_signing_key(b64)
        assert bytes(sk.verify_key) == bytes(sk_original.verify_key)

    def test_load_signing_key_invalid_length(self) -> None:
        b64 = base64.b64encode(b"\x01" * 16).decode()
        with pytest.raises(ValueError, match="Invalid private key length"):
            load_signing_key(b64)

    def test_public_key_b64(self, signing_key: nacl.signing.SigningKey) -> None:
        result = public_key_b64(signing_key)
        decoded = base64.b64decode(result)
        assert len(decoded) == 32
        assert decoded == bytes(signing_key.verify_key)


class TestJWKThumbprint:
    def test_deterministic(self, verify_key: nacl.signing.VerifyKey) -> None:
        t1 = jwk_thumbprint(verify_key)
        t2 = jwk_thumbprint(verify_key)
        assert t1 == t2

    def test_different_keys_different_thumbprints(self) -> None:
        sk1 = nacl.signing.SigningKey(b"\x01" * 32)
        sk2 = nacl.signing.SigningKey(b"\x02" * 32)
        assert jwk_thumbprint(sk1.verify_key) != jwk_thumbprint(sk2.verify_key)

    def test_thumbprint_format(self, verify_key: nacl.signing.VerifyKey) -> None:
        t = jwk_thumbprint(verify_key)
        # Should be base64url-encoded SHA-256 (43 chars without padding)
        assert len(t) == 43
        # Should be decodable
        decoded = b64url_decode(t)
        assert len(decoded) == 32  # SHA-256 output


class TestDPoPProof:
    def test_proof_structure(self, signing_key: nacl.signing.SigningKey) -> None:
        proof = make_dpop_proof(signing_key, "POST", "https://localhost:8080/v1/oauth/token")
        parts = proof.split(".")
        assert len(parts) == 3

        header = json.loads(b64url_decode(parts[0]))
        assert header["typ"] == "dpop+jwt"
        assert header["alg"] == "EdDSA"
        assert header["jwk"]["kty"] == "OKP"
        assert header["jwk"]["crv"] == "Ed25519"
        assert "x" in header["jwk"]

        payload = json.loads(b64url_decode(parts[1]))
        assert payload["htm"] == "POST"
        assert payload["htu"] == "https://localhost:8080/v1/oauth/token"
        assert "jti" in payload
        assert "iat" in payload

    def test_proof_with_access_token_hash(self, signing_key: nacl.signing.SigningKey) -> None:
        token = "test_access_token"
        proof = make_dpop_proof(
            signing_key, "GET", "https://localhost:8080/v1/approvals", access_token=token
        )
        parts = proof.split(".")
        payload = json.loads(b64url_decode(parts[1]))
        assert "ath" in payload
        # Verify ath is base64url(SHA-256(token))
        expected_ath = b64url_encode(hashlib.sha256(token.encode("ascii")).digest())
        assert payload["ath"] == expected_ath

    def test_proof_without_access_token_no_ath(self, signing_key: nacl.signing.SigningKey) -> None:
        proof = make_dpop_proof(signing_key, "GET", "https://localhost:8080/v1/health")
        parts = proof.split(".")
        payload = json.loads(b64url_decode(parts[1]))
        assert "ath" not in payload

    def test_proof_signature_verifiable(self, signing_key: nacl.signing.SigningKey) -> None:
        proof = make_dpop_proof(signing_key, "POST", "https://localhost:8080/v1/oauth/token")
        parts = proof.split(".")
        signing_input = f"{parts[0]}.{parts[1]}".encode()
        signature = b64url_decode(parts[2])
        # Should not raise
        signing_key.verify_key.verify(signing_input, signature)

    def test_unique_jti(self, signing_key: nacl.signing.SigningKey) -> None:
        p1 = make_dpop_proof(signing_key, "GET", "https://example.com")
        p2 = make_dpop_proof(signing_key, "GET", "https://example.com")
        payload1 = json.loads(b64url_decode(p1.split(".")[1]))
        payload2 = json.loads(b64url_decode(p2.split(".")[1]))
        assert payload1["jti"] != payload2["jti"]


class TestPKCE:
    def test_generate_pkce(self) -> None:
        verifier, challenge = generate_pkce()
        assert len(verifier) > 0
        assert len(challenge) > 0
        # Verify S256: challenge = base64url(SHA-256(verifier))
        expected = b64url_encode(hashlib.sha256(verifier.encode()).digest())
        assert challenge == expected

    def test_unique_verifiers(self) -> None:
        v1, _ = generate_pkce()
        v2, _ = generate_pkce()
        assert v1 != v2


class TestDomainFromDID:
    def test_standard_did(self) -> None:
        assert domain_from_did("did:web:atap.dev:agent:abc") == "atap.dev"

    def test_did_with_port(self) -> None:
        assert domain_from_did("did:web:localhost%3A8080:agent:abc") == "localhost:8080"

    def test_invalid_did(self) -> None:
        with pytest.raises(ValueError, match="Invalid DID format"):
            domain_from_did("invalid")
