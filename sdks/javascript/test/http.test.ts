import { describe, expect, it } from "vitest";
import {
  ATAPAuthError,
  ATAPConflictError,
  ATAPError,
  ATAPNotFoundError,
  ATAPProblemError,
  ATAPRateLimitError,
} from "../src/errors.js";
import { HTTPClient } from "../src/http.js";
import { createMockFetch } from "./helpers.js";

describe("HTTPClient", () => {
  describe("request", () => {
    it("makes a GET request and returns JSON", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { id: "123", name: "test" } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const result = await client.request("GET", "/v1/entities/123");

      expect(result).toEqual({ id: "123", name: "test" });
      expect(mockFetch.calls).toHaveLength(1);
      expect(mockFetch.calls[0].url).toBe(
        "http://localhost:8080/v1/entities/123",
      );
      expect(mockFetch.calls[0].init.method).toBe("GET");
    });

    it("sends JSON body for POST", async () => {
      const mockFetch = createMockFetch([
        { status: 201, body: { id: "456" } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await client.request("POST", "/v1/entities", {
        jsonBody: { type: "agent", name: "bot" },
      });

      const init = mockFetch.calls[0].init;
      expect(init.method).toBe("POST");
      expect(init.body).toBe(JSON.stringify({ type: "agent", name: "bot" }));
      const headers = init.headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe("application/json");
    });

    it("appends query params", async () => {
      const mockFetch = createMockFetch([{ status: 200, body: {} }]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await client.request("GET", "/v1/revocations", {
        params: { entity: "did:web:example.com:agent:abc" },
      });

      expect(mockFetch.calls[0].url).toContain("?entity=");
    });

    it("handles 204 No Content", async () => {
      const mockFetch = createMockFetch([{ status: 204 }]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const result = await client.request("DELETE", "/v1/entities/123");
      expect(result).toEqual({});
    });

    it("strips trailing slashes from base URL", async () => {
      const mockFetch = createMockFetch([{ status: 200, body: {} }]);
      const client = new HTTPClient(
        "http://localhost:8080///",
        30000,
        mockFetch,
      );

      await client.request("GET", "/test");
      expect(mockFetch.calls[0].url).toBe("http://localhost:8080/test");
    });
  });

  describe("handleResponse error mapping", () => {
    it("throws ATAPAuthError on 401", async () => {
      const mockFetch = createMockFetch([
        {
          status: 401,
          body: {
            type: "about:blank",
            title: "Unauthorized",
            status: 401,
            detail: "Invalid token",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPAuthError,
      );
    });

    it("throws ATAPAuthError on 403", async () => {
      const mockFetch = createMockFetch([
        {
          status: 403,
          body: {
            type: "about:blank",
            title: "Forbidden",
            status: 403,
            detail: "Insufficient scope",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPAuthError,
      );
    });

    it("throws ATAPNotFoundError on 404", async () => {
      const mockFetch = createMockFetch([
        {
          status: 404,
          body: {
            type: "about:blank",
            title: "Not Found",
            status: 404,
            detail: "Entity not found",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPNotFoundError,
      );
    });

    it("throws ATAPConflictError on 409", async () => {
      const mockFetch = createMockFetch([
        {
          status: 409,
          body: {
            type: "about:blank",
            title: "Conflict",
            status: 409,
            detail: "Already exists",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPConflictError,
      );
    });

    it("throws ATAPRateLimitError on 429", async () => {
      const mockFetch = createMockFetch([
        {
          status: 429,
          body: {
            type: "about:blank",
            title: "Too Many Requests",
            status: 429,
            detail: "Rate limit exceeded",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPRateLimitError,
      );
    });

    it("throws ATAPProblemError for other errors with problem detail", async () => {
      const mockFetch = createMockFetch([
        {
          status: 500,
          body: {
            type: "about:blank",
            title: "Internal Server Error",
            status: 500,
            detail: "Something broke",
          },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPProblemError,
      );
    });

    it("throws ATAPError for non-JSON error responses", async () => {
      const mockFetch = createMockFetch([
        { status: 500, body: "Internal Server Error" },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPError,
      );
    });

    it("throws ATAPError for errors without problem detail", async () => {
      const mockFetch = createMockFetch([
        { status: 502, body: { message: "Bad gateway" } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(client.request("GET", "/v1/test")).rejects.toThrow(
        ATAPError,
      );
    });

    it("returns empty object for 2xx non-JSON", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: "not json" },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const result = await client.request("GET", "/v1/test");
      expect(result).toEqual({});
    });
  });

  describe("postForm", () => {
    it("sends form-encoded data", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { access_token: "tok" } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const result = await client.postForm("/v1/oauth/token", {
        grant_type: "client_credentials",
        client_id: "did:web:test",
      });

      expect(result.access_token).toBe("tok");
      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe("application/x-www-form-urlencoded");
    });

    it("includes DPoP header when provided", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { access_token: "tok" } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await client.postForm(
        "/v1/oauth/token",
        { grant_type: "client_credentials" },
        "dpop_proof_here",
      );

      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers["DPoP"]).toBe("dpop_proof_here");
    });
  });

  describe("getRedirect", () => {
    it("returns Location header on 302", async () => {
      const mockFetch = createMockFetch([
        {
          status: 302,
          body: "",
          headers: { location: "atap://callback?code=abc123" },
        },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const location = await client.getRedirect("/v1/oauth/authorize", {
        response_type: "code",
      });
      expect(location).toBe("atap://callback?code=abc123");
    });

    it("throws if no Location header on 302", async () => {
      const mockFetch = createMockFetch([
        { status: 302, body: "" },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(
        client.getRedirect("/v1/oauth/authorize"),
      ).rejects.toThrow("302 redirect with no Location header");
    });

    it("throws on non-302 response", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: {} },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      await expect(
        client.getRedirect("/v1/oauth/authorize"),
      ).rejects.toThrow("Expected 302 redirect");
    });
  });

  describe("authenticatedRequest", () => {
    it("includes Authorization and DPoP headers", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { ok: true } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      // Generate a valid key for signing
      const { generateKeypair } = await import("../src/crypto.js");
      const { privateKey } = await generateKeypair();

      await client.authenticatedRequest(
        "GET",
        "/v1/approvals",
        privateKey,
        "test_token",
        "localhost:8080",
      );

      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers["Authorization"]).toBe("DPoP test_token");
      expect(headers["DPoP"]).toBeDefined();
    });

    it("sends JSON body in authenticated request", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { ok: true } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const { generateKeypair } = await import("../src/crypto.js");
      const { privateKey } = await generateKeypair();

      await client.authenticatedRequest(
        "POST",
        "/v1/approvals",
        privateKey,
        "test_token",
        "localhost:8080",
        { jsonBody: { from: "did:web:test" } },
      );

      const body = mockFetch.calls[0].init.body as string;
      expect(JSON.parse(body)).toEqual({ from: "did:web:test" });
    });

    it("sends raw body in authenticated request", async () => {
      const mockFetch = createMockFetch([
        { status: 200, body: { ok: true } },
      ]);
      const client = new HTTPClient("http://localhost:8080", 30000, mockFetch);

      const { generateKeypair } = await import("../src/crypto.js");
      const { privateKey } = await generateKeypair();

      const rawBody = new TextEncoder().encode("raw data");
      await client.authenticatedRequest(
        "POST",
        "/v1/didcomm",
        privateKey,
        "test_token",
        "localhost:8080",
        { rawBody, contentType: "application/didcomm-encrypted+json" },
      );

      const headers = mockFetch.calls[0].init.headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe(
        "application/didcomm-encrypted+json",
      );
    });
  });
});
