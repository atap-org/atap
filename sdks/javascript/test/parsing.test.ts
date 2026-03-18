/**
 * Tests focused on branch coverage for parsing functions.
 */

import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import { createMockFetch, SAMPLE_TOKEN_RESPONSE } from "./helpers.js";

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

describe("Entity parsing", () => {
  it("handles minimal entity response (missing optional fields)", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: { id: "abc", type: "agent" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const entity = await client.entities.get("abc");
    expect(entity.id).toBe("abc");
    expect(entity.did).toBe("");
    expect(entity.principalDid).toBe("");
    expect(entity.name).toBe("");
    expect(entity.keyId).toBe("");
    expect(entity.publicKey).toBe("");
    expect(entity.trustLevel).toBe(0);
    expect(entity.registry).toBe("");
    expect(entity.createdAt).toBe("");
    expect(entity.updatedAt).toBe("");
    expect(entity.clientSecret).toBeUndefined();
    expect(entity.privateKey).toBeUndefined();
  });

  it("handles entity with private_key field", async () => {
    const mockFetch = createMockFetch([
      {
        status: 201,
        body: {
          id: "abc",
          type: "agent",
          private_key: "base64key",
          client_secret: "atap_secret",
        },
      },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const entity = await client.entities.register("agent");
    expect(entity.privateKey).toBe("base64key");
    expect(entity.clientSecret).toBe("atap_secret");
  });
});

describe("KeyVersion parsing", () => {
  it("handles missing optional fields", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: { id: "key_1" } },
    ]);
    const client = await createAuthedClient(mockFetch);

    const kv = await client.entities.rotateKey("entity1", "pubkey");
    expect(kv.id).toBe("key_1");
    expect(kv.entityId).toBe("");
    expect(kv.keyIndex).toBe(0);
    expect(kv.validFrom).toBe("");
    expect(kv.validUntil).toBeUndefined();
    expect(kv.createdAt).toBe("");
  });

  it("handles valid_until present", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: {
          id: "key_1",
          valid_until: "2025-01-01T00:00:00Z",
        },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const kv = await client.entities.rotateKey("entity1", "pubkey");
    expect(kv.validUntil).toBe("2025-01-01T00:00:00Z");
  });
});

describe("Approval parsing", () => {
  it("handles approval without subject", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: { id: "apr_1", state: "pending" } },
    ]);
    const client = await createAuthedClient(mockFetch);

    const result = await client.approvals.respond("apr_1", "sig");
    expect(result.id).toBe("apr_1");
    expect(result.subject).toBeUndefined();
    expect(result.fromDid).toBe("");
    expect(result.toDid).toBe("");
    expect(result.via).toBe("");
    expect(result.parent).toBe("");
    expect(result.templateUrl).toBe("");
    expect(result.validUntil).toBeUndefined();
    expect(result.respondedAt).toBeUndefined();
    expect(result.fanOut).toBeUndefined();
  });

  it("handles subject with reversible true", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: {
          id: "apr_1",
          subject: {
            type: "test",
            label: "Test",
            reversible: true,
            payload: { key: "value" },
          },
        },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const result = await client.approvals.respond("apr_1", "sig");
    expect(result.subject?.reversible).toBe(true);
    expect(result.subject?.payload).toEqual({ key: "value" });
  });

  it("handles subject without payload", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: {
          id: "apr_1",
          subject: { type: "test", label: "Test" },
        },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const result = await client.approvals.respond("apr_1", "sig");
    expect(result.subject?.payload).toBeUndefined();
    expect(result.subject?.reversible).toBe(false);
  });

  it("handles approval with all optional fields present", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: {
          id: "apr_1",
          state: "approved",
          valid_until: "2025-01-01T00:00:00Z",
          responded_at: "2024-06-01T00:00:00Z",
          fan_out: 3,
          from: "did:web:a",
          to: "did:web:b",
          via: "did:web:c",
          parent: "apr_0",
          template_url: "https://template.example.com",
          signatures: { a: "sig1", b: "sig2" },
          created_at: "2024-01-01T00:00:00Z",
        },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const result = await client.approvals.respond("apr_1", "sig");
    expect(result.validUntil).toBe("2025-01-01T00:00:00Z");
    expect(result.respondedAt).toBe("2024-06-01T00:00:00Z");
    expect(result.fanOut).toBe(3);
    expect(result.via).toBe("did:web:c");
    expect(result.parent).toBe("apr_0");
    expect(result.templateUrl).toBe("https://template.example.com");
    expect(result.signatures).toEqual({ a: "sig1", b: "sig2" });
  });

  it("handles empty list response", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: {} },
    ]);
    const client = await createAuthedClient(mockFetch);

    const result = await client.approvals.list();
    expect(result).toEqual([]);
  });
});

describe("Revocation parsing", () => {
  it("handles minimal revocation", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 201, body: { id: "rev_1" } },
    ]);
    const client = await createAuthedClient(mockFetch);

    const rev = await client.revocations.submit("apr_1", "sig");
    expect(rev.id).toBe("rev_1");
    expect(rev.approvalId).toBe("");
    expect(rev.approverDid).toBe("");
    expect(rev.revokedAt).toBe("");
    expect(rev.expiresAt).toBe("");
  });

  it("handles missing revocations array in list", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: { entity: "did:web:test" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const result = await client.revocations.list("did:web:test");
    expect(result.revocations).toEqual([]);
    expect(result.checkedAt).toBe("");
  });
});

describe("DIDComm parsing", () => {
  it("handles minimal message fields", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: { messages: [{ id: "msg_1" }], count: 1 },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const inbox = await client.didcomm.inbox();
    expect(inbox.messages[0].senderDid).toBe("");
    expect(inbox.messages[0].messageType).toBe("");
    expect(inbox.messages[0].payload).toBe("");
    expect(inbox.messages[0].createdAt).toBe("");
  });

  it("handles missing count (defaults to messages length)", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: { messages: [{ id: "m1" }, { id: "m2" }] },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const inbox = await client.didcomm.inbox();
    expect(inbox.count).toBe(2);
  });

  it("handles missing messages array", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: {} },
    ]);
    const client = await createAuthedClient(mockFetch);

    const inbox = await client.didcomm.inbox();
    expect(inbox.messages).toEqual([]);
    expect(inbox.count).toBe(0);
  });
});

describe("Credential parsing", () => {
  it("handles minimal credential fields", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: {} },
    ]);
    const client = await createAuthedClient(mockFetch);

    const cred = await client.credentials.verifyEmail("a@b.c", "123");
    expect(cred.id).toBe("");
    expect(cred.type).toBe("");
    expect(cred.credential).toBe("");
    expect(cred.issuedAt).toBe("");
    expect(cred.revokedAt).toBeUndefined();
  });

  it("handles credential with revoked_at", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      {
        status: 200,
        body: {
          id: "cred_1",
          type: "ATAPEmailVerification",
          credential: "jwt",
          issued_at: "2024-01-01",
          revoked_at: "2024-06-01",
        },
      },
    ]);
    const client = await createAuthedClient(mockFetch);

    const cred = await client.credentials.verifyEmail("a@b.c", "123");
    expect(cred.revokedAt).toBe("2024-06-01");
  });

  it("handles empty credentials list", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      { status: 200, body: { credentials: [] } },
    ]);
    const client = await createAuthedClient(mockFetch);

    const creds = await client.credentials.list();
    expect(creds).toEqual([]);
  });
});

describe("Discovery parsing", () => {
  it("handles discovery with all missing optional fields", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: {} },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const doc = await client.discovery.discover();
    expect(doc.domain).toBe("");
    expect(doc.apiBase).toBe("");
    expect(doc.didcommEndpoint).toBe("");
    expect(doc.claimTypes).toEqual([]);
    expect(doc.maxApprovalTtl).toBe("");
    expect(doc.trustLevel).toBe(0);
    expect(doc.oauth).toBeUndefined();
  });

  it("handles DID document with all fields", async () => {
    const mockFetch = createMockFetch([
      {
        status: 200,
        body: {
          id: "did:web:test",
          "@context": ["https://www.w3.org/ns/did/v1"],
          verificationMethod: [
            {
              id: "did:web:test#key-1",
              type: "Ed25519VerificationKey2020",
              controller: "did:web:test",
              publicKeyMultibase: "z6Mk",
            },
          ],
          authentication: ["did:web:test#key-1"],
          assertionMethod: ["did:web:test#key-1"],
          keyAgreement: ["did:web:test#key-2"],
          service: [{ id: "#svc", type: "DIDComm", endpoint: "https://e.com" }],
          "atap:type": "agent",
          "atap:principal": "did:web:principal",
        },
      },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const doc = await client.discovery.resolveDid("agent", "test");
    expect(doc.id).toBe("did:web:test");
    expect(doc.context).toEqual(["https://www.w3.org/ns/did/v1"]);
    expect(doc.verificationMethod).toHaveLength(1);
    expect(doc.verificationMethod[0].controller).toBe("did:web:test");
    expect(doc.authentication).toHaveLength(1);
    expect(doc.assertionMethod).toHaveLength(1);
    expect(doc.keyAgreement).toHaveLength(1);
    expect(doc.service).toHaveLength(1);
    expect(doc.atapType).toBe("agent");
    expect(doc.atapPrincipal).toBe("did:web:principal");
  });

  it("handles DID document with empty fields", async () => {
    const mockFetch = createMockFetch([
      { status: 200, body: { id: "did:web:test" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const doc = await client.discovery.resolveDid("agent", "test");
    expect(doc.context).toEqual([]);
    expect(doc.verificationMethod).toEqual([]);
    expect(doc.authentication).toEqual([]);
    expect(doc.assertionMethod).toEqual([]);
    expect(doc.keyAgreement).toEqual([]);
    expect(doc.service).toEqual([]);
    expect(doc.atapType).toBe("");
    expect(doc.atapPrincipal).toBe("");
  });

  it("handles verification method with missing fields", async () => {
    const mockFetch = createMockFetch([
      {
        status: 200,
        body: {
          id: "did:web:test",
          verificationMethod: [{ id: "key-1" }],
        },
      },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    const doc = await client.discovery.resolveDid("agent", "test");
    expect(doc.verificationMethod[0].type).toBe("");
    expect(doc.verificationMethod[0].controller).toBe("");
    expect(doc.verificationMethod[0].publicKeyMultibase).toBe("");
  });
});

describe("HTTP error branches", () => {
  it("handles 401 without problem detail", async () => {
    const mockFetch = createMockFetch([
      { status: 401, body: { detail: "Bad token" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toBe("Bad token");
    }
  });

  it("handles 401 fallback message", async () => {
    const mockFetch = createMockFetch([
      { status: 401, body: { foo: "bar" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toBe("Authentication failed");
    }
  });

  it("handles 404 fallback message", async () => {
    const mockFetch = createMockFetch([
      { status: 404, body: { foo: "bar" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toBe("Not found");
    }
  });

  it("handles 409 fallback message", async () => {
    const mockFetch = createMockFetch([
      { status: 409, body: { foo: "bar" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toBe("Conflict");
    }
  });

  it("handles 429 fallback message", async () => {
    const mockFetch = createMockFetch([
      { status: 429, body: { foo: "bar" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toBe("Rate limit exceeded");
    }
  });

  it("handles error with detail field (no problem)", async () => {
    const mockFetch = createMockFetch([
      { status: 502, body: { detail: "bad gateway" } },
    ]);
    const client = new ATAPClient({
      baseUrl: "http://localhost:8080",
      fetch: mockFetch,
    });

    try {
      await client.entities.get("test");
    } catch (e: unknown) {
      expect((e as Error).message).toContain("bad gateway");
    }
  });
});
