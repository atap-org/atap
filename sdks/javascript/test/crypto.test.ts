import { describe, expect, it } from "vitest";
import {
  b64urlDecode,
  b64urlEncode,
  domainFromDID,
  generateKeypair,
  generatePKCE,
  jwkThumbprint,
  loadSigningKey,
  makeDPoPProof,
} from "../src/crypto.js";

describe("b64urlEncode / b64urlDecode", () => {
  it("round-trips binary data", () => {
    const data = new Uint8Array([0, 1, 2, 255, 254, 253]);
    const encoded = b64urlEncode(data);
    expect(encoded).not.toContain("+");
    expect(encoded).not.toContain("/");
    expect(encoded).not.toContain("=");
    const decoded = b64urlDecode(encoded);
    expect(decoded).toEqual(data);
  });

  it("handles empty data", () => {
    const data = new Uint8Array(0);
    const encoded = b64urlEncode(data);
    const decoded = b64urlDecode(encoded);
    expect(decoded).toEqual(data);
  });

  it("encodes known value correctly", () => {
    // "Hello" in base64url should be "SGVsbG8"
    const data = new TextEncoder().encode("Hello");
    const encoded = b64urlEncode(data);
    expect(encoded).toBe("SGVsbG8");
  });
});

describe("generateKeypair", () => {
  it("generates a 32-byte private key and 32-byte public key", async () => {
    const { privateKey, publicKey } = await generateKeypair();
    expect(privateKey).toHaveLength(32);
    expect(publicKey).toHaveLength(32);
  });

  it("generates unique keypairs each time", async () => {
    const kp1 = await generateKeypair();
    const kp2 = await generateKeypair();
    expect(b64urlEncode(kp1.privateKey)).not.toBe(b64urlEncode(kp2.privateKey));
  });
});

describe("loadSigningKey", () => {
  it("loads a 32-byte seed", () => {
    // Create a base64-encoded 32-byte key
    const seed = new Uint8Array(32);
    seed.fill(42);
    const b64 = btoa(String.fromCharCode(...seed));
    const key = loadSigningKey(b64);
    expect(key).toHaveLength(32);
    expect(key).toEqual(seed);
  });

  it("loads a 64-byte full key (takes first 32 bytes)", () => {
    const fullKey = new Uint8Array(64);
    fullKey.fill(42, 0, 32);
    fullKey.fill(99, 32, 64);
    const b64 = btoa(String.fromCharCode(...fullKey));
    const key = loadSigningKey(b64);
    expect(key).toHaveLength(32);
    expect(key).toEqual(fullKey.slice(0, 32));
  });

  it("throws on invalid key length", () => {
    const bad = new Uint8Array(16);
    const b64 = btoa(String.fromCharCode(...bad));
    expect(() => loadSigningKey(b64)).toThrow("Invalid private key length");
  });
});

describe("jwkThumbprint", () => {
  it("produces a deterministic thumbprint", async () => {
    const { publicKey } = await generateKeypair();
    const t1 = jwkThumbprint(publicKey);
    const t2 = jwkThumbprint(publicKey);
    expect(t1).toBe(t2);
    expect(t1.length).toBeGreaterThan(0);
    // Should be base64url (no +, /, =)
    expect(t1).not.toContain("+");
    expect(t1).not.toContain("/");
    expect(t1).not.toContain("=");
  });

  it("produces different thumbprints for different keys", async () => {
    const kp1 = await generateKeypair();
    const kp2 = await generateKeypair();
    expect(jwkThumbprint(kp1.publicKey)).not.toBe(
      jwkThumbprint(kp2.publicKey),
    );
  });
});

describe("makeDPoPProof", () => {
  it("creates a valid JWT with three parts", async () => {
    const { privateKey } = await generateKeypair();
    const proof = await makeDPoPProof(
      privateKey,
      "POST",
      "https://example.com/v1/oauth/token",
    );
    const parts = proof.split(".");
    expect(parts).toHaveLength(3);

    // Decode header
    const header = JSON.parse(
      new TextDecoder().decode(b64urlDecode(parts[0])),
    );
    expect(header.typ).toBe("dpop+jwt");
    expect(header.alg).toBe("EdDSA");
    expect(header.jwk).toBeDefined();
    expect(header.jwk.kty).toBe("OKP");
    expect(header.jwk.crv).toBe("Ed25519");

    // Decode payload
    const payload = JSON.parse(
      new TextDecoder().decode(b64urlDecode(parts[1])),
    );
    expect(payload.htm).toBe("POST");
    expect(payload.htu).toBe("https://example.com/v1/oauth/token");
    expect(payload.jti).toBeDefined();
    expect(payload.iat).toBeDefined();
  });

  it("includes ath claim when access_token provided", async () => {
    const { privateKey } = await generateKeypair();
    const proof = await makeDPoPProof(
      privateKey,
      "GET",
      "https://example.com/v1/resource",
      "my_access_token",
    );
    const parts = proof.split(".");
    const payload = JSON.parse(
      new TextDecoder().decode(b64urlDecode(parts[1])),
    );
    expect(payload.ath).toBeDefined();
    expect(typeof payload.ath).toBe("string");
  });

  it("omits ath claim when no access_token", async () => {
    const { privateKey } = await generateKeypair();
    const proof = await makeDPoPProof(
      privateKey,
      "GET",
      "https://example.com/v1/resource",
    );
    const parts = proof.split(".");
    const payload = JSON.parse(
      new TextDecoder().decode(b64urlDecode(parts[1])),
    );
    expect(payload.ath).toBeUndefined();
  });
});

describe("generatePKCE", () => {
  it("generates verifier and challenge", () => {
    const { verifier, challenge } = generatePKCE();
    expect(verifier.length).toBeGreaterThan(0);
    expect(challenge.length).toBeGreaterThan(0);
    expect(verifier).not.toBe(challenge);
  });

  it("challenge is base64url encoded SHA-256 of verifier", () => {
    const { verifier, challenge } = generatePKCE();
    // Both should be base64url safe
    expect(verifier).not.toContain("+");
    expect(verifier).not.toContain("/");
    expect(challenge).not.toContain("+");
    expect(challenge).not.toContain("/");
  });

  it("generates unique values each time", () => {
    const pkce1 = generatePKCE();
    const pkce2 = generatePKCE();
    expect(pkce1.verifier).not.toBe(pkce2.verifier);
  });
});

describe("domainFromDID", () => {
  it("extracts domain from standard DID", () => {
    expect(
      domainFromDID("did:web:localhost%3A8080:agent:abc"),
    ).toBe("localhost:8080");
  });

  it("extracts domain without port", () => {
    expect(domainFromDID("did:web:example.com:agent:abc")).toBe("example.com");
  });

  it("throws on invalid DID", () => {
    expect(() => domainFromDID("invalid")).toThrow("Invalid DID format");
    expect(() => domainFromDID("did:web")).toThrow("Invalid DID format");
  });

  it("handles multiple %3A replacements", () => {
    expect(domainFromDID("did:web:host%3A8080:agent:x")).toBe("host:8080");
  });
});
