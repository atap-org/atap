import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import {
  createMockFetch,
  SAMPLE_APPROVAL,
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

describe("ApprovalAPI", () => {
  describe("create", () => {
    it("creates an approval request", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 201, body: SAMPLE_APPROVAL },
      ]);
      const client = await createAuthedClient(mockFetch);

      const approval = await client.approvals.create(
        "did:web:localhost%3A8080:agent:requester",
        "did:web:localhost%3A8080:human:approver",
        {
          type: "com.example.payment",
          label: "Payment of $100",
          payload: { amount: 100 },
        },
      );

      expect(approval.id).toBe("apr_abc123");
      expect(approval.state).toBe("pending");
      expect(approval.fromDid).toBe(
        "did:web:localhost%3A8080:agent:requester",
      );
      expect(approval.toDid).toBe(
        "did:web:localhost%3A8080:human:approver",
      );
      expect(approval.subject?.type).toBe("com.example.payment");
      expect(approval.subject?.label).toBe("Payment of $100");
      expect(approval.subject?.payload).toEqual({ amount: 100 });
    });

    it("sends via field when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 201, body: SAMPLE_APPROVAL },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.approvals.create(
        "did:web:test:agent:a",
        "did:web:test:human:b",
        { type: "test", label: "test" },
        { via: "did:web:test:machine:mediator" },
      );

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.via).toBe("did:web:test:machine:mediator");
    });
  });

  describe("respond", () => {
    it("responds to an approval", async () => {
      const responded = {
        ...SAMPLE_APPROVAL,
        state: "approved",
        responded_at: "2024-01-01T01:00:00Z",
        signatures: { approver: "jws_signature" },
      };
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: responded },
      ]);
      const client = await createAuthedClient(mockFetch);

      const result = await client.approvals.respond(
        "apr_abc123",
        "jws_signature",
      );

      expect(result.state).toBe("approved");
      expect(result.respondedAt).toBe("2024-01-01T01:00:00Z");
      expect(result.signatures).toEqual({ approver: "jws_signature" });
    });
  });

  describe("list", () => {
    it("lists approvals from array response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: [SAMPLE_APPROVAL, SAMPLE_APPROVAL] },
      ]);
      const client = await createAuthedClient(mockFetch);

      const approvals = await client.approvals.list();
      expect(approvals).toHaveLength(2);
      expect(approvals[0].id).toBe("apr_abc123");
    });

    it("lists approvals from object response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        {
          status: 200,
          body: { approvals: [SAMPLE_APPROVAL] },
        },
      ]);
      const client = await createAuthedClient(mockFetch);

      const approvals = await client.approvals.list();
      expect(approvals).toHaveLength(1);
    });

    it("handles items key in response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        {
          status: 200,
          body: { items: [SAMPLE_APPROVAL] },
        },
      ]);
      const client = await createAuthedClient(mockFetch);

      const approvals = await client.approvals.list();
      expect(approvals).toHaveLength(1);
    });
  });

  describe("revoke", () => {
    it("revokes an approval", async () => {
      const revoked = { ...SAMPLE_APPROVAL, state: "revoked" };
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: revoked },
      ]);
      const client = await createAuthedClient(mockFetch);

      const result = await client.approvals.revoke("apr_abc123");
      expect(result.state).toBe("revoked");
      expect(mockFetch.calls[1].init.method).toBe("DELETE");
    });
  });
});
