import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import {
  createMockFetch,
  SAMPLE_DID_DOCUMENT,
  SAMPLE_DISCOVERY,
} from "./helpers.js";

describe("DiscoveryAPI", () => {
  describe("discover", () => {
    it("fetches discovery document", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_DISCOVERY },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const doc = await client.discovery.discover();

      expect(doc.domain).toBe("localhost:8080");
      expect(doc.apiBase).toBe("http://localhost:8080/v1");
      expect(doc.didcommEndpoint).toBe("http://localhost:8080/v1/didcomm");
      expect(doc.claimTypes).toEqual(["email", "phone", "personhood"]);
      expect(doc.maxApprovalTtl).toBe("P30D");
      expect(doc.trustLevel).toBe(3);
      expect(doc.oauth).toBeDefined();

      expect(mockFetch.calls[0].url).toContain("/.well-known/atap.json");
    });

    it("handles missing optional fields", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { domain: "test.com" } },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const doc = await client.discovery.discover();
      expect(doc.domain).toBe("test.com");
      expect(doc.claimTypes).toEqual([]);
      expect(doc.oauth).toBeUndefined();
    });
  });

  describe("resolveDid", () => {
    it("resolves a DID document", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_DID_DOCUMENT },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const didDoc = await client.discovery.resolveDid("agent", "01HQXYZ");

      expect(didDoc.id).toBe("did:web:localhost%3A8080:agent:01HQXYZ");
      expect(didDoc.context).toHaveLength(2);
      expect(didDoc.verificationMethod).toHaveLength(1);
      expect(didDoc.verificationMethod[0].type).toBe(
        "Ed25519VerificationKey2020",
      );
      expect(didDoc.verificationMethod[0].publicKeyMultibase).toBe(
        "z6Mktest123",
      );
      expect(didDoc.authentication).toHaveLength(1);
      expect(didDoc.atapType).toBe("agent");

      expect(mockFetch.calls[0].url).toContain("/agent/01HQXYZ/did.json");
    });
  });

  describe("serverDid", () => {
    it("fetches server DID document", async () => {
      const serverDid = {
        ...SAMPLE_DID_DOCUMENT,
        id: "did:web:localhost%3A8080:server:platform",
        "atap:type": "server",
      };
      const mockFetch = createMockFetch([
        { status: 200, body: serverDid },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const didDoc = await client.discovery.serverDid();

      expect(didDoc.id).toBe("did:web:localhost%3A8080:server:platform");
      expect(didDoc.atapType).toBe("server");
      expect(mockFetch.calls[0].url).toContain(
        "/server/platform/did.json",
      );
    });
  });

  describe("health", () => {
    it("checks server health", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { status: "ok", version: "0.1.0" } },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const health = await client.discovery.health();

      expect(health.status).toBe("ok");
      expect(health.version).toBe("0.1.0");
      expect(mockFetch.calls[0].url).toContain("/v1/health");
    });
  });
});
