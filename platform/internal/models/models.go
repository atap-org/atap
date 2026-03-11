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
	URI  string `json:"uri"` // e.g., "agent://a1b2c3"

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

// RegisterRequest is the API input for agent registration.
type RegisterRequest struct {
	Name string `json:"name,omitempty"`
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	URI        string `json:"uri"`
	ID         string `json:"id"`
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

// ============================================================
// SIGNAL TYPES
// ============================================================

// Signal source types
const (
	SignalSourceAgent    = "agent"
	SignalSourceExternal = "external"
	SignalSourceSystem   = "system"
)

// Delivery statuses
const (
	DeliveryPending   = "pending"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
)

// Channel types
const (
	ChannelTypeTrusted = "trusted"
	ChannelTypeOpen    = "open"
)

// Priority levels
const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// Default signal configuration
const (
	DefaultSignalTTL = 7 * 24 * time.Hour // 7 days
	MaxSignalPayload = 64 * 1024          // 64 KB
)

// Signal represents a message delivered through the ATAP network.
type Signal struct {
	ID      string      `json:"id"`      // sig_ + ULID
	Version string      `json:"version"` // "1"
	TS      time.Time   `json:"ts"`
	Route   SignalRoute `json:"route"`
	Trust   SignalTrust `json:"trust"`
	Signal  SignalBody  `json:"signal"`
	Context SignalContext `json:"context"`

	// Server-side fields (not exposed in JSON)
	TargetEntityID string     `json:"-"`
	DeliveryStatus string     `json:"-"` // pending/delivered/failed
	ExpiresAt      *time.Time `json:"-"`
	CreatedAt      time.Time  `json:"-"`
}

// SignalRoute describes the routing information for a signal.
type SignalRoute struct {
	Origin  string `json:"origin"`
	Target  string `json:"target"`
	ReplyTo string `json:"reply_to,omitempty"`
	Channel string `json:"channel,omitempty"`
	Thread  string `json:"thread,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// SignalTrust contains trust and signature information.
type SignalTrust struct {
	Level      int    `json:"level"`
	Signer     string `json:"signer"`
	SignerKeyID string `json:"signer_key_id"`
	Signature  string `json:"signature"`
}

// SignalBody contains the signal payload.
type SignalBody struct {
	Type      string          `json:"type"`
	Encrypted bool            `json:"encrypted"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// SignalContext contains metadata about the signal.
type SignalContext struct {
	Source      string   `json:"source"`
	Idempotency string  `json:"idempotency,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	TTL         int      `json:"ttl,omitempty"`      // seconds
	Priority    string   `json:"priority,omitempty"` // normal/high/urgent
}

// InboxResponse is a paginated inbox response.
type InboxResponse struct {
	Signals []*Signal `json:"signals"`
	HasMore bool      `json:"has_more"`
	Cursor  string    `json:"cursor,omitempty"`
}

// SendSignalRequest is the API input for sending a signal.
type SendSignalRequest struct {
	Route   SignalRoute   `json:"route"`
	Signal  SignalBody    `json:"signal"`
	Trust   SignalTrust   `json:"trust"`
	Context SignalContext `json:"context"`
}

// ============================================================
// CHANNEL TYPES
// ============================================================

// Channel represents an inbound webhook endpoint for receiving signals.
type Channel struct {
	ID            string     `json:"id"`              // chn_ + hex
	EntityID      string     `json:"entity_id"`
	WebhookURL    string     `json:"webhook_url,omitempty"`
	Label         string     `json:"label,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	Type          string     `json:"type"`             // trusted/open
	TrusteeID     string     `json:"trustee_id,omitempty"`
	Active        bool       `json:"active"`
	BasicAuthHash []byte     `json:"-"`                // bcrypt hash for open channels
	SignalCount   int64      `json:"signal_count"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// CreateChannelRequest is the API input for channel creation.
type CreateChannelRequest struct {
	Label     string   `json:"label,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Type      string   `json:"type"`               // trusted/open
	TrusteeID string   `json:"trustee_id,omitempty"`
}

// CreateChannelResponse is returned after channel creation.
type CreateChannelResponse struct {
	Channel
	BasicAuthPassword string `json:"basic_auth_password,omitempty"` // only for open channels, returned once
}

// ============================================================
// WEBHOOK TYPES
// ============================================================

// WebhookConfig stores the webhook delivery URL for an entity.
type WebhookConfig struct {
	EntityID  string    `json:"entity_id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetWebhookRequest is the API input for setting a webhook URL.
type SetWebhookRequest struct {
	URL string `json:"url"`
}

// DeliveryAttempt tracks a webhook delivery attempt.
type DeliveryAttempt struct {
	ID          string     `json:"id"`
	SignalID    string     `json:"signal_id"`
	WebhookURL  string     `json:"webhook_url"`
	Attempt     int        `json:"attempt"`
	StatusCode  int        `json:"status_code,omitempty"`
	Error       string     `json:"error,omitempty"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
