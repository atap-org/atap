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
	TrustLevel1 = 1 // Email + Phone (SIMRelay)
	TrustLevel2 = 2 // World ID
	TrustLevel3 = 3 // eID + Org
)

// Entity represents any participant in the ATAP network.
type Entity struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URI  string `json:"uri"` // e.g., "agent://a1b2c3"

	// Cryptographic identity
	PublicKeyEd25519 []byte `json:"-"`
	KeyID            string `json:"key_id"`

	// Public key (base64, for API responses)
	PublicKeyBase64 string `json:"public_key,omitempty"`

	// Metadata
	Name       string `json:"name,omitempty"`
	TrustLevel int    `json:"trust_level"`

	// Auth
	TokenHash []byte `json:"-"`

	// Registry
	Registry string `json:"registry"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterRequest is the API input for agent registration.
type RegisterRequest struct {
	Name string `json:"name,omitempty"`
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	URI        string `json:"uri"`
	ID         string `json:"id"`
	Token      string `json:"token"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	KeyID      string `json:"key_id"`
}

// EntityLookupResponse is the public view of an entity.
type EntityLookupResponse struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	URI        string    `json:"uri"`
	PublicKey  string    `json:"public_key"`
	KeyID      string    `json:"key_id"`
	Name       string    `json:"name,omitempty"`
	TrustLevel int       `json:"trust_level"`
	Registry   string    `json:"registry"`
	CreatedAt  time.Time `json:"created_at"`
}

// ProblemDetail follows RFC 7807 for error responses.
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}
