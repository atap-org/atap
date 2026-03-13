package didcomm_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/atap-dev/atap/platform/internal/didcomm"
)

func TestPlaintextMessageMarshal(t *testing.T) {
	msg := &didcomm.PlaintextMessage{
		ID:          "msg_01234567890abcdef",
		Type:        didcomm.TypeApprovalRequest,
		From:        "did:web:atap.app:agent:sender",
		To:          []string{"did:web:atap.app:agent:recipient"},
		CreatedTime: 1700000000,
		Body:        map[string]any{"action": "approve"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if out["id"] != "msg_01234567890abcdef" {
		t.Errorf("expected id=msg_01234567890abcdef, got %v", out["id"])
	}
	if out["type"] != didcomm.TypeApprovalRequest {
		t.Errorf("expected type=%s, got %v", didcomm.TypeApprovalRequest, out["type"])
	}
	if out["from"] != "did:web:atap.app:agent:sender" {
		t.Errorf("expected from field, got %v", out["from"])
	}

	toSlice, ok := out["to"].([]any)
	if !ok || len(toSlice) != 1 {
		t.Errorf("expected to=[...], got %v", out["to"])
	}

	if out["created_time"] == nil {
		t.Errorf("expected created_time to be set")
	}

	body, ok := out["body"].(map[string]any)
	if !ok {
		t.Errorf("expected body to be a map, got %T", out["body"])
	}
	if body["action"] != "approve" {
		t.Errorf("expected body.action=approve, got %v", body["action"])
	}
}

func TestPlaintextMessageOmitsEmptyOptionals(t *testing.T) {
	msg := &didcomm.PlaintextMessage{
		ID:   "msg_test",
		Type: didcomm.TypePing,
		Body: map[string]any{},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	optionals := []string{"from", "to", "created_time", "expires_time", "thid", "pthid", "attachments"}
	for _, field := range optionals {
		if _, exists := out[field]; exists {
			t.Errorf("expected field %q to be omitted, but it was present", field)
		}
	}
}

func TestATAPProtocolTypeConstants(t *testing.T) {
	constants := []string{
		didcomm.TypeApprovalRequest,
		didcomm.TypeApprovalResponse,
		didcomm.TypeApprovalRevoke,
		didcomm.TypeApprovalStatus,
		didcomm.TypeApprovalRejected,
		didcomm.TypePing,
		didcomm.TypePong,
		didcomm.TypeProblemReport,
	}

	prefix := "https://atap.dev/protocols/"
	for _, c := range constants {
		if !strings.HasPrefix(c, prefix) {
			t.Errorf("constant %q does not start with %q", c, prefix)
		}
	}
}

func TestContentTypeConstants(t *testing.T) {
	if didcomm.ContentTypePlain != "application/didcomm-plain+json" {
		t.Errorf("ContentTypePlain = %q", didcomm.ContentTypePlain)
	}
	if didcomm.ContentTypeSigned != "application/didcomm-signed+json" {
		t.Errorf("ContentTypeSigned = %q", didcomm.ContentTypeSigned)
	}
	if didcomm.ContentTypeEncrypted != "application/didcomm-encrypted+json" {
		t.Errorf("ContentTypeEncrypted = %q", didcomm.ContentTypeEncrypted)
	}
}

func TestNewMessage(t *testing.T) {
	before := time.Now().Unix()
	msg := didcomm.NewMessage(
		didcomm.TypeApprovalRequest,
		"did:web:atap.app:agent:sender",
		[]string{"did:web:atap.app:agent:recipient"},
		map[string]any{"key": "value"},
	)
	after := time.Now().Unix()

	if msg == nil {
		t.Fatal("NewMessage returned nil")
	}

	if !strings.HasPrefix(msg.ID, "msg_") {
		t.Errorf("expected ID to start with msg_, got %q", msg.ID)
	}

	if len(msg.ID) <= len("msg_") {
		t.Errorf("expected non-empty ULID after msg_ prefix, got %q", msg.ID)
	}

	if msg.Type != didcomm.TypeApprovalRequest {
		t.Errorf("expected type=%s, got %s", didcomm.TypeApprovalRequest, msg.Type)
	}

	if msg.From != "did:web:atap.app:agent:sender" {
		t.Errorf("expected from set, got %s", msg.From)
	}

	if len(msg.To) != 1 || msg.To[0] != "did:web:atap.app:agent:recipient" {
		t.Errorf("expected to set, got %v", msg.To)
	}

	if msg.CreatedTime < before || msg.CreatedTime > after {
		t.Errorf("expected created_time between %d and %d, got %d", before, after, msg.CreatedTime)
	}

	if msg.Body["key"] != "value" {
		t.Errorf("expected body.key=value, got %v", msg.Body["key"])
	}
}

func TestNewMessageUniqueIDs(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		msg := didcomm.NewMessage(didcomm.TypePing, "", nil, nil)
		if ids[msg.ID] {
			t.Errorf("duplicate ID generated: %s", msg.ID)
		}
		ids[msg.ID] = true
	}
}
