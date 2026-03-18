/**
 * Entity management operations.
 */

import type { ATAPClient } from "./client.js";
import type { Entity, KeyVersion } from "./models.js";

/** Entity registration, retrieval, deletion, and key rotation. */
export class EntityAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /**
   * Register a new entity.
   *
   * @param entityType - One of "agent", "machine", "human", "org".
   * @param options.name - Optional display name.
   * @param options.publicKey - Optional base64-encoded Ed25519 public key.
   * @param options.principalDid - Optional DID for agent-to-principal binding.
   * @returns Entity with id, did, type, name, keyId. For agent/machine: also includes clientSecret.
   */
  async register(
    entityType: string,
    options: {
      name?: string;
      publicKey?: string;
      principalDid?: string;
    } = {},
  ): Promise<Entity> {
    const body: Record<string, string> = { type: entityType };
    if (options.name) body.name = options.name;
    if (options.publicKey) body.public_key = options.publicKey;
    if (options.principalDid) body.principal_did = options.principalDid;

    const data = await this._client._http.request("POST", "/v1/entities", {
      jsonBody: body,
    });
    return parseEntity(data);
  }

  /** Get public entity info by ID. */
  async get(entityId: string): Promise<Entity> {
    const data = await this._client._http.request(
      "GET",
      `/v1/entities/${entityId}`,
    );
    return parseEntity(data);
  }

  /** Delete an entity (crypto-shred). Requires atap:manage scope. */
  async delete(entityId: string): Promise<void> {
    await this._client._authedRequest("DELETE", `/v1/entities/${entityId}`);
  }

  /**
   * Rotate an entity's Ed25519 public key. Requires atap:manage scope.
   *
   * @param entityId - The entity ID.
   * @param publicKey - Base64-encoded new Ed25519 public key.
   * @returns New KeyVersion with id, keyIndex, validFrom.
   */
  async rotateKey(entityId: string, publicKey: string): Promise<KeyVersion> {
    const data = await this._client._authedRequest(
      "POST",
      `/v1/entities/${entityId}/keys/rotate`,
      { jsonBody: { public_key: publicKey } },
    );
    return {
      id: (data.id as string) || "",
      entityId: (data.entity_id as string) || "",
      keyIndex: (data.key_index as number) || 0,
      validFrom: (data.valid_from as string) || "",
      validUntil: data.valid_until as string | undefined,
      createdAt: (data.created_at as string) || "",
    };
  }
}

export function parseEntity(data: Record<string, unknown>): Entity {
  return {
    id: (data.id as string) || "",
    type: (data.type as string) || "",
    did: (data.did as string) || "",
    principalDid: (data.principal_did as string) || "",
    name: (data.name as string) || "",
    keyId: (data.key_id as string) || "",
    publicKey: (data.public_key as string) || "",
    trustLevel: (data.trust_level as number) || 0,
    registry: (data.registry as string) || "",
    createdAt: (data.created_at as string) || "",
    updatedAt: (data.updated_at as string) || "",
    clientSecret: data.client_secret as string | undefined,
    privateKey: data.private_key as string | undefined,
  };
}
