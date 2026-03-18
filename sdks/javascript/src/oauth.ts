/**
 * OAuth 2.1 + DPoP token management for the ATAP SDK.
 */

import { domainFromDID, generatePKCE, makeDPoPProof } from "./crypto.js";
import type { HTTPClient } from "./http.js";
import type { OAuthToken } from "./models.js";

/**
 * Manages OAuth 2.1 tokens with DPoP binding and auto-refresh.
 *
 * Supports client_credentials (agent/machine) and
 * authorization_code+PKCE (human/org) grant types.
 */
export class TokenManager {
  private readonly _http: HTTPClient;
  private readonly _signingKey: Uint8Array;
  private readonly _did: string;
  private readonly _clientSecret?: string;
  private readonly _scopes: string[];
  private readonly _platformDomain: string;
  private _token: OAuthToken | null = null;
  private _tokenObtainedAt = 0;

  constructor(options: {
    httpClient: HTTPClient;
    signingKey: Uint8Array;
    did: string;
    clientSecret?: string;
    scopes?: string[];
    platformDomain?: string;
  }) {
    this._http = options.httpClient;
    this._signingKey = options.signingKey;
    this._did = options.did;
    this._clientSecret = options.clientSecret;
    this._scopes = options.scopes || [
      "atap:inbox",
      "atap:send",
      "atap:revoke",
      "atap:manage",
    ];
    this._platformDomain =
      options.platformDomain || domainFromDID(options.did);
  }

  private get _tokenUrl(): string {
    return `https://${this._platformDomain}/v1/oauth/token`;
  }

  /** Get a valid access token, refreshing if needed. */
  async getAccessToken(): Promise<string> {
    if (this._token && !this._isExpired()) {
      return this._token.accessToken;
    }
    if (this._token?.refreshToken) {
      const refreshed = await this._refresh();
      return refreshed.accessToken;
    }
    const obtained = await this._obtain();
    return obtained.accessToken;
  }

  private _isExpired(): boolean {
    if (!this._token) return true;
    const elapsed = (Date.now() - this._tokenObtainedAt) / 1000;
    // Refresh 60 seconds before expiry
    return elapsed >= this._token.expiresIn - 60;
  }

  /** Obtain a new token via client_credentials grant. */
  private async _obtain(): Promise<OAuthToken> {
    if (!this._clientSecret) {
      throw new Error(
        "client_secret is required for client_credentials grant. " +
          "For human/org entities, use obtainAuthorizationCode() instead.",
      );
    }

    const dpopProof = await makeDPoPProof(
      this._signingKey,
      "POST",
      this._tokenUrl,
    );

    const formData: Record<string, string> = {
      grant_type: "client_credentials",
      client_id: this._did,
      client_secret: this._clientSecret,
      scope: this._scopes.join(" "),
    };

    const data = await this._http.postForm(
      "/v1/oauth/token",
      formData,
      dpopProof,
    );

    this._token = {
      accessToken: data.access_token as string,
      tokenType: (data.token_type as string) || "DPoP",
      expiresIn: (data.expires_in as number) ?? 3600,
      scope: (data.scope as string) || "",
      refreshToken: data.refresh_token as string | undefined,
    };
    this._tokenObtainedAt = Date.now();
    return this._token;
  }

  /** Refresh an expired token using the refresh token. */
  private async _refresh(): Promise<OAuthToken> {
    if (!this._token?.refreshToken) {
      return this._obtain();
    }

    const dpopProof = await makeDPoPProof(
      this._signingKey,
      "POST",
      this._tokenUrl,
    );

    const formData: Record<string, string> = {
      grant_type: "refresh_token",
      refresh_token: this._token.refreshToken,
    };

    const data = await this._http.postForm(
      "/v1/oauth/token",
      formData,
      dpopProof,
    );

    this._token = {
      accessToken: data.access_token as string,
      tokenType: (data.token_type as string) || "DPoP",
      expiresIn: (data.expires_in as number) ?? 3600,
      scope: (data.scope as string) || "",
      refreshToken:
        (data.refresh_token as string) || this._token.refreshToken,
    };
    this._tokenObtainedAt = Date.now();
    return this._token;
  }

  /** Obtain a token via authorization_code + PKCE flow (for human/org). */
  async obtainAuthorizationCode(
    redirectUri = "atap://callback",
  ): Promise<OAuthToken> {
    const { verifier, challenge } = generatePKCE();
    const authorizeUrl = `https://${this._platformDomain}/v1/oauth/authorize`;

    const dpopProof = await makeDPoPProof(
      this._signingKey,
      "GET",
      authorizeUrl,
    );

    const params: Record<string, string> = {
      response_type: "code",
      client_id: this._did,
      redirect_uri: redirectUri,
      scope: this._scopes.join(" "),
      code_challenge: challenge,
      code_challenge_method: "S256",
    };

    const redirectLocation = await this._http.getRedirect(
      "/v1/oauth/authorize",
      params,
      dpopProof,
    );

    // Extract code from redirect URL
    const parsed = new URL(redirectLocation);
    const code = parsed.searchParams.get("code");
    if (!code) {
      throw new Error(
        `No authorization code in redirect: ${redirectLocation}`,
      );
    }

    // Exchange code for token
    const dpopProof2 = await makeDPoPProof(
      this._signingKey,
      "POST",
      this._tokenUrl,
    );

    const formData: Record<string, string> = {
      grant_type: "authorization_code",
      code,
      redirect_uri: redirectUri,
      code_verifier: verifier,
    };

    const data = await this._http.postForm(
      "/v1/oauth/token",
      formData,
      dpopProof2,
    );

    this._token = {
      accessToken: data.access_token as string,
      tokenType: (data.token_type as string) || "DPoP",
      expiresIn: (data.expires_in as number) ?? 3600,
      scope: (data.scope as string) || "",
      refreshToken: data.refresh_token as string | undefined,
    };
    this._tokenObtainedAt = Date.now();
    return this._token;
  }

  /** Clear cached token, forcing re-authentication on next request. */
  invalidate(): void {
    this._token = null;
    this._tokenObtainedAt = 0;
  }
}
