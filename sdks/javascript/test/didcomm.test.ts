import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import {
  createMockFetch,
  SAMPLE_DIDCOMM_MESSAGE,
  SAMPLE_TOKEN_RESPONSE,
} from "./helpers.js";

async function createAuthedClient(mockFetch: ReturnType<typeof createMockFetch>) {
  const { privateKey } = await generateKeypair();
  return new ATAPClient({
    baseUrl: "http://localhost:8080",
    did: "did:web:localhost%3A8080:agent:test",
    signingKey: privateKey,
    clientSecret: "atap_secret",
    fetch: mockFetch,
  });
}

describe("DIDCommAPI", () => {
  describe("send", () => {
    it("sends a DIDComm message", async () => {
      const mockFetch = createMockFetch([
        { status: 202, body: { id: "msg_new", status: "queued" } },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const jweBytes = new TextEncoder().encode(
        JSON.stringify({ protected: "header", ciphertext: "data" }),
      );
      const result = await client.didcomm.send(jweBytes);

      expect(result.id).toBe("msg_new");
      expect(result.status).toBe("queued");

      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe(
        "application/didcomm-encrypted+json",
      );
    });
  });

  describe("inbox", () => {
    it("retrieves inbox messages", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        {
          status: 200,
          body: {
            messages: [SAMPLE_DIDCOMM_MESSAGE, SAMPLE_DIDCOMM_MESSAGE],
            count: 2,
          },
        },
      ]);
      const client = await createAuthedClient(mockFetch);

      const inbox = await client.didcomm.inbox();

      expect(inbox.count).toBe(2);
      expect(inbox.messages).toHaveLength(2);
      expect(inbox.messages[0].id).toBe("msg_abc123");
      expect(inbox.messages[0].senderDid).toContain("did:web:");
      expect(inbox.messages[0].messageType).toBe(
        "https://atap.dev/protocol/signal",
      );
      expect(inbox.messages[0].payload).toBe('{"hello":"world"}');
    });

    it("respects limit parameter", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { messages: [], count: 0 } },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.didcomm.inbox({ limit: 10 });

      expect(mockFetch.calls[1].url).toContain("limit=10");
    });

    it("caps limit at 100", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { messages: [], count: 0 } },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.didcomm.inbox({ limit: 200 });

      expect(mockFetch.calls[1].url).toContain("limit=100");
    });

    it("defaults limit to 50", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { messages: [], count: 0 } },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.didcomm.inbox();

      expect(mockFetch.calls[1].url).toContain("limit=50");
    });

    it("handles empty inbox", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { messages: [], count: 0 } },
      ]);
      const client = await createAuthedClient(mockFetch);

      const inbox = await client.didcomm.inbox();
      expect(inbox.messages).toHaveLength(0);
      expect(inbox.count).toBe(0);
    });
  });
});
