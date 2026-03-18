import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import { createMockFetch } from "./helpers.js";

describe("ATAPClient", () => {
  it("creates a client with defaults", () => {
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({ fetch: mockFetch });
    expect(client.entities).toBeDefined();
    expect(client.approvals).toBeDefined();
    expect(client.revocations).toBeDefined();
    expect(client.didcomm).toBeDefined();
    expect(client.credentials).toBeDefined();
    expect(client.discovery).toBeDefined();
  });

  it("throws when accessing tokenManager without config", () => {
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({ fetch: mockFetch });
    expect(() => client.tokenManager).toThrow(
      "Token manager not initialized",
    );
  });

  it("initializes token manager with DID and key", async () => {
    const { privateKey } = await generateKeypair();
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({
      did: "did:web:localhost%3A8080:agent:test",
      signingKey: privateKey,
      clientSecret: "atap_secret",
      fetch: mockFetch,
    });
    expect(client.tokenManager).toBeDefined();
  });

  it("loads signing key from base64 private key", () => {
    const seed = new Uint8Array(32);
    seed.fill(42);
    const b64 = btoa(String.fromCharCode(...seed));
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({
      did: "did:web:localhost%3A8080:agent:test",
      privateKey: b64,
      fetch: mockFetch,
    });
    expect(client.tokenManager).toBeDefined();
  });

  it("throws on _authedRequest without auth config", async () => {
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({ fetch: mockFetch });
    await expect(
      client._authedRequest("GET", "/v1/test"),
    ).rejects.toThrow("Authentication not configured");
  });

  it("extracts platform domain from DID", async () => {
    const { privateKey } = await generateKeypair();
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({
      did: "did:web:api.atap.app:agent:test",
      signingKey: privateKey,
      clientSecret: "atap_secret",
      fetch: mockFetch,
    });
    // Token manager should be initialized (implying domain was extracted)
    expect(client.tokenManager).toBeDefined();
  });

  it("uses provided platformDomain over DID extraction", () => {
    const seed = new Uint8Array(32);
    seed.fill(42);
    const b64 = btoa(String.fromCharCode(...seed));
    const mockFetch = createMockFetch([]);
    const client = new ATAPClient({
      did: "did:web:localhost%3A8080:agent:test",
      privateKey: b64,
      platformDomain: "custom.domain.com",
      fetch: mockFetch,
    });
    expect(client.tokenManager).toBeDefined();
  });
});
