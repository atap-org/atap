package atap

import (
	"context"
	"testing"
)

func TestCredentialAPI_StartEmailVerification(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/email/start": {
			Status: 200,
			Body:   map[string]interface{}{"message": "OTP sent to user@example.com"},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	msg, err := client.Credentials.StartEmailVerification(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("StartEmailVerification: %v", err)
	}
	if msg != "OTP sent to user@example.com" {
		t.Errorf("message = %q", msg)
	}
}

func TestCredentialAPI_StartEmailVerification_DefaultMessage(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/email/start": {
			Status: 200,
			Body:   map[string]interface{}{},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	msg, err := client.Credentials.StartEmailVerification(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("StartEmailVerification: %v", err)
	}
	if msg != "OTP sent" {
		t.Errorf("default message = %q, want 'OTP sent'", msg)
	}
}

func TestCredentialAPI_VerifyEmail(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/email/verify": {
			Status: 200,
			Body: map[string]interface{}{
				"id":         "cred_email_1",
				"type":       "ATAPEmailVerification",
				"credential": "vc-jwt-here",
				"issued_at":  "2024-01-01T00:00:00Z",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	cred, err := client.Credentials.VerifyEmail(context.Background(), "user@example.com", "123456")
	if err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if cred.ID != "cred_email_1" {
		t.Errorf("ID = %q", cred.ID)
	}
	if cred.Type != "ATAPEmailVerification" {
		t.Errorf("Type = %q", cred.Type)
	}
}

func TestCredentialAPI_StartPhoneVerification(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/phone/start": {
			Status: 200,
			Body:   map[string]interface{}{"message": "OTP sent"},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	msg, err := client.Credentials.StartPhoneVerification(context.Background(), "+15551234567")
	if err != nil {
		t.Fatalf("StartPhoneVerification: %v", err)
	}
	if msg != "OTP sent" {
		t.Errorf("message = %q", msg)
	}
}

func TestCredentialAPI_VerifyPhone(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/phone/verify": {
			Status: 200,
			Body: map[string]interface{}{
				"id":   "cred_phone_1",
				"type": "ATAPPhoneVerification",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	cred, err := client.Credentials.VerifyPhone(context.Background(), "+15551234567", "654321")
	if err != nil {
		t.Fatalf("VerifyPhone: %v", err)
	}
	if cred.Type != "ATAPPhoneVerification" {
		t.Errorf("Type = %q", cred.Type)
	}
}

func TestCredentialAPI_SubmitPersonhood(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/personhood": {
			Status: 200,
			Body: map[string]interface{}{
				"id":   "cred_person_1",
				"type": "ATAPPersonhood",
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	cred, err := client.Credentials.SubmitPersonhood(context.Background(), "provider-tok")
	if err != nil {
		t.Fatalf("SubmitPersonhood: %v", err)
	}
	if cred.Type != "ATAPPersonhood" {
		t.Errorf("Type = %q", cred.Type)
	}
}

func TestCredentialAPI_SubmitPersonhood_NoToken(t *testing.T) {
	routes := map[string]mockRoute{
		"POST /v1/credentials/personhood": {
			Status: 200,
			Body:   map[string]interface{}{"id": "cred_p2", "type": "ATAPPersonhood"},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	cred, err := client.Credentials.SubmitPersonhood(context.Background(), "")
	if err != nil {
		t.Fatalf("SubmitPersonhood: %v", err)
	}
	if cred.ID != "cred_p2" {
		t.Errorf("ID = %q", cred.ID)
	}
}

func TestCredentialAPI_List(t *testing.T) {
	routes := map[string]mockRoute{
		"GET /v1/credentials": {
			Status: 200,
			Body: map[string]interface{}{
				"credentials": []interface{}{
					map[string]interface{}{"id": "cred_1", "type": "ATAPEmailVerification"},
					map[string]interface{}{"id": "cred_2", "type": "ATAPPersonhood"},
				},
			},
		},
	}
	server := newOAuthMockServer(t, routes)
	defer server.Close()

	client := newTestClient(t, server)
	creds, err := client.Credentials.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("count = %d, want 2", len(creds))
	}
}

func TestCredentialAPI_StatusList(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/credentials/status/1": {
			Status: 200,
			Body: map[string]interface{}{
				"id":   "status-list-1",
				"type": "BitstringStatusListCredential",
			},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	data, err := client.Credentials.StatusList(context.Background(), "")
	if err != nil {
		t.Fatalf("StatusList: %v", err)
	}
	if data["type"] != "BitstringStatusListCredential" {
		t.Errorf("type = %v", data["type"])
	}
}

func TestCredentialAPI_StatusList_CustomID(t *testing.T) {
	server := newMockServer(t, map[string]mockRoute{
		"GET /v1/credentials/status/42": {
			Status: 200,
			Body:   map[string]interface{}{"id": "status-list-42"},
		},
	})
	defer server.Close()

	client, _ := NewClient(WithBaseURL(server.URL))
	data, err := client.Credentials.StatusList(context.Background(), "42")
	if err != nil {
		t.Fatalf("StatusList: %v", err)
	}
	if data["id"] != "status-list-42" {
		t.Errorf("id = %v", data["id"])
	}
}
