import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import {
  createMockFetch,
  SAMPLE_ENTITY,
  SAMPLE_TOKEN_RESPONSE,
} from "./helpers.js";

describe("EntityAPI", () => {
  describe("register", () => {
    it("registers a new entity", async () => {
      const mockFetch = createMockFetch([
        { status: 201, body: SAMPLE_ENTITY },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const entity = await client.entities.register("agent", {
        name: "test-agent",
      });

      expect(entity.id).toBe("01HQXYZ123456789ABCDEF");
      expect(entity.type).toBe("agent");
      expect(entity.name).toBe("test-agent");
      expect(entity.did).toContain("did:web:");
      expect(entity.clientSecret).toBe("atap_dGVzdHNlY3JldA==");

      const body = JSON.parse(mockFetch.calls[0].init.body as string);
      expect(body.type).toBe("agent");
      expect(body.name).toBe("test-agent");
    });

    it("sends public_key when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 201, body: SAMPLE_ENTITY },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      await client.entities.register("human", {
        publicKey: "base64pubkey",
      });

      const body = JSON.parse(mockFetch.calls[0].init.body as string);
      expect(body.public_key).toBe("base64pubkey");
    });

    it("sends principal_did when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 201, body: SAMPLE_ENTITY },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      await client.entities.register("agent", {
        principalDid: "did:web:example.com:human:abc",
      });

      const body = JSON.parse(mockFetch.calls[0].init.body as string);
      expect(body.principal_did).toBe("did:web:example.com:human:abc");
    });
  });

  describe("get", () => {
    it("gets entity by ID", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_ENTITY },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const entity = await client.entities.get("01HQXYZ123456789ABCDEF");

      expect(entity.id).toBe("01HQXYZ123456789ABCDEF");
      expect(mockFetch.calls[0].url).toContain(
        "/v1/entities/01HQXYZ123456789ABCDEF",
      );
    });
  });

  describe("delete", () => {
    it("deletes an entity with authentication", async () => {
      const { privateKey } = await generateKeypair();
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 204 },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        did: "did:web:localhost%3A8080:agent:test",
        signingKey: privateKey,
        clientSecret: "atap_secret",
        fetch: mockFetch,
      });

      await client.entities.delete("01HQXYZ123456789ABCDEF");

      // First call is token, second is the delete
      expect(mockFetch.calls).toHaveLength(2);
      expect(mockFetch.calls[1].init.method).toBe("DELETE");
    });
  });

  describe("rotateKey", () => {
    it("rotates entity key", async () => {
      const { privateKey } = await generateKeypair();
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        {
          status: 200,
          body: {
            id: "key_new_abc",
            entity_id: "01HQXYZ",
            key_index: 2,
            valid_from: "2024-01-02T00:00:00Z",
            created_at: "2024-01-02T00:00:00Z",
          },
        },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        did: "did:web:localhost%3A8080:agent:test",
        signingKey: privateKey,
        clientSecret: "atap_secret",
        fetch: mockFetch,
      });

      const kv = await client.entities.rotateKey("01HQXYZ", "newpubkey");

      expect(kv.id).toBe("key_new_abc");
      expect(kv.keyIndex).toBe(2);
      expect(kv.entityId).toBe("01HQXYZ");
    });
  });
});
