package models

import (
	"encoding/json"
	"time"
)

// Entity types
const (
	EntityTypeAgent   = "agent"
	EntityTypeMachine = "machine"
	EntityTypeHuman   = "human"
	EntityTypeOrg     = "org"
)

// Trust levels
const (
	TrustLevel0 = 0 // Anonymous
	TrustLevel1 = 1 // Email + Phone verified
	TrustLevel2 = 2 // World ID
	TrustLevel3 = 3 // eID + Org
)

// Entity represents any participant in the ATAP network.
type Entity struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	// DID-based identity (replaces legacy URI field)
	DID          string `json:"did,omitempty"`
	PrincipalDID string `json:"principal_did,omitempty"` // for agents: controlling human/org DID

	// Client credentials (agents/machines only)
	ClientSecretHash string `json:"-"`

	// Cryptographic identity
	PublicKeyEd25519 []byte `json:"-"`
	KeyID            string `json:"key_id"`

	// X25519 key agreement (for DIDComm encrypted messaging)
	X25519PublicKey  []byte `json:"-"`
	X25519PrivateKey []byte `json:"-"`

	// Public key (base64, for API responses)
	PublicKeyBase64 string `json:"public_key,omitempty"`

	// Metadata
	Name       string `json:"name,omitempty"`
	TrustLevel int    `json:"trust_level"`

	// Registry
	Registry string `json:"registry"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProblemDetail follows RFC 7807 for error responses.
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// ============================================================
// DID TYPES
// ============================================================

// DIDDocument represents a W3C DID Document (did:web).
type DIDDocument struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	VerificationMethod []VerificationMethod `json:"verificationMethod"`
	Authentication     []string             `json:"authentication"`
	AssertionMethod    []string             `json:"assertionMethod"`
	KeyAgreement       []string             `json:"keyAgreement,omitempty"`
	Service            []DIDService         `json:"service,omitempty"`
	ATAPType           string               `json:"atap:type,omitempty"`
	ATAPPrincipal      string               `json:"atap:principal,omitempty"`
}

// VerificationMethod represents a verification method in a DID Document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// DIDService represents a service endpoint in a DID Document.
type DIDService struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"`
	ServiceEndpoint DIDServiceEndpoint `json:"serviceEndpoint"`
}

// DIDServiceEndpoint represents the endpoint details for a DID service.
type DIDServiceEndpoint struct {
	URI         string   `json:"uri"`
	Accept      []string `json:"accept"`
	RoutingKeys []string `json:"routingKeys"`
}

// DIDCommMessage maps to the didcomm_messages table for offline delivery (MSG-02).
type DIDCommMessage struct {
	ID           string     `json:"id"`
	RecipientDID string     `json:"recipient_did"`
	SenderDID    string     `json:"sender_did,omitempty"`
	MessageType  string     `json:"message_type,omitempty"`
	Payload      []byte     `json:"-"`
	State        string     `json:"state"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
}

// ============================================================
// KEY VERSION TYPES
// ============================================================

// KeyVersion represents a versioned public key for an entity (for key rotation).
type KeyVersion struct {
	ID         string     `json:"id"`
	EntityID   string     `json:"entity_id"`
	PublicKey  []byte     `json:"-"`
	KeyIndex   int        `json:"key_index"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ============================================================
// OAUTH 2.1 TYPES
// ============================================================

// OAuthAuthCode represents an OAuth 2.1 authorization code.
type OAuthAuthCode struct {
	Code          string     `json:"code"`
	EntityID      string     `json:"entity_id"`
	RedirectURI   string     `json:"redirect_uri"`
	Scope         []string   `json:"scope"`
	CodeChallenge string     `json:"code_challenge"`
	DPoPJKT       string     `json:"dpop_jkt"`
	ExpiresAt     time.Time  `json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// OAuthToken represents an OAuth 2.1 access or refresh token.
type OAuthToken struct {
	ID        string     `json:"id"`
	EntityID  string     `json:"entity_id"`
	TokenType string     `json:"token_type"` // "access" | "refresh"
	Scope     []string   `json:"scope"`
	DPoPJKT   string     `json:"dpop_jkt"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ============================================================
// API REQUEST/RESPONSE TYPES
// ============================================================

// CreateEntityRequest is the API input for entity creation.
type CreateEntityRequest struct {
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	PublicKey    string `json:"public_key,omitempty"`    // base64-encoded Ed25519 public key
	PrincipalDID string `json:"principal_did,omitempty"` // required for agent type
}

// CreateEntityResponse is returned after successful entity creation.
// ClientSecret is only populated for agent and machine types and returned once.
// PrivateKey is only populated when the server generates the keypair (public_key omitted in request).
type CreateEntityResponse struct {
	ID           string `json:"id"`
	DID          string `json:"did"`
	Type         string `json:"type"`
	Name         string `json:"name,omitempty"`
	KeyID        string `json:"key_id,omitempty"`        // initial key version ID
	ClientSecret string `json:"client_secret,omitempty"` // returned once at registration
	PrivateKey   string `json:"private_key,omitempty"`   // returned once if server-generated; base64 Ed25519 seed
}

// ============================================================
// APPROVAL TYPES
// ============================================================

// Approval state constants per spec §8.3 lifecycle.
const (
	ApprovalStateRequested = "requested"
	ApprovalStateApproved  = "approved"
	ApprovalStateDeclined  = "declined"
	ApprovalStateExpired   = "expired"
	ApprovalStateRejected  = "rejected"
	ApprovalStateConsumed  = "consumed"
	ApprovalStateRevoked   = "revoked"
)

// Approval represents a multi-signature approval document per spec §8.5-8.7.
// Server-side fields (State, RespondedAt, UpdatedAt) use json:"-" so they are
// excluded from JCS/JWS signing operations.
type Approval struct {
	AtapApproval string     `json:"atap_approval"` // always "1"
	ID           string     `json:"id"`            // "apr_" + ULID
	CreatedAt    time.Time  `json:"created_at"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"` // nil = one-time

	From   string `json:"from"`             // requester DID
	To     string `json:"to"`               // approver DID
	Via    string `json:"via,omitempty"`    // mediating system DID
	Parent string `json:"parent,omitempty"` // parent approval ID

	Subject     ApprovalSubject   `json:"subject"`
	TemplateURL string            `json:"template_url,omitempty"`
	Signatures  map[string]string `json:"signatures"` // role -> JWS compact

	// Server-side only (not part of signed document)
	State       string     `json:"-"`
	RespondedAt *time.Time `json:"-"`
	UpdatedAt   time.Time  `json:"-"`
}

// ApprovalSubject carries the purpose and payload of an approval per spec §8.7.
type ApprovalSubject struct {
	Type       string          `json:"type"` // reverse-domain
	Label      string          `json:"label"`
	Reversible bool            `json:"reversible"`
	Payload    json.RawMessage `json:"payload"` // system-specific JSON
}

// ApprovalResponse is the signed response document from the approver per spec §8.11.
type ApprovalResponse struct {
	AtapApprovalResponse string    `json:"atap_approval_response"` // always "1"
	ApprovalID           string    `json:"approval_id"`
	Status               string    `json:"status"` // "approved" | "declined"
	RespondedAt          time.Time `json:"responded_at"`
	Signature            string    `json:"signature"` // JWS from `to` entity
}

// ============================================================
// REVOCATION TYPES
// ============================================================

// Revocation represents an entry in the revocation list.
// When an approver revokes a previously-granted approval, a Revocation is
// stored so that verifiers can check the revocation list. The server does NOT
// store approvals — it stores only revocations.
type Revocation struct {
	ID          string    `json:"id"`           // "rev_" + ULID
	ApprovalID  string    `json:"approval_id"`  // the revoked approval ID
	ApproverDID string    `json:"approver_did"` // indexed for entity query
	RevokedAt   time.Time `json:"revoked_at"`
	ExpiresAt   time.Time `json:"expires_at"` // valid_until or revoked_at+60min
}

// ============================================================
// CREDENTIAL TYPES
// ============================================================

// CredentialIDPrefix is the prefix for credential IDs.
const CredentialIDPrefix = "crd_"

// Credential represents a W3C Verifiable Credential stored for an entity.
// The VC JWT content is AES-256-GCM encrypted before storage (PRV-01).
type Credential struct {
	ID           string     `json:"id"`
	EntityID     string     `json:"entity_id"`
	Type         string     `json:"type"`         // ATAPEmailVerification, etc.
	StatusIndex  int        `json:"status_index"` // index in the Bitstring Status List
	StatusListID string     `json:"status_list_id"`
	CredentialCT []byte     `json:"-"` // AES-256-GCM encrypted VC JWT ciphertext
	IssuedAt     time.Time  `json:"issued_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

// EncryptionKey holds the per-entity AES-256-GCM key for credential encryption.
// Deleting this row crypto-shreds all credentials for the entity (PRV-02).
type EncryptionKey struct {
	EntityID  string    `json:"entity_id"`
	KeyBytes  []byte    `json:"-"` // 32-byte AES-256 key — never serialized
	CreatedAt time.Time `json:"created_at"`
}

// CredentialStatusList holds the Bitstring Status List for W3C revocation checks.
type CredentialStatusList struct {
	ID        string    `json:"id"`
	Bits      []byte    `json:"-"` // raw bitstring, 16384 bytes = 131072 slots
	NextIndex int       `json:"next_index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================
// CLAIM TYPES
// ============================================================

// Claim status constants.
const (
	ClaimStatusPending  = "pending"
	ClaimStatusRedeemed = "redeemed"
	ClaimStatusExpired  = "expired"
	ClaimStatusDeclined = "declined"
)

// Claim represents a pending agent-to-human claim.
// An agent creates a claim with a short code. A human opens the claim link,
// authenticates, and approves — binding the agent to the human's identity.
type Claim struct {
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	AgentID     string     `json:"agent_id"`
	AgentName   string     `json:"agent_name"`
	Description string     `json:"description"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	RedeemedBy  string     `json:"redeemed_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	RedeemedAt  *time.Time `json:"redeemed_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

// CreateClaimRequest is the API input for claim creation (agent-authenticated).
type CreateClaimRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}

// CreateClaimResponse is returned after successful claim creation.
type CreateClaimResponse struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ============================================================
// TEMPLATE TYPES
// ============================================================

// Template defines approval rendering provided by a via system per spec §11.2.
// Uses Adaptive Cards format per spec v1.0-rc1.
type Template struct {
	AtapTemplate string          `json:"atap_template"` // always "1"
	Card         json.RawMessage `json:"card"`          // opaque Adaptive Card JSON
	Proof        TemplateProof   `json:"proof"`
}

// TemplateProof carries the JWS proof authenticating a template per spec §11.3.
type TemplateProof struct {
	KID string `json:"kid"` // did:web:...#key-id
	Alg string `json:"alg"` // "EdDSA"
	Sig string `json:"sig"` // base64url JWS
}
