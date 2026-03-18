/**
 * Discovery and DID resolution operations.
 */

import type { ATAPClient } from "./client.js";
import type {
  DIDDocument,
  DiscoveryDocument,
  VerificationMethod,
} from "./models.js";

/** Server discovery and DID document resolution. */
export class DiscoveryAPI {
  private readonly _client: ATAPClient;

  constructor(client: ATAPClient) {
    this._client = client;
  }

  /** Fetch the server discovery document from /.well-known/atap.json. */
  async discover(): Promise<DiscoveryDocument> {
    const data = await this._client._http.request(
      "GET",
      "/.well-known/atap.json",
    );
    return {
      domain: (data.domain as string) || "",
      apiBase: (data.api_base as string) || "",
      didcommEndpoint: (data.didcomm_endpoint as string) || "",
      claimTypes: (data.claim_types as string[]) || [],
      maxApprovalTtl: (data.max_approval_ttl as string) || "",
      trustLevel: (data.trust_level as number) || 0,
      oauth: data.oauth as Record<string, unknown> | undefined,
    };
  }

  /**
   * Resolve an entity's DID Document.
   *
   * @param entityType - Entity type (agent, machine, human, org).
   * @param entityId - Entity ID.
   */
  async resolveDid(
    entityType: string,
    entityId: string,
  ): Promise<DIDDocument> {
    const data = await this._client._http.request(
      "GET",
      `/${entityType}/${entityId}/did.json`,
    );
    return parseDIDDocument(data);
  }

  /** Fetch the server's DID Document. */
  async serverDid(): Promise<DIDDocument> {
    const data = await this._client._http.request(
      "GET",
      "/server/platform/did.json",
    );
    return parseDIDDocument(data);
  }

  /** Check server health. */
  async health(): Promise<Record<string, unknown>> {
    return this._client._http.request("GET", "/v1/health");
  }
}

export function parseDIDDocument(
  data: Record<string, unknown>,
): DIDDocument {
  const vms = (
    (data.verificationMethod as Record<string, unknown>[]) || []
  ).map(
    (vm): VerificationMethod => ({
      id: (vm.id as string) || "",
      type: (vm.type as string) || "",
      controller: (vm.controller as string) || "",
      publicKeyMultibase: (vm.publicKeyMultibase as string) || "",
    }),
  );
  return {
    id: (data.id as string) || "",
    context: (data["@context"] as string[]) || [],
    verificationMethod: vms,
    authentication: (data.authentication as string[]) || [],
    assertionMethod: (data.assertionMethod as string[]) || [],
    keyAgreement: (data.keyAgreement as string[]) || [],
    service: (data.service as Record<string, unknown>[]) || [],
    atapType: (data["atap:type"] as string) || "",
    atapPrincipal: (data["atap:principal"] as string) || "",
  };
}
