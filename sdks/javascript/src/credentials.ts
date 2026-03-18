/**
 * Credential (W3C Verifiable Credentials) operations.
 */

import type { ATAPClient } from "./client.js";
import type { Credential } from "./models.js";

/** Email/phone/personhood verification and credential management. */
export class CredentialAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /**
   * Initiate email verification (OTP). Requires atap:manage scope.
   *
   * @param email - Email address to verify.
   * @returns Status message.
   */
  async startEmailVerification(email: string): Promise<string> {
    const data = await this._client._authedRequest(
      "POST",
      "/v1/credentials/email/start",
      { jsonBody: { email } },
    );
    return (data.message as string) || "OTP sent";
  }

  /**
   * Verify email with OTP, issuing ATAPEmailVerification VC. Requires atap:manage scope.
   *
   * @param email - Email address.
   * @param otp - The OTP code.
   * @returns Credential with VC JWT.
   */
  async verifyEmail(email: string, otp: string): Promise<Credential> {
    const data = await this._client._authedRequest(
      "POST",
      "/v1/credentials/email/verify",
      { jsonBody: { email, otp } },
    );
    return parseCredential(data);
  }

  /**
   * Initiate phone verification (OTP). Requires atap:manage scope.
   *
   * @param phone - Phone number (E.164 format).
   * @returns Status message.
   */
  async startPhoneVerification(phone: string): Promise<string> {
    const data = await this._client._authedRequest(
      "POST",
      "/v1/credentials/phone/start",
      { jsonBody: { phone } },
    );
    return (data.message as string) || "OTP sent";
  }

  /**
   * Verify phone with OTP, issuing ATAPPhoneVerification VC. Requires atap:manage scope.
   *
   * @param phone - Phone number.
   * @param otp - The OTP code.
   * @returns Credential with VC JWT.
   */
  async verifyPhone(phone: string, otp: string): Promise<Credential> {
    const data = await this._client._authedRequest(
      "POST",
      "/v1/credentials/phone/verify",
      { jsonBody: { phone, otp } },
    );
    return parseCredential(data);
  }

  /**
   * Submit personhood attestation, issuing ATAPPersonhood VC. Requires atap:manage scope.
   *
   * @param options.providerToken - Optional provider token.
   * @returns Credential with VC JWT.
   */
  async submitPersonhood(
    options: { providerToken?: string } = {},
  ): Promise<Credential> {
    const body: Record<string, unknown> = {};
    if (options.providerToken) body.provider_token = options.providerToken;

    const data = await this._client._authedRequest(
      "POST",
      "/v1/credentials/personhood",
      { jsonBody: body },
    );
    return parseCredential(data);
  }

  /** List credentials for the authenticated entity. Requires atap:manage scope. */
  async list(): Promise<Credential[]> {
    const data = await this._client._authedRequest("GET", "/v1/credentials");
    if (Array.isArray(data)) {
      return (data as Record<string, unknown>[]).map(parseCredential);
    }
    return ((data.credentials as Record<string, unknown>[]) || []).map(
      parseCredential,
    );
  }

  /**
   * Get W3C Bitstring Status List VC (public endpoint).
   *
   * @param listId - Status list ID (default "1").
   */
  async statusList(
    listId = "1",
  ): Promise<Record<string, unknown>> {
    return this._client._http.request(
      "GET",
      `/v1/credentials/status/${listId}`,
    );
  }
}

export function parseCredential(data: Record<string, unknown>): Credential {
  return {
    id: (data.id as string) || "",
    type: (data.type as string) || "",
    credential: (data.credential as string) || "",
    issuedAt: (data.issued_at as string) || "",
    revokedAt: data.revoked_at as string | undefined,
  };
}
