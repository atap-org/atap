// Package atap provides a Go SDK for the ATAP (Agent Trust and Authority Protocol) platform.
package atap

// ProblemDetail represents an RFC 7807 Problem Details error response.
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Entity represents an ATAP entity (agent, machine, human, or org).
type Entity struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	DID          string `json:"did,omitempty"`
	PrincipalDID string `json:"principal_did,omitempty"`
	Name         string `json:"name,omitempty"`
	KeyID        string `json:"key_id,omitempty"`
	PublicKey    string `json:"public_key,omitempty"`
	TrustLevel   int    `json:"trust_level,omitempty"`
	Registry     string `json:"registry,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	// Only returned at registration, not stored.
	ClientSecret string `json:"client_secret,omitempty"`
	PrivateKey   string `json:"private_key,omitempty"`
}

// KeyVersion represents a versioned public key for an entity.
type KeyVersion struct {
	ID         string `json:"id"`
	EntityID   string `json:"entity_id,omitempty"`
	KeyIndex   int    `json:"key_index,omitempty"`
	ValidFrom  string `json:"valid_from,omitempty"`
	ValidUntil string `json:"valid_until,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// ApprovalSubject describes the purpose and payload of an approval.
type ApprovalSubject struct {
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	Reversible bool                   `json:"reversible,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// Approval represents a multi-signature approval document.
type Approval struct {
	ID          string            `json:"id"`
	State       string            `json:"state,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
	ValidUntil  string            `json:"valid_until,omitempty"`
	FromDID     string            `json:"from,omitempty"`
	ToDID       string            `json:"to,omitempty"`
	Via         string            `json:"via,omitempty"`
	Parent      string            `json:"parent,omitempty"`
	Subject     *ApprovalSubject  `json:"subject,omitempty"`
	TemplateURL string            `json:"template_url,omitempty"`
	Signatures  map[string]string `json:"signatures,omitempty"`
	RespondedAt string            `json:"responded_at,omitempty"`
	FanOut      *int              `json:"fan_out,omitempty"`
}

// Revocation represents a revocation entry for a previously-granted approval.
type Revocation struct {
	ID          string `json:"id"`
	ApprovalID  string `json:"approval_id"`
	ApproverDID string `json:"approver_did,omitempty"`
	RevokedAt   string `json:"revoked_at,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
}

// RevocationList contains active revocations for an entity.
type RevocationList struct {
	Entity      string       `json:"entity"`
	Revocations []Revocation `json:"revocations"`
	CheckedAt   string       `json:"checked_at,omitempty"`
}

// DIDCommMessage represents a DIDComm message from the inbox.
type DIDCommMessage struct {
	ID          string `json:"id"`
	SenderDID   string `json:"sender_did,omitempty"`
	MessageType string `json:"message_type,omitempty"`
	Payload     string `json:"payload,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// DIDCommInbox represents a DIDComm inbox response.
type DIDCommInbox struct {
	Messages []DIDCommMessage `json:"messages"`
	Count    int              `json:"count"`
}

// Credential represents a W3C Verifiable Credential.
type Credential struct {
	ID         string `json:"id,omitempty"`
	Type       string `json:"type,omitempty"`
	Credential string `json:"credential,omitempty"`
	IssuedAt   string `json:"issued_at,omitempty"`
	RevokedAt  string `json:"revoked_at,omitempty"`
}

// OAuthToken represents an OAuth 2.1 token response.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// DiscoveryDocument represents a server discovery document from /.well-known/atap.json.
type DiscoveryDocument struct {
	Domain          string                 `json:"domain,omitempty"`
	APIBase         string                 `json:"api_base,omitempty"`
	DIDCommEndpoint string                 `json:"didcomm_endpoint,omitempty"`
	ClaimTypes      []string               `json:"claim_types,omitempty"`
	MaxApprovalTTL  string                 `json:"max_approval_ttl,omitempty"`
	TrustLevel      int                    `json:"trust_level,omitempty"`
	OAuth           map[string]interface{} `json:"oauth,omitempty"`
}

// VerificationMethod represents a verification method in a DID Document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type,omitempty"`
	Controller         string `json:"controller,omitempty"`
	PublicKeyMultibase string `json:"publicKeyMultibase,omitempty"`
}

// DIDDocument represents a W3C DID Document.
type DIDDocument struct {
	ID                 string                   `json:"id"`
	Context            []string                 `json:"@context,omitempty"`
	VerificationMethod []VerificationMethod     `json:"verificationMethod,omitempty"`
	Authentication     []string                 `json:"authentication,omitempty"`
	AssertionMethod    []string                 `json:"assertionMethod,omitempty"`
	KeyAgreement       []string                 `json:"keyAgreement,omitempty"`
	Service            []map[string]interface{} `json:"service,omitempty"`
	ATAPType           string                   `json:"atap:type,omitempty"`
	ATAPPrincipal      string                   `json:"atap:principal,omitempty"`
}
