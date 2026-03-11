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
	TrustLevel1 = 1 // Email + Phone (SIMRelay)
	TrustLevel2 = 2 // World ID
	TrustLevel3 = 3 // eID + Org
)

// Delivery preferences
const (
	DeliverySSE     = "sse"
	DeliveryWebhook = "webhook"
	DeliveryPoll    = "poll"
)

// Entity represents any participant in the ATAP network.
type Entity struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URI  string `json:"uri"` // e.g., "agent://a1b2c3"

	// Cryptographic identity
	PublicKeyEd25519 []byte `json:"-"`
	PublicKeyX25519  []byte `json:"-"`
	KeyID            string `json:"key_id"`

	// Public key (base64, for API responses)
	PublicKeyBase64 string `json:"public_key,omitempty"`

	// Metadata
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	TrustLevel  int    `json:"trust_level"`

	// Delivery
	DeliveryPref string `json:"delivery_pref"`
	WebhookURL   string `json:"webhook_url,omitempty"`
	PushToken    string `json:"-"`
	PushPlatform string `json:"-"`

	// Ownership
	OwnerID string `json:"owner_id,omitempty"`
	OrgID   string `json:"org_id,omitempty"`

	// Attestations (for humans — verified properties, NOT identity)
	Attestations json.RawMessage `json:"attestations,omitempty"`

	// Recovery
	RecoveryBackup json.RawMessage `json:"-"`

	// Auth
	TokenHash []byte `json:"-"`

	// Registry
	Registry      string `json:"registry"`
	RevocationURL string `json:"revocation_url,omitempty"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// Signal is the fundamental unit of communication in ATAP.
type Signal struct {
	ID      string `json:"id"`
	Version string `json:"v"`
	TS      string `json:"ts"`

	Route   SignalRoute   `json:"route"`
	Trust   *SignalTrust  `json:"trust,omitempty"`
	Signal  SignalBody    `json:"signal"`
	Context *SignalContext `json:"context,omitempty"`

	// Delivery tracking (internal, not in JSON)
	Delivered      bool       `json:"-"`
	DeliveredAt    *time.Time `json:"-"`
	DeliveryMethod string     `json:"-"`
	CreatedAt      time.Time  `json:"-"`
	ExpiresAt      *time.Time `json:"-"`
}

type SignalRoute struct {
	Origin  string `json:"origin"`
	Target  string `json:"target"`
	ReplyTo string `json:"reply_to,omitempty"`
	Channel string `json:"channel,omitempty"`
	Thread  string `json:"thread,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

type SignalTrust struct {
	Scheme     string          `json:"scheme"`
	KeyID      string          `json:"key_id"`
	Sig        string          `json:"sig"`
	Delegation string          `json:"delegation,omitempty"`
	Enc        *SignalEncryption `json:"enc,omitempty"`
}

type SignalEncryption struct {
	Scheme       string `json:"scheme"`
	EphemeralKey string `json:"ephemeral_key"`
	Nonce        string `json:"nonce"`
}

type SignalBody struct {
	Type      string          `json:"type"`
	Encrypted bool            `json:"encrypted"`
	Data      json.RawMessage `json:"data"`
}

type SignalContext struct {
	Source      string   `json:"source,omitempty"`
	Idempotency string  `json:"idempotency,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	TTL         *int     `json:"ttl,omitempty"`
	Priority    int      `json:"priority,omitempty"`
}

// Channel is a unique inbound pathway to an entity's inbox.
type Channel struct {
	ID         string `json:"id"`
	EntityID   string `json:"entity_id"`
	WebhookURL string `json:"webhook_url"`

	Label string   `json:"label,omitempty"`
	Tags  []string `json:"tags,omitempty"`

	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RateLimit *int       `json:"rate_limit,omitempty"`

	Active    bool       `json:"active"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`

	SignalCount  int64      `json:"signal_count"`
	LastSignalAt *time.Time `json:"last_signal_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// Delegation is a signed document granting scoped authority.
type Delegation struct {
	ID      string `json:"id"`
	Version string `json:"atap_delegation"`

	PrincipalID string   `json:"principal"`
	DelegateID  string   `json:"delegate"`
	ViaIDs      []string `json:"via,omitempty"`

	Scope       DelegationScope       `json:"scope"`
	Constraints *DelegationConstraints `json:"constraints,omitempty"`

	HumanVerification json.RawMessage `json:"human_verification,omitempty"`
	Signatures        json.RawMessage `json:"signatures"`

	Status      string     `json:"status"` // active, revoked, expired
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RevokedBy   string     `json:"revoked_by,omitempty"`
	RevokeReason string    `json:"revoke_reason,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

type DelegationScope struct {
	Actions    []string        `json:"actions"`
	SpendLimit json.RawMessage `json:"spend_limit,omitempty"`
	DataClasses []string       `json:"data_classes,omitempty"`
	Expires    time.Time       `json:"expires"`
}

type DelegationConstraints struct {
	Geo          []string        `json:"geo,omitempty"`
	TimeWindow   json.RawMessage `json:"time_window,omitempty"`
	ConfirmAbove json.RawMessage `json:"confirm_above,omitempty"`
}

// Claim is an agent's request for a human to assert authority.
type Claim struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	ClaimCode string `json:"claim_code"`

	RequestedScopes []string        `json:"requested_scopes,omitempty"`
	MachineID       string          `json:"machine_id,omitempty"`
	Description     string          `json:"description,omitempty"`
	Context         json.RawMessage `json:"context,omitempty"`

	Status       string   `json:"status"` // pending, approved, declined, expired
	ApprovedBy   string   `json:"approved_by,omitempty"`
	DelegationID string   `json:"delegation_id,omitempty"`
	ApprovedScopes []string `json:"approved_scopes,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// ApprovalTemplate defines a branded approval screen.
type ApprovalTemplate struct {
	ID        string `json:"id"`
	MachineID string `json:"machine_id"`
	Version   int    `json:"version"`

	BrandName      string          `json:"brand_name"`
	BrandLogoURL   string          `json:"brand_logo_url,omitempty"`
	BrandColors    json.RawMessage `json:"brand_colors,omitempty"`
	VerifiedDomain string          `json:"verified_domain,omitempty"`
	DomainVerified bool            `json:"domain_verified"`

	ApprovalType      string          `json:"approval_type"`
	Title             string          `json:"title"`
	Description       string          `json:"description,omitempty"`
	DisplayFields     json.RawMessage `json:"display_fields"`
	RequiredTrustLevel int            `json:"required_trust_level"`
	ScopesRequested   []string        `json:"scopes_requested,omitempty"`

	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterRequest is the API input for agent registration.
type RegisterRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	DeliveryPref string `json:"delivery_pref,omitempty"`
	WebhookURL  string `json:"webhook_url,omitempty"`
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	URI       string `json:"uri"`
	ID        string `json:"id"`
	Token     string `json:"token"`
	PublicKey string `json:"public_key"`
	KeyID     string `json:"key_id"`
	InboxURL  string `json:"inbox_url"`
	StreamURL string `json:"stream_url"`
}

// SendSignalRequest is the API input for sending a signal.
type SendSignalRequest struct {
	Data     json.RawMessage `json:"data"`
	Type     string          `json:"type,omitempty"`
	Thread   string          `json:"thread,omitempty"`
	Ref      string          `json:"ref,omitempty"`
	TTL      *int            `json:"ttl,omitempty"`
	Priority *int            `json:"priority,omitempty"`
	Tags     []string        `json:"tags,omitempty"`
}

// ProblemDetail follows RFC 7807 for error responses.
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}
