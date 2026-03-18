/**
 * Approval operations.
 */

import type { ATAPClient } from "./client.js";
import type { Approval, ApprovalSubject } from "./models.js";

/** Create, respond to, list, and revoke approvals. */
export class ApprovalAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /**
   * Create an approval request. Requires atap:send scope.
   *
   * @param fromDid - Requester DID.
   * @param toDid - Approver DID (or org DID for fan-out).
   * @param subject - The approval subject with type, label, and payload.
   * @param options.via - Optional mediating system DID.
   */
  async create(
    fromDid: string,
    toDid: string,
    subject: ApprovalSubject,
    options: { via?: string } = {},
  ): Promise<Approval> {
    const body: Record<string, unknown> = {
      from: fromDid,
      to: toDid,
      subject: {
        type: subject.type,
        label: subject.label,
        payload: subject.payload || {},
      },
    };
    if (options.via) body.via = options.via;

    const data = await this._client._authedRequest("POST", "/v1/approvals", {
      jsonBody: body,
    });
    return parseApproval(data);
  }

  /**
   * Respond to an approval (approve). Requires atap:send scope.
   *
   * @param approvalId - The approval ID (apr_...).
   * @param signature - JWS signature from the approver.
   */
  async respond(approvalId: string, signature: string): Promise<Approval> {
    const data = await this._client._authedRequest(
      "POST",
      `/v1/approvals/${approvalId}/respond`,
      { jsonBody: { signature } },
    );
    return parseApproval(data);
  }

  /** List approvals addressed to the authenticated entity. Requires atap:inbox scope. */
  async list(): Promise<Approval[]> {
    const data = await this._client._authedRequest("GET", "/v1/approvals");
    if (Array.isArray(data)) {
      return (data as Record<string, unknown>[]).map(parseApproval);
    }
    const items =
      (data.approvals as Record<string, unknown>[]) ||
      (data.items as Record<string, unknown>[]) ||
      [];
    return items.map(parseApproval);
  }

  /** Revoke an approval. Requires atap:revoke scope. */
  async revoke(approvalId: string): Promise<Approval> {
    const data = await this._client._authedRequest(
      "DELETE",
      `/v1/approvals/${approvalId}`,
    );
    return parseApproval(data);
  }
}

export function parseApproval(data: Record<string, unknown>): Approval {
  let subject: ApprovalSubject | undefined;
  if (data.subject) {
    const s = data.subject as Record<string, unknown>;
    subject = {
      type: (s.type as string) || "",
      label: (s.label as string) || "",
      reversible: (s.reversible as boolean) || false,
      payload: s.payload as Record<string, unknown> | undefined,
    };
  }
  return {
    id: (data.id as string) || "",
    state: (data.state as string) || "",
    createdAt: (data.created_at as string) || "",
    validUntil: data.valid_until as string | undefined,
    fromDid: (data.from as string) || "",
    toDid: (data.to as string) || "",
    via: (data.via as string) || "",
    parent: (data.parent as string) || "",
    subject,
    templateUrl: (data.template_url as string) || "",
    signatures: (data.signatures as Record<string, string>) || {},
    respondedAt: data.responded_at as string | undefined,
    fanOut: data.fan_out as number | undefined,
  };
}
