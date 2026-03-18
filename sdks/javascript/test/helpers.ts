/**
 * Test helpers -- mock fetch implementation and utilities.
 */

import { vi } from "vitest";

export interface MockResponse {
  status: number;
  body?: Record<string, unknown> | string;
  headers?: Record<string, string>;
}

/**
 * Create a mock fetch function that returns predefined responses.
 * Responses are consumed in order (queue-based).
 */
export function createMockFetch(
  responses: MockResponse[] = [],
): typeof fetch & {
  calls: Array<{ url: string; init: RequestInit }>;
  pushResponse: (r: MockResponse) => void;
} {
  const queue = [...responses];
  const calls: Array<{ url: string; init: RequestInit }> = [];

  const mockFetch = vi.fn(
    async (input: string | URL | Request, init?: RequestInit): Promise<Response> => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      calls.push({ url, init: init || {} });

      const mockResponse = queue.shift();
      if (!mockResponse) {
        throw new Error(`No mock response queued for ${init?.method || "GET"} ${url}`);
      }

      const headers = new Headers(mockResponse.headers || {});
      let bodyStr: string | null = null;
      if (mockResponse.body !== undefined) {
        if (typeof mockResponse.body === "string") {
          bodyStr = mockResponse.body;
        } else {
          bodyStr = JSON.stringify(mockResponse.body);
          headers.set("Content-Type", "application/json");
        }
      }

      return new Response(bodyStr, {
        status: mockResponse.status,
        headers,
      });
    },
  ) as unknown as typeof fetch & {
    calls: Array<{ url: string; init: RequestInit }>;
    pushResponse: (r: MockResponse) => void;
  };

  mockFetch.calls = calls;
  mockFetch.pushResponse = (r: MockResponse) => queue.push(r);

  return mockFetch;
}

/** Sample entity response data. */
export const SAMPLE_ENTITY = {
  id: "01HQXYZ123456789ABCDEF",
  type: "agent",
  did: "did:web:localhost%3A8080:agent:01HQXYZ123456789ABCDEF",
  principal_did: "",
  name: "test-agent",
  key_id: "key_abc_123",
  public_key: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  trust_level: 1,
  registry: "localhost:8080",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
  client_secret: "atap_dGVzdHNlY3JldA==",
};

/** Sample approval response data. */
export const SAMPLE_APPROVAL = {
  id: "apr_abc123",
  state: "pending",
  created_at: "2024-01-01T00:00:00Z",
  valid_until: "2024-01-02T00:00:00Z",
  from: "did:web:localhost%3A8080:agent:requester",
  to: "did:web:localhost%3A8080:human:approver",
  via: "",
  parent: "",
  subject: {
    type: "com.example.payment",
    label: "Payment of $100",
    reversible: false,
    payload: { amount: 100 },
  },
  template_url: "",
  signatures: {},
  responded_at: null,
  fan_out: null,
};

/** Sample revocation response data. */
export const SAMPLE_REVOCATION = {
  id: "rev_abc123",
  approval_id: "apr_abc123",
  approver_did: "did:web:localhost%3A8080:human:approver",
  revoked_at: "2024-01-01T12:00:00Z",
  expires_at: "2024-01-01T13:00:00Z",
};

/** Sample DIDComm message. */
export const SAMPLE_DIDCOMM_MESSAGE = {
  id: "msg_abc123",
  sender_did: "did:web:localhost%3A8080:agent:sender",
  message_type: "https://atap.dev/protocol/signal",
  payload: '{"hello":"world"}',
  created_at: "2024-01-01T00:00:00Z",
};

/** Sample credential. */
export const SAMPLE_CREDENTIAL = {
  id: "cred_abc123",
  type: "ATAPEmailVerification",
  credential: "eyJ0eXAiOiJKV1QiLCJhbGciOiJFZERTQSJ9.test.sig",
  issued_at: "2024-01-01T00:00:00Z",
  revoked_at: null,
};

/** Sample OAuth token response. */
export const SAMPLE_TOKEN_RESPONSE = {
  access_token: "test_access_token_123",
  token_type: "DPoP",
  expires_in: 3600,
  scope: "atap:inbox atap:send atap:revoke atap:manage",
  refresh_token: "test_refresh_token_456",
};

/** Sample discovery document. */
export const SAMPLE_DISCOVERY = {
  domain: "localhost:8080",
  api_base: "http://localhost:8080/v1",
  didcomm_endpoint: "http://localhost:8080/v1/didcomm",
  claim_types: ["email", "phone", "personhood"],
  max_approval_ttl: "P30D",
  trust_level: 3,
  oauth: {
    token_endpoint: "http://localhost:8080/v1/oauth/token",
  },
};

/** Sample DID document. */
export const SAMPLE_DID_DOCUMENT = {
  "@context": [
    "https://www.w3.org/ns/did/v1",
    "https://w3id.org/security/suites/ed25519-2020/v1",
  ],
  id: "did:web:localhost%3A8080:agent:01HQXYZ",
  verificationMethod: [
    {
      id: "did:web:localhost%3A8080:agent:01HQXYZ#key-1",
      type: "Ed25519VerificationKey2020",
      controller: "did:web:localhost%3A8080:agent:01HQXYZ",
      publicKeyMultibase: "z6Mktest123",
    },
  ],
  authentication: ["did:web:localhost%3A8080:agent:01HQXYZ#key-1"],
  assertionMethod: ["did:web:localhost%3A8080:agent:01HQXYZ#key-1"],
  keyAgreement: [],
  service: [],
  "atap:type": "agent",
  "atap:principal": "",
};
