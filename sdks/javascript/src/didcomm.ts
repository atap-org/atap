/**
 * DIDComm messaging operations.
 */

import type { ATAPClient } from "./client.js";
import type { DIDCommInbox, DIDCommMessage } from "./models.js";

/** Send and receive DIDComm messages. */
export class DIDCommAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /**
   * Send a DIDComm message (JWE envelope). Public endpoint.
   *
   * @param jweBytes - Raw JWE bytes (application/didcomm-encrypted+json).
   * @returns Dict with id and status ("queued").
   */
  async send(jweBytes: Uint8Array): Promise<Record<string, unknown>> {
    return this._client._http.request("POST", "/v1/didcomm", {
      headers: {
        "Content-Type": "application/didcomm-encrypted+json",
      },
      jsonBody: JSON.parse(new TextDecoder().decode(jweBytes)),
    });
  }

  /**
   * Retrieve pending DIDComm messages. Requires atap:inbox scope.
   *
   * @param options.limit - Max messages to return (default 50, max 100).
   */
  async inbox(options: { limit?: number } = {}): Promise<DIDCommInbox> {
    const limit = Math.min(options.limit || 50, 100);
    const data = await this._client._authedRequest(
      "GET",
      "/v1/didcomm/inbox",
      { params: { limit: String(limit) } },
    );
    const messages: DIDCommMessage[] = (
      (data.messages as Record<string, unknown>[]) || []
    ).map((m) => ({
      id: (m.id as string) || "",
      senderDid: (m.sender_did as string) || "",
      messageType: (m.message_type as string) || "",
      payload: (m.payload as string) || "",
      createdAt: (m.created_at as string) || "",
    }));
    return {
      messages,
      count: (data.count as number) || messages.length,
    };
  }
}
