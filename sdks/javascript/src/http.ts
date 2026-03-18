/**
 * HTTP client for the ATAP SDK with DPoP authentication.
 */

import { makeDPoPProof } from "./crypto.js";
import {
  ATAPAuthError,
  ATAPConflictError,
  ATAPError,
  ATAPNotFoundError,
  ATAPProblemError,
  ATAPRateLimitError,
} from "./errors.js";
import type { ProblemDetail } from "./models.js";

export interface RequestOptions {
  jsonBody?: Record<string, unknown>;
  headers?: Record<string, string>;
  params?: Record<string, string>;
}

export interface AuthenticatedRequestOptions extends RequestOptions {
  rawBody?: Uint8Array;
  contentType?: string;
}

/** Low-level HTTP client with DPoP proof injection and error handling. */
export class HTTPClient {
  private readonly _baseUrl: string;
  private readonly _timeout: number;
  private readonly _fetch: typeof fetch;

  constructor(
    baseUrl = "http://localhost:8080",
    timeout = 30000,
    fetchImpl?: typeof fetch,
  ) {
    this._baseUrl = baseUrl.replace(/\/+$/, "");
    this._timeout = timeout;
    this._fetch = fetchImpl || globalThis.fetch;
  }

  /** Make an HTTP request and return parsed JSON response. */
  async request(
    method: string,
    path: string,
    opts: RequestOptions = {},
  ): Promise<Record<string, unknown>> {
    const url = this._buildUrl(path, opts.params);
    const headers: Record<string, string> = { ...opts.headers };

    let body: string | undefined;
    if (opts.jsonBody !== undefined) {
      headers["Content-Type"] = headers["Content-Type"] || "application/json";
      body = JSON.stringify(opts.jsonBody);
    }

    const response = await this._fetch(url, {
      method,
      headers,
      body,
      signal: AbortSignal.timeout(this._timeout),
    });

    return this.handleResponse(response);
  }

  /** Make a DPoP-authenticated HTTP request. */
  async authenticatedRequest(
    method: string,
    path: string,
    signingKey: Uint8Array,
    accessToken: string,
    platformDomain: string,
    opts: AuthenticatedRequestOptions = {},
  ): Promise<Record<string, unknown>> {
    const htu = `https://${platformDomain}${path}`;
    const dpopProof = await makeDPoPProof(
      signingKey,
      method,
      htu,
      accessToken,
    );

    const headers: Record<string, string> = {
      Authorization: `DPoP ${accessToken}`,
      DPoP: dpopProof,
      ...opts.headers,
    };

    const url = this._buildUrl(path, opts.params);
    let body: BodyInit | undefined;

    if (opts.rawBody !== undefined) {
      if (opts.contentType) {
        headers["Content-Type"] = opts.contentType;
      }
      body = opts.rawBody as unknown as BodyInit;
    } else if (opts.jsonBody !== undefined) {
      headers["Content-Type"] = headers["Content-Type"] || "application/json";
      body = JSON.stringify(opts.jsonBody);
    }

    const response = await this._fetch(url, {
      method,
      headers,
      body,
      signal: AbortSignal.timeout(this._timeout),
    });

    return this.handleResponse(response);
  }

  /** POST form-encoded data (for OAuth token endpoint). */
  async postForm(
    path: string,
    formData: Record<string, string>,
    dpopProof?: string,
  ): Promise<Record<string, unknown>> {
    const url = this._buildUrl(path);
    const headers: Record<string, string> = {
      "Content-Type": "application/x-www-form-urlencoded",
    };
    if (dpopProof) {
      headers["DPoP"] = dpopProof;
    }

    const body = new URLSearchParams(formData).toString();

    const response = await this._fetch(url, {
      method: "POST",
      headers,
      body,
      signal: AbortSignal.timeout(this._timeout),
    });

    return this.handleResponse(response);
  }

  /** GET request expecting a 302 redirect, returns the Location URL. */
  async getRedirect(
    path: string,
    params?: Record<string, string>,
    dpopProof?: string,
  ): Promise<string> {
    const url = this._buildUrl(path, params);
    const headers: Record<string, string> = {};
    if (dpopProof) {
      headers["DPoP"] = dpopProof;
    }

    const response = await this._fetch(url, {
      method: "GET",
      headers,
      redirect: "manual",
      signal: AbortSignal.timeout(this._timeout),
    });

    if (response.status !== 302) {
      await this.handleResponse(response);
      throw new ATAPError(
        `Expected 302 redirect, got ${response.status}`,
        response.status,
      );
    }

    const location = response.headers.get("location") || "";
    if (!location) {
      throw new ATAPError("302 redirect with no Location header");
    }
    return location;
  }

  /** Parse response, raising typed errors for non-2xx status codes. */
  async handleResponse(
    response: Response,
  ): Promise<Record<string, unknown>> {
    if (response.status === 204) {
      return {};
    }

    let data: Record<string, unknown>;
    try {
      data = (await response.json()) as Record<string, unknown>;
    } catch {
      if (response.status >= 200 && response.status < 300) {
        return {};
      }
      const text = await response.text().catch(() => "");
      throw new ATAPError(
        `HTTP ${response.status}: ${text}`,
        response.status,
      );
    }

    if (response.status >= 200 && response.status < 300) {
      return data;
    }

    // Parse RFC 7807 Problem Detail
    let problem: ProblemDetail | undefined;
    if ("type" in data && "status" in data) {
      problem = {
        type: (data.type as string) || "",
        title: (data.title as string) || "",
        status: (data.status as number) || response.status,
        detail: data.detail as string | undefined,
        instance: data.instance as string | undefined,
      };
    }

    const status = response.status;

    if (status === 401 || status === 403) {
      const msg = problem?.detail || (data.detail as string) || "Authentication failed";
      throw new ATAPAuthError(msg, status, problem);
    } else if (status === 404) {
      const msg = problem?.detail || "Not found";
      throw new ATAPNotFoundError(msg, problem);
    } else if (status === 409) {
      const msg = problem?.detail || "Conflict";
      throw new ATAPConflictError(msg, problem);
    } else if (status === 429) {
      const msg = problem?.detail || "Rate limit exceeded";
      throw new ATAPRateLimitError(msg, problem);
    } else if (problem) {
      throw new ATAPProblemError(problem);
    } else {
      const msg =
        (data.detail as string) ||
        (data.message as string) ||
        JSON.stringify(data);
      throw new ATAPError(`HTTP ${status}: ${msg}`, status);
    }
  }

  private _buildUrl(
    path: string,
    params?: Record<string, string>,
  ): string {
    let url = `${this._baseUrl}${path}`;
    if (params && Object.keys(params).length > 0) {
      url += "?" + new URLSearchParams(params).toString();
    }
    return url;
  }
}
