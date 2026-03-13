// Package didcomm provides DIDComm v2.1 message types and JWE envelope encryption
// for authenticated entity-to-entity communication in the ATAP platform.
package didcomm

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// Content type constants per DIDComm v2.1 spec.
const (
	ContentTypePlain     = "application/didcomm-plain+json"
	ContentTypeSigned    = "application/didcomm-signed+json"
	ContentTypeEncrypted = "application/didcomm-encrypted+json"
)

// ATAP protocol message type constants (MSG-05).
// All types are URIs under the https://atap.dev/protocols/ namespace.
const (
	// Approval lifecycle events.
	TypeApprovalRequest  = "https://atap.dev/protocols/approval/1.0/request"
	TypeApprovalResponse = "https://atap.dev/protocols/approval/1.0/response"
	TypeApprovalRevoke   = "https://atap.dev/protocols/approval/1.0/revoke"
	TypeApprovalStatus   = "https://atap.dev/protocols/approval/1.0/status"
	TypeApprovalRejected = "https://atap.dev/protocols/approval/1.0/rejected"

	// System messages.
	TypePing          = "https://atap.dev/protocols/basic/1.0/ping"
	TypePong          = "https://atap.dev/protocols/basic/1.0/pong"
	TypeProblemReport = "https://atap.dev/protocols/report-problem/1.0/problem-report"
)

// PlaintextMessage is a DIDComm v2.1 plaintext message per the DIDComm Messaging Specification.
// https://identity.foundation/didcomm-messaging/spec/v2.1/#plaintext-message-structure
type PlaintextMessage struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	From        string         `json:"from,omitempty"`
	To          []string       `json:"to,omitempty"`
	CreatedTime int64          `json:"created_time,omitempty"`
	ExpiresTime int64          `json:"expires_time,omitempty"`
	ThreadID    string         `json:"thid,omitempty"`
	ParentID    string         `json:"pthid,omitempty"`
	Body        map[string]any `json:"body"`
	Attachments []Attachment   `json:"attachments,omitempty"`
}

// Attachment is a DIDComm v2.1 message attachment.
type Attachment struct {
	ID        string         `json:"id,omitempty"`
	MediaType string         `json:"media_type,omitempty"`
	Data      AttachmentData `json:"data"`
}

// AttachmentData holds the attachment content, either as base64-encoded bytes or inline JSON.
type AttachmentData struct {
	Base64 string `json:"base64,omitempty"`
	JSON   any    `json:"json,omitempty"`
}

// NewMessage creates a new PlaintextMessage with an auto-generated msg_ ULID ID
// and CreatedTime set to the current Unix timestamp.
func NewMessage(msgType, from string, to []string, body map[string]any) *PlaintextMessage {
	id := "msg_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	return &PlaintextMessage{
		ID:          id,
		Type:        msgType,
		From:        from,
		To:          to,
		CreatedTime: time.Now().Unix(),
		Body:        body,
	}
}
