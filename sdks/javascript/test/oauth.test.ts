import { describe, expect, it } from "vitest";
import { HTTPClient } from "../src/http.js";
import { TokenManager } from "../src/oauth.js";
import { generateKeypair } from "../src/crypto.js";
import { createMockFetch, SAMPLE_TOKEN_RESPONSE } from "./helpers.js";

async function createTokenManager(
  mockFetch: ReturnType<typeof createMockFetch>,
  options: { clientSecret?: string; scopes?: string[] } = {},
) {
  const { privateKey } = await generateKeypair();
  const http = new HTTPClient("http://localhost:8080", 30000, mockFetch);
  return new TokenManager({
    httpClient: http,
    signingKey: privateKey,
    did: "did:web:localhost%3A8080:agent:test",
    clientSecret: options.clientSecret || "atap_testsecret",
    scopes: options.scopes,
    platformDomain: "localhost:8080",
  });
}

describe("TokenManager", () => {
  describe("getAccessToken", () => {
    it("obtains a new token via client_credentials", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const tm = await createTokenManager(mockFetch);

      const token = await tm.getAccessToken();
      expect(token).toBe("test_access_token_123");

      // Verify the form data was sent correctly
      const body = mockFetch.calls[0].init.body as string;
      expect(body).toContain("grant_type=client_credentials");
      expect(body).toContain("client_id=");
      expect(body).toContain("client_secret=atap_testsecret");
    });

    it("returns cached token on subsequent calls", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const tm = await createTokenManager(mockFetch);

      const token1 = await tm.getAccessToken();
      const token2 = await tm.getAccessToken();
      expect(token1).toBe(token2);
      // Should only have made one HTTP call
      expect(mockFetch.calls).toHaveLength(1);
    });

    it("throws when no client_secret for client_credentials", async () => {
      const mockFetch = createMockFetch([]);
      const { privateKey } = await generateKeypair();
      const http = new HTTPClient("http://localhost:8080", 30000, mockFetch);
      const tm = new TokenManager({
        httpClient: http,
        signingKey: privateKey,
        did: "did:web:localhost%3A8080:agent:test",
        platformDomain: "localhost:8080",
      });

      await expect(tm.getAccessToken()).rejects.toThrow(
        "client_secret is required",
      );
    });

    it("refreshes when token has refresh_token and is expired", async () => {
      const expiredToken = {
        ...SAMPLE_TOKEN_RESPONSE,
        expires_in: 0, // immediately expired
      };
      const refreshedToken = {
        ...SAMPLE_TOKEN_RESPONSE,
        access_token: "refreshed_token",
      };

      const mockFetch = createMockFetch([
        { status: 200, body: expiredToken },
        { status: 200, body: refreshedToken },
      ]);
      const tm = await createTokenManager(mockFetch);

      // First call obtains expired token, second call detects expiry and refreshes
      const token1 = await tm.getAccessToken();
      // The token we got is from the expired response (it's valid for the first return)
      expect(token1).toBe("test_access_token_123");

      // Second call should detect expiry and use refresh_token
      const token2 = await tm.getAccessToken();
      expect(token2).toBe("refreshed_token");
      expect(mockFetch.calls).toHaveLength(2);

      // Verify refresh request
      const body = mockFetch.calls[1].init.body as string;
      expect(body).toContain("grant_type=refresh_token");
    });
  });

  describe("invalidate", () => {
    it("clears cached token", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const tm = await createTokenManager(mockFetch);

      await tm.getAccessToken();
      expect(mockFetch.calls).toHaveLength(1);

      tm.invalidate();

      await tm.getAccessToken();
      expect(mockFetch.calls).toHaveLength(2);
    });
  });

  describe("obtainAuthorizationCode", () => {
    it("performs authorization code + PKCE flow", async () => {
      const mockFetch = createMockFetch([
        {
          status: 302,
          body: "",
          headers: {
            location: "atap://callback?code=authcode123",
          },
        },
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const { privateKey } = await generateKeypair();
      const http = new HTTPClient("http://localhost:8080", 30000, mockFetch);
      const tm = new TokenManager({
        httpClient: http,
        signingKey: privateKey,
        did: "did:web:localhost%3A8080:human:test",
        platformDomain: "localhost:8080",
      });

      const token = await tm.obtainAuthorizationCode();
      expect(token.accessToken).toBe("test_access_token_123");
      expect(mockFetch.calls).toHaveLength(2);

      // Verify token exchange
      const body = mockFetch.calls[1].init.body as string;
      expect(body).toContain("grant_type=authorization_code");
      expect(body).toContain("code=authcode123");
      expect(body).toContain("code_verifier=");
    });

    it("throws if no code in redirect", async () => {
      const mockFetch = createMockFetch([
        {
          status: 302,
          body: "",
          headers: {
            location: "atap://callback?error=access_denied",
          },
        },
      ]);
      const { privateKey } = await generateKeypair();
      const http = new HTTPClient("http://localhost:8080", 30000, mockFetch);
      const tm = new TokenManager({
        httpClient: http,
        signingKey: privateKey,
        did: "did:web:localhost%3A8080:human:test",
        platformDomain: "localhost:8080",
      });

      await expect(tm.obtainAuthorizationCode()).rejects.toThrow(
        "No authorization code in redirect",
      );
    });
  });

  describe("default scopes", () => {
    it("uses default scopes when not specified", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const tm = await createTokenManager(mockFetch);

      await tm.getAccessToken();
      const body = mockFetch.calls[0].init.body as string;
      expect(body).toContain(
        "scope=atap%3Ainbox+atap%3Asend+atap%3Arevoke+atap%3Amanage",
      );
    });

    it("uses custom scopes when specified", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: SAMPLE_TOKEN_RESPONSE },
      ]);
      const tm = await createTokenManager(mockFetch, {
        scopes: ["atap:inbox"],
      });

      await tm.getAccessToken();
      const body = mockFetch.calls[0].init.body as string;
      expect(body).toContain("scope=atap%3Ainbox");
    });
  });
});
