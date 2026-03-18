package atap

import (
	"context"
	"testing"
)

func TestApprovalAPI_Create(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/approvals": {
			Status: 200,
			Body: map[string]interface{}{
				"id":         "apr_123",
				"state":      "pending",
				"from":       "did:web:localhost:agent:a1",
				"to":         "did:web:localhost:human:h1",
				"created_at": "2024-01-01T00:00:00Z",
				"subject": map[string]interface{}{
					"type":  "data_access",
					"label": "Access user data",
				},
				"signatures": map[string]interface{}{},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approval, err := client.Approvals.Create(
		context.Background(),
		"did:web:localhost:agent:a1",
		"did:web:localhost:human:h1",
		ApprovalSubject{Type: "data_access", Label: "Access user data"},
		"",
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if approval.ID != "apr_123" {
		t.Errorf("ID = %q", approval.ID)
	}
	if approval.State != "pending" {
		t.Errorf("State = %q", approval.State)
	}
	if approval.Subject == nil {
		t.Fatal("Subject is nil")
	}
	if approval.Subject.Type != "data_access" {
		t.Errorf("Subject.Type = %q", approval.Subject.Type)
	}
}

func TestApprovalAPI_Create_WithVia(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/approvals": {
			Status: 200,
			Body: map[string]interface{}{
				"id":    "apr_456",
				"state": "pending",
				"via":   "did:web:localhost:machine:m1",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approval, err := client.Approvals.Create(
		context.Background(),
		"did:web:localhost:agent:a1",
		"did:web:localhost:human:h1",
		ApprovalSubject{Type: "test", Label: "Test"},
		"did:web:localhost:machine:m1",
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if approval.Via != "did:web:localhost:machine:m1" {
		t.Errorf("Via = %q", approval.Via)
	}
}

func TestApprovalAPI_Respond(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/approvals/apr_123/respond": {
			Status: 200,
			Body: map[string]interface{}{
				"id":           "apr_123",
				"state":        "approved",
				"responded_at": "2024-01-01T01:00:00Z",
				"signatures":   map[string]interface{}{"did:web:localhost:human:h1": "sig123"},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approval, err := client.Approvals.Respond(context.Background(), "apr_123", "sig123")
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if approval.State != "approved" {
		t.Errorf("State = %q", approval.State)
	}
	if len(approval.Signatures) != 1 {
		t.Errorf("Signatures count = %d", len(approval.Signatures))
	}
}

func TestApprovalAPI_List(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/approvals": {
			Status: 200,
			Body: map[string]interface{}{
				"approvals": []interface{}{
					map[string]interface{}{"id": "apr_1", "state": "pending"},
					map[string]interface{}{"id": "apr_2", "state": "approved"},
				},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approvals, err := client.Approvals.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(approvals) != 2 {
		t.Fatalf("count = %d, want 2", len(approvals))
	}
	if approvals[0].ID != "apr_1" {
		t.Errorf("first ID = %q", approvals[0].ID)
	}
}

func TestApprovalAPI_List_Items(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/approvals": {
			Status: 200,
			Body: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "apr_x"},
				},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approvals, err := client.Approvals.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("count = %d, want 1", len(approvals))
	}
}

func TestApprovalAPI_Revoke(t *testing.T) {
	routes := map[string]mockRoute{
		"DELETE /v1/approvals/apr_123": {
			Status: 200,
			Body: map[string]interface{}{
				"id":    "apr_123",
				"state": "revoked",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approval, err := client.Approvals.Revoke(context.Background(), "apr_123")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if approval.State != "revoked" {
		t.Errorf("State = %q", approval.State)
	}
}

func TestApprovalAPI_FanOut(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/approvals": {
			Status: 200,
			Body: map[string]interface{}{
				"id":      "apr_fan",
				"state":   "pending",
				"fan_out": 3,
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	approval, err := client.Approvals.Create(
		context.Background(), "from", "to",
		ApprovalSubject{Type: "t", Label: "l"}, "",
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if approval.FanOut == nil || *approval.FanOut != 3 {
		t.Errorf("FanOut = %v", approval.FanOut)
	}
}
