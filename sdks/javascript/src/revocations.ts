/**
 * Revocation operations.
 */

import type { ATAPClient } from "./client.js";
import type { Revocation, RevocationList } from "./models.js";

/** Submit and query revocations. */
export class RevocationAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /**
   * Submit a revocation. Requires atap:revoke scope.
   *
   * @param approvalId - The approval ID to revoke (apr_...).
   * @param signature - JWS signature.
   * @param options.validUntil - Optional RFC3339 expiry (defaults to revoked_at + 60min).
   */
  async submit(
    approvalId: string,
    signature: string,
    options: { validUntil?: string } = {},
  ): Promise<Revocation> {
    const body: Record<string, unknown> = {
      approval_id: approvalId,
      signature,
    };
    if (options.validUntil) body.valid_until = options.validUntil;

    const data = await this._client._authedRequest(
      "POST",
      "/v1/revocations",
      { jsonBody: body },
    );
    return parseRevocation(data);
  }

  /**
   * Query active revocations for an entity (public endpoint).
   *
   * @param entityDid - The approver DID to query.
   */
  async list(entityDid: string): Promise<RevocationList> {
    const data = await this._client._http.request("GET", "/v1/revocations", {
      params: { entity: entityDid },
    });
    const revocations = (
      (data.revocations as Record<string, unknown>[]) || []
    ).map(parseRevocation);
    return {
      entity: (data.entity as string) || entityDid,
      revocations,
      checkedAt: (data.checked_at as string) || "",
    };
  }
}

export function parseRevocation(data: Record<string, unknown>): Revocation {
  return {
    id: (data.id as string) || "",
    approvalId: (data.approval_id as string) || "",
    approverDid: (data.approver_did as string) || "",
    revokedAt: (data.revoked_at as string) || "",
    expiresAt: (data.expires_at as string) || "",
  };
}
