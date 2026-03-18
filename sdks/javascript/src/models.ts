/** RFC 7807 Problem Details error response. */
export interface ProblemDetail {
  type: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
}

/** An ATAP entity (agent, machine, human, or org). */
export interface Entity {
  id: string;
  type: string;
  did: string;
  principalDid: string;
  name: string;
  keyId: string;
  publicKey: string;
  trustLevel: number;
  registry: string;
  createdAt: string;
  updatedAt: string;
  /** Only returned at registration, not stored. */
  clientSecret?: string;
  /** Only returned when server generates keypair. */
  privateKey?: string;
}

/** A versioned public key for an entity. */
export interface KeyVersion {
  id: string;
  entityId: string;
  keyIndex: number;
  validFrom: string;
  validUntil?: string;
  createdAt: string;
}

/** The purpose and payload of an approval. */
export interface ApprovalSubject {
  type: string;
  label: string;
  reversible?: boolean;
  payload?: Record<string, unknown>;
}

/** A multi-signature approval document. */
export interface Approval {
  id: string;
  state: string;
  createdAt: string;
  validUntil?: string;
  fromDid: string;
  toDid: string;
  via: string;
  parent: string;
  subject?: ApprovalSubject;
  templateUrl: string;
  signatures: Record<string, string>;
  respondedAt?: string;
  fanOut?: number;
}

/** A revocation entry for a previously-granted approval. */
export interface Revocation {
  id: string;
  approvalId: string;
  approverDid: string;
  revokedAt: string;
  expiresAt: string;
}

/** A list of active revocations for an entity. */
export interface RevocationList {
  entity: string;
  revocations: Revocation[];
  checkedAt: string;
}

/** A DIDComm message from the inbox. */
export interface DIDCommMessage {
  id: string;
  senderDid: string;
  messageType: string;
  payload: string;
  createdAt: string;
}

/** DIDComm inbox response. */
export interface DIDCommInbox {
  messages: DIDCommMessage[];
  count: number;
}

/** A W3C Verifiable Credential. */
export interface Credential {
  id: string;
  type: string;
  credential: string;
  issuedAt: string;
  revokedAt?: string;
}

/** An OAuth 2.1 token response. */
export interface OAuthToken {
  accessToken: string;
  tokenType: string;
  expiresIn: number;
  scope: string;
  refreshToken?: string;
}

/** Server discovery document from /.well-known/atap.json. */
export interface DiscoveryDocument {
  domain: string;
  apiBase: string;
  didcommEndpoint: string;
  claimTypes: string[];
  maxApprovalTtl: string;
  trustLevel: number;
  oauth?: Record<string, unknown>;
}

/** A verification method in a DID Document. */
export interface VerificationMethod {
  id: string;
  type: string;
  controller: string;
  publicKeyMultibase: string;
}

/** A W3C DID Document. */
export interface DIDDocument {
  id: string;
  context: string[];
  verificationMethod: VerificationMethod[];
  authentication: string[];
  assertionMethod: string[];
  keyAgreement: string[];
  service: Record<string, unknown>[];
  atapType: string;
  atapPrincipal: string;
}
