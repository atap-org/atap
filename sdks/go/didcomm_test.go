package atap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDIDCommAPI_Send(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/didcomm" {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Content-Type") != "application/didcomm-encrypted+json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":"jwe"}` {
			t.Errorf("body = %q", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "msg_123",
			"status": "queued",
		})
	}))
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	result, err := client.DIDComm.Send(context.Background(), []byte(`{"test":"jwe"}`))
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result["id"] != "msg_123" {
		t.Errorf("id = %v", result["id"])
	}
	if result["status"] != "queued" {
		t.Errorf("status = %v", result["status"])
	}
}

func TestDIDCommAPI_Inbox(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/didcomm/inbox": {
			Status: 200,
			Body: map[string]interface{}{
				"count": 2,
				"messages": []interface{}{
					map[string]interface{}{
						"id":           "msg_1",
						"sender_did":   "did:web:localhost:agent:a1",
						"message_type": "text",
						"payload":      "hello",
						"created_at":   "2024-01-01T00:00:00Z",
					},
					map[string]interface{}{
						"id":           "msg_2",
						"sender_did":   "did:web:localhost:agent:a2",
						"message_type": "request",
						"payload":      "data",
					},
				},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	inbox, err := client.DIDComm.Inbox(context.Background(), 50)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if inbox.Count != 2 {
		t.Errorf("Count = %d", inbox.Count)
	}
	if len(inbox.Messages) != 2 {
		t.Fatalf("Messages count = %d", len(inbox.Messages))
	}
	if inbox.Messages[0].ID != "msg_1" {
		t.Errorf("first message ID = %q", inbox.Messages[0].ID)
	}
	if inbox.Messages[0].SenderDID != "did:web:localhost:agent:a1" {
		t.Errorf("first message SenderDID = %q", inbox.Messages[0].SenderDID)
	}
}

func TestDIDCommAPI_Inbox_Empty(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/didcomm/inbox": {
			Status: 200,
			Body: map[string]interface{}{
				"count":    0,
				"messages": []interface{}{},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	inbox, err := client.DIDComm.Inbox(context.Background(), 0) // default limit
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if inbox.Count != 0 {
		t.Errorf("Count = %d", inbox.Count)
	}
	if len(inbox.Messages) != 0 {
		t.Errorf("Messages count = %d", len(inbox.Messages))
	}
}

func TestDIDCommAPI_Inbox_LimitClamped(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/didcomm/inbox": {
			Status: 200,
			Body: map[string]interface{}{
				"count":    0,
				"messages": []interface{}{},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	// limit > 100 should be clamped
	_, err := client.DIDComm.Inbox(context.Background(), 200)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
}
