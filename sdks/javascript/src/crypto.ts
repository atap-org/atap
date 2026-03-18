/**
 * Cryptographic operations for the ATAP SDK.
 *
 * Handles Ed25519 key generation, DPoP proof creation (RFC 9449),
 * JWK thumbprint computation (RFC 7638), and PKCE S256 challenges.
 */

import * as ed from "@noble/ed25519";
import { sha256 } from "@noble/hashes/sha256";

/** Base64url encode without padding. */
export function b64urlEncode(data: Uint8Array): string {
  const base64 = btoa(String.fromCharCode(...data));
  return base64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** Base64url decode with padding restoration. */
export function b64urlDecode(s: string): Uint8Array {
  let padded = s.replace(/-/g, "+").replace(/_/g, "/");
  const padding = 4 - (padded.length % 4);
  if (padding !== 4) {
    padded += "=".repeat(padding);
  }
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/** Standard base64 decode (not URL-safe). */
function base64Decode(s: string): Uint8Array {
  let padded = s;
  const padding = 4 - (padded.length % 4);
  if (padding !== 4) {
    padded += "=".repeat(padding);
  }
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/** Generate a new Ed25519 keypair. Returns [privateKey (32-byte seed), publicKey (32 bytes)]. */
export async function generateKeypair(): Promise<{
  privateKey: Uint8Array;
  publicKey: Uint8Array;
}> {
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = await ed.getPublicKeyAsync(privateKey);
  return { privateKey, publicKey };
}

/**
 * Load an Ed25519 signing key from base64.
 * Accepts 32-byte seed or 64-byte full key (uses first 32 bytes as seed).
 */
export function loadSigningKey(b64: string): Uint8Array {
  const raw = base64Decode(b64);
  if (raw.length === 64) {
    return raw.slice(0, 32);
  } else if (raw.length === 32) {
    return raw;
  }
  throw new Error(
    `Invalid private key length: ${raw.length} bytes (expected 32 or 64)`,
  );
}

/** Compute JWK thumbprint (RFC 7638) for an Ed25519 public key. */
export function jwkThumbprint(publicKey: Uint8Array): string {
  const x = b64urlEncode(publicKey);
  const canonical = JSON.stringify({ crv: "Ed25519", kty: "OKP", x });
  const digest = sha256(new TextEncoder().encode(canonical));
  return b64urlEncode(digest);
}

/**
 * Create a DPoP proof JWT (RFC 9449).
 *
 * @param privateKey - Ed25519 private key (32-byte seed).
 * @param method - HTTP method (GET, POST, etc.).
 * @param url - Full URL for the htu claim (must use https://{platformDomain}/path).
 * @param accessToken - If provided, includes ath (access token hash) claim.
 * @returns Compact JWS string (header.payload.signature).
 */
export async function makeDPoPProof(
  privateKey: Uint8Array,
  method: string,
  url: string,
  accessToken?: string,
): Promise<string> {
  const publicKey = await ed.getPublicKeyAsync(privateKey);
  const x = b64urlEncode(publicKey);

  const header = {
    typ: "dpop+jwt",
    alg: "EdDSA",
    jwk: { kty: "OKP", crv: "Ed25519", x },
  };

  const payload: Record<string, unknown> = {
    jti: crypto.randomUUID(),
    htm: method,
    htu: url,
    iat: Math.floor(Date.now() / 1000),
  };

  if (accessToken) {
    const ath = sha256(new TextEncoder().encode(accessToken));
    payload.ath = b64urlEncode(ath);
  }

  const headerB64 = b64urlEncode(
    new TextEncoder().encode(JSON.stringify(header)),
  );
  const payloadB64 = b64urlEncode(
    new TextEncoder().encode(JSON.stringify(payload)),
  );
  const signingInput = new TextEncoder().encode(`${headerB64}.${payloadB64}`);

  const signature = await ed.signAsync(signingInput, privateKey);
  const sigB64 = b64urlEncode(signature);

  return `${headerB64}.${payloadB64}.${sigB64}`;
}

/** Generate PKCE code verifier and S256 challenge. */
export function generatePKCE(): { verifier: string; challenge: string } {
  const randomBytes = new Uint8Array(32);
  crypto.getRandomValues(randomBytes);
  const verifier = b64urlEncode(randomBytes);
  const challenge = b64urlEncode(
    sha256(new TextEncoder().encode(verifier)),
  );
  return { verifier, challenge };
}

/**
 * Extract platform domain from a DID.
 *
 * did:web:localhost%3A8080:agent:abc -> localhost:8080
 */
export function domainFromDID(did: string): string {
  const parts = did.split(":");
  if (parts.length < 3) {
    throw new Error(`Invalid DID format: ${did}`);
  }
  return parts[2].replace(/%3A/g, ":");
}
