package models

import (
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
	VerificationMethod []VerificationMethod  `json:"verificationMethod"`
	Authentication     []string             `json:"authentication"`
	AssertionMethod    []string             `json:"assertionMethod"`
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
	Type      string `json:"type"`
	Name      string `json:"name,omitempty"`
	PublicKey string `json:"public_key,omitempty"` // multibase-encoded Ed25519 public key
}

// CreateEntityResponse is returned after successful entity creation.
type CreateEntityResponse struct {
	ID   string `json:"id"`
	DID  string `json:"did"`
	Type string `json:"type"`
}
