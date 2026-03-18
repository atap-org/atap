package atap

import (
	"context"
	"testing"
)

func TestRevocationAPI_Submit(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/revocations": {
			Status: 200,
			Body: map[string]interface{}{
				"id":           "rev_abc",
				"approval_id":  "apr_123",
				"approver_did": "did:web:localhost:human:h1",
				"revoked_at":   "2024-01-01T00:00:00Z",
				"expires_at":   "2024-01-01T01:00:00Z",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	rev, err := client.Revocations.Submit(context.Background(), "apr_123", "sig_revoke", "")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if rev.ID != "rev_abc" {
		t.Errorf("ID = %q", rev.ID)
	}
	if rev.ApprovalID != "apr_123" {
		t.Errorf("ApprovalID = %q", rev.ApprovalID)
	}
}

func TestRevocationAPI_Submit_WithValidUntil(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/revocations": {
			Status: 200,
			Body: map[string]interface{}{
				"id":          "rev_def",
				"approval_id": "apr_456",
				"expires_at":  "2025-01-01T00:00:00Z",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	rev, err := client.Revocations.Submit(context.Background(), "apr_456", "sig", "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if rev.ExpiresAt != "2025-01-01T00:00:00Z" {
		t.Errorf("ExpiresAt = %q", rev.ExpiresAt)
	}
}

func TestRevocationAPI_List(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/revocations": {
			Status: 200,
			Body: map[string]interface{}{
				"entity":     "did:web:localhost:human:h1",
				"checked_at": "2024-01-01T00:00:00Z",
				"revocations": []interface{}{
					map[string]interface{}{
						"id":           "rev_1",
						"approval_id":  "apr_1",
						"approver_did": "did:web:localhost:human:h1",
						"revoked_at":   "2024-01-01T00:00:00Z",
						"expires_at":   "2024-01-01T01:00:00Z",
					},
					map[string]interface{}{
						"id":          "rev_2",
						"approval_id": "apr_2",
					},
				},
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	list, err := client.Revocations.List(context.Background(), "did:web:localhost:human:h1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list.Entity != "did:web:localhost:human:h1" {
		t.Errorf("Entity = %q", list.Entity)
	}
	if len(list.Revocations) != 2 {
		t.Fatalf("count = %d, want 2", len(list.Revocations))
	}
	if list.Revocations[0].ID != "rev_1" {
		t.Errorf("first ID = %q", list.Revocations[0].ID)
	}
	if list.CheckedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CheckedAt = %q", list.CheckedAt)
	}
}

func TestRevocationAPI_List_Empty(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/revocations": {
			Status: 200,
			Body: map[string]interface{}{
				"entity":      "did:web:test",
				"revocations": []interface{}{},
				"checked_at":  "2024-01-01T00:00:00Z",
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	list, err := client.Revocations.List(context.Background(), "did:web:test")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Revocations) != 0 {
		t.Errorf("expected empty revocations, got %d", len(list.Revocations))
	}
}
