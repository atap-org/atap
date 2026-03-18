import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import {
  createMockFetch,
  SAMPLE_REVOCATION,
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

describe("RevocationAPI", () => {
  describe("submit", () => {
    it("submits a revocation", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 201, body: SAMPLE_REVOCATION },
      ]);
      const client = await createAuthedClient(mockFetch);

      const rev = await client.revocations.submit("apr_abc123", "jws_sig");

      expect(rev.id).toBe("rev_abc123");
      expect(rev.approvalId).toBe("apr_abc123");
      expect(rev.approverDid).toContain("did:web:");
      expect(rev.revokedAt).toBeTruthy();
      expect(rev.expiresAt).toBeTruthy();

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.approval_id).toBe("apr_abc123");
      expect(body.signature).toBe("jws_sig");
    });

    it("sends valid_until when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 201, body: SAMPLE_REVOCATION },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.revocations.submit("apr_abc123", "jws_sig", {
        validUntil: "2024-12-31T23:59:59Z",
      });

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.valid_until).toBe("2024-12-31T23:59:59Z");
    });
  });

  describe("list", () => {
    it("lists revocations for an entity", async () => {
      const mockFetch = createMockFetch([
        {
          status: 200,
          body: {
            entity: "did:web:localhost%3A8080:agent:abc",
            revocations: [SAMPLE_REVOCATION],
            checked_at: "2024-01-01T12:30:00Z",
          },
        },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const result = await client.revocations.list(
        "did:web:localhost%3A8080:agent:abc",
      );

      expect(result.entity).toBe("did:web:localhost%3A8080:agent:abc");
      expect(result.revocations).toHaveLength(1);
      expect(result.revocations[0].id).toBe("rev_abc123");
      expect(result.checkedAt).toBe("2024-01-01T12:30:00Z");

      // Verify it's a public endpoint (no auth headers)
      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers?.Authorization).toBeUndefined();
    });

    it("handles empty revocation list", async () => {
      const mockFetch = createMockFetch([
        {
          status: 200,
          body: {
            entity: "did:web:test",
            revocations: [],
            checked_at: "2024-01-01T00:00:00Z",
          },
        },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const result = await client.revocations.list("did:web:test");
      expect(result.revocations).toHaveLength(0);
    });
  });
});
