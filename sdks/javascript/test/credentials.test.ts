import { describe, expect, it } from "vitest";
import { ATAPClient } from "../src/client.js";
import { generateKeypair } from "../src/crypto.js";
import {
  createMockFetch,
  SAMPLE_CREDENTIAL,
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

describe("CredentialAPI", () => {
  describe("startEmailVerification", () => {
    it("initiates email verification", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { message: "OTP sent to user@example.com" } },
      ]);
      const client = await createAuthedClient(mockFetch);

      const msg = await client.credentials.startEmailVerification(
        "user@example.com",
      );
      expect(msg).toBe("OTP sent to user@example.com");

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.email).toBe("user@example.com");
    });

    it("returns default message when none provided", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: {} },
      ]);
      const client = await createAuthedClient(mockFetch);

      const msg = await client.credentials.startEmailVerification(
        "user@example.com",
      );
      expect(msg).toBe("OTP sent");
    });
  });

  describe("verifyEmail", () => {
    it("verifies email with OTP", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: SAMPLE_CREDENTIAL },
      ]);
      const client = await createAuthedClient(mockFetch);

      const cred = await client.credentials.verifyEmail(
        "user@example.com",
        "123456",
      );
      expect(cred.id).toBe("cred_abc123");
      expect(cred.type).toBe("ATAPEmailVerification");
      expect(cred.credential).toContain("eyJ");

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.email).toBe("user@example.com");
      expect(body.otp).toBe("123456");
    });
  });

  describe("startPhoneVerification", () => {
    it("initiates phone verification", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: { message: "OTP sent" } },
      ]);
      const client = await createAuthedClient(mockFetch);

      const msg = await client.credentials.startPhoneVerification(
        "+1234567890",
      );
      expect(msg).toBe("OTP sent");

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.phone).toBe("+1234567890");
    });
  });

  describe("verifyPhone", () => {
    it("verifies phone with OTP", async () => {
      const phoneCred = {
        ...SAMPLE_CREDENTIAL,
        type: "ATAPPhoneVerification",
      };
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: phoneCred },
      ]);
      const client = await createAuthedClient(mockFetch);

      const cred = await client.credentials.verifyPhone(
        "+1234567890",
        "654321",
      );
      expect(cred.type).toBe("ATAPPhoneVerification");
    });
  });

  describe("submitPersonhood", () => {
    it("submits personhood attestation", async () => {
      const personhoodCred = {
        ...SAMPLE_CREDENTIAL,
        type: "ATAPPersonhood",
      };
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: personhoodCred },
      ]);
      const client = await createAuthedClient(mockFetch);

      const cred = await client.credentials.submitPersonhood();
      expect(cred.type).toBe("ATAPPersonhood");
    });

    it("sends provider_token when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: SAMPLE_CREDENTIAL },
      ]);
      const client = await createAuthedClient(mockFetch);

      await client.credentials.submitPersonhood({
        providerToken: "worldid_token",
      });

      const body = JSON.parse(mockFetch.calls[1].init.body as string);
      expect(body.provider_token).toBe("worldid_token");
    });
  });

  describe("list", () => {
    it("lists credentials from array response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: [SAMPLE_CREDENTIAL] },
      ]);
      const client = await createAuthedClient(mockFetch);

      const creds = await client.credentials.list();
      expect(creds).toHaveLength(1);
      expect(creds[0].id).toBe("cred_abc123");
    });

    it("lists credentials from object response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        {
          status: 200,
          body: { credentials: [SAMPLE_CREDENTIAL, SAMPLE_CREDENTIAL] },
        },
      ]);
      const client = await createAuthedClient(mockFetch);

      const creds = await client.credentials.list();
      expect(creds).toHaveLength(2);
    });
  });

  describe("statusList", () => {
    it("gets status list (public endpoint)", async () => {
      const statusData = {
        "@context": ["https://www.w3.org/2018/credentials/v1"],
        type: ["VerifiableCredential", "BitstringStatusListCredential"],
        id: "http://localhost:8080/v1/credentials/status/1",
      };
      const mockFetch = createMockFetch([
        { status: 200, body: statusData },
      ]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      const result = await client.credentials.statusList("1");
      expect(result.type).toEqual([
        "VerifiableCredential",
        "BitstringStatusListCredential",
      ]);
    });

    it("uses default list ID", async () => {
      const mockFetch = createMockFetch([{ status: 200, body: {} }]);
      const client = new ATAPClient({
        baseUrl: "http://localhost:8080",
        fetch: mockFetch,
      });

      await client.credentials.statusList();
      expect(mockFetch.calls[0].url).toContain("/v1/credentials/status/1");
    });
  });
});
