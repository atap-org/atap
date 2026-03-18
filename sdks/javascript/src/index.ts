/**
 * ATAP SDK -- JavaScript/TypeScript client for the Agent Trust and Authority Protocol.
 */

export { ATAPClient } from "./client.js";
export type { ATAPClientOptions } from "./client.js";

export {
  ATAPError,
  ATAPProblemError,
  ATAPAuthError,
  ATAPNotFoundError,
  ATAPConflictError,
  ATAPRateLimitError,
} from "./errors.js";

export type {
  Entity,
  KeyVersion,
  ApprovalSubject,
  Approval,
  Revocation,
  RevocationList,
  DIDCommMessage,
  DIDCommInbox,
  Credential,
  OAuthToken,
  DiscoveryDocument,
  DIDDocument,
  VerificationMethod,
  ProblemDetail,
} from "./models.js";

export {
  b64urlEncode,
  b64urlDecode,
  generateKeypair,
  loadSigningKey,
  jwkThumbprint,
  makeDPoPProof,
  generatePKCE,
  domainFromDID,
} from "./crypto.js";

export { HTTPClient } from "./http.js";
export { TokenManager } from "./oauth.js";
export { EntityAPI } from "./entities.js";
export { ApprovalAPI } from "./approvals.js";
export { RevocationAPI } from "./revocations.js";
export { DIDCommAPI } from "./didcomm.js";
export { CredentialAPI } from "./credentials.js";
export { DiscoveryAPI } from "./discovery.js";
