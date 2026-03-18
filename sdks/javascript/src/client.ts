/**
 * Main ATAP client that ties all API modules together.
 */

import { ApprovalAPI } from "./approvals.js";
import { CredentialAPI } from "./credentials.js";
import { domainFromDID, loadSigningKey } from "./crypto.js";
import { DIDCommAPI } from "./didcomm.js";
import { DiscoveryAPI } from "./discovery.js";
import { EntityAPI } from "./entities.js";
import { HTTPClient } from "./http.js";
import type { AuthenticatedRequestOptions } from "./http.js";
import { TokenManager } from "./oauth.js";
import { RevocationAPI } from "./revocations.js";

/** Options for constructing an ATAPClient. */
export interface ATAPClientOptions {
  /** HTTP base URL for the ATAP server. */
  baseUrl?: string;
  /** Entity DID (used as client_id for OAuth). */
  did?: string;
  /** Base64-encoded Ed25519 private key (seed or full key). */
  privateKey?: string;
  /** Pre-loaded Ed25519 signing key (32-byte seed). Alternative to privateKey. */
  signingKey?: Uint8Array;
  /** Client secret for agent/machine client_credentials grant. */
  clientSecret?: string;
  /** OAuth scopes (defaults to all). */
  scopes?: string[];
  /** Domain for DPoP htu construction. Defaults to domain extracted from DID. */
  platformDomain?: string;
  /** HTTP request timeout in milliseconds. */
  timeout?: number;
  /** Custom fetch implementation (for testing). */
  fetch?: typeof fetch;
}

/**
 * High-level client for the ATAP platform.
 *
 * Usage for agent/machine (client_credentials):
 * ```typescript
 * const client = new ATAPClient({
 *   baseUrl: "http://localhost:8080",
 *   did: "did:web:localhost%3A8080:agent:abc",
 *   privateKey: "<base64 Ed25519 seed>",
 *   clientSecret: "atap_...",
 * });
 * ```
 *
 * Usage for human/org (authorization_code + PKCE):
 * ```typescript
 * const client = new ATAPClient({
 *   baseUrl: "http://localhost:8080",
 *   did: "did:web:localhost%3A8080:human:abc",
 *   privateKey: "<base64 Ed25519 seed>",
 * });
 * await client.tokenManager.obtainAuthorizationCode();
 * ```
 */
export class ATAPClient {
  /** @internal */
  readonly _http: HTTPClient;
  private readonly _did: string;
  private readonly _platformDomain: string;
  private readonly _signingKey: Uint8Array | null;
  private readonly _tokenManager: TokenManager | null;

  readonly entities: EntityAPI;
  readonly approvals: ApprovalAPI;
  readonly revocations: RevocationAPI;
  readonly didcomm: DIDCommAPI;
  readonly credentials: CredentialAPI;
  readonly discovery: DiscoveryAPI;

  constructor(options: ATAPClientOptions = {}) {
    const baseUrl = options.baseUrl || "http://localhost:8080";
    const timeout = options.timeout || 30000;

    this._http = new HTTPClient(baseUrl, timeout, options.fetch);
    this._did = options.did || "";
    this._platformDomain =
      options.platformDomain ||
      (this._did ? domainFromDID(this._did) : "localhost");

    if (options.signingKey) {
      this._signingKey = options.signingKey;
    } else if (options.privateKey) {
      this._signingKey = loadSigningKey(options.privateKey);
    } else {
      this._signingKey = null;
    }

    if (this._signingKey && this._did) {
      this._tokenManager = new TokenManager({
        httpClient: this._http,
        signingKey: this._signingKey,
        did: this._did,
        clientSecret: options.clientSecret,
        scopes: options.scopes,
        platformDomain: this._platformDomain,
      });
    } else {
      this._tokenManager = null;
    }

    // API modules
    this.entities = new EntityAPI(this);
    this.approvals = new ApprovalAPI(this);
    this.revocations = new RevocationAPI(this);
    this.didcomm = new DIDCommAPI(this);
    this.credentials = new CredentialAPI(this);
    this.discovery = new DiscoveryAPI(this);
  }

  /** Access the token manager for manual token operations. */
  get tokenManager(): TokenManager {
    if (!this._tokenManager) {
      throw new Error(
        "Token manager not initialized. Provide did and privateKey.",
      );
    }
    return this._tokenManager;
  }

  /** @internal Make an authenticated request using the token manager. */
  async _authedRequest(
    method: string,
    path: string,
    opts: AuthenticatedRequestOptions = {},
  ): Promise<Record<string, unknown>> {
    if (!this._tokenManager || !this._signingKey) {
      throw new Error(
        "Authentication not configured. Provide did, privateKey, and optionally clientSecret.",
      );
    }

    const accessToken = await this._tokenManager.getAccessToken();
    return this._http.authenticatedRequest(
      method,
      path,
      this._signingKey,
      accessToken,
      this._platformDomain,
      opts,
    );
  }
}
