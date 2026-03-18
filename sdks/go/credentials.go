package atap

import "context"

// CredentialAPI provides email/phone/personhood verification and credential management.
type CredentialAPI struct {
	client *Client
}

// StartEmailVerification initiates email verification (OTP). Requires atap:manage scope.
func (c *CredentialAPI) StartEmailVerification(ctx context.Context, email string) (string, error) {
	data, err := c.client.authedRequest(ctx, "POST", "/v1/credentials/email/start", &RequestOptions{
		JSONBody: map[string]string{"email": email},
	})
	if err != nil {
		return "", err
	}
	msg := getString(data, "message")
	if msg == "" {
		msg = "OTP sent"
	}
	return msg, nil
}

// VerifyEmail verifies an email with OTP, issuing an ATAPEmailVerification VC. Requires atap:manage scope.
func (c *CredentialAPI) VerifyEmail(ctx context.Context, email, otp string) (*Credential, error) {
	data, err := c.client.authedRequest(ctx, "POST", "/v1/credentials/email/verify", &RequestOptions{
		JSONBody: map[string]string{"email": email, "otp": otp},
	})
	if err != nil {
		return nil, err
	}
	return parseCredential(data), nil
}

// StartPhoneVerification initiates phone verification (OTP). Requires atap:manage scope.
func (c *CredentialAPI) StartPhoneVerification(ctx context.Context, phone string) (string, error) {
	data, err := c.client.authedRequest(ctx, "POST", "/v1/credentials/phone/start", &RequestOptions{
		JSONBody: map[string]string{"phone": phone},
	})
	if err != nil {
		return "", err
	}
	msg := getString(data, "message")
	if msg == "" {
		msg = "OTP sent"
	}
	return msg, nil
}

// VerifyPhone verifies a phone with OTP, issuing an ATAPPhoneVerification VC. Requires atap:manage scope.
func (c *CredentialAPI) VerifyPhone(ctx context.Context, phone, otp string) (*Credential, error) {
	data, err := c.client.authedRequest(ctx, "POST", "/v1/credentials/phone/verify", &RequestOptions{
		JSONBody: map[string]string{"phone": phone, "otp": otp},
	})
	if err != nil {
		return nil, err
	}
	return parseCredential(data), nil
}

// SubmitPersonhood submits a personhood attestation, issuing an ATAPPersonhood VC. Requires atap:manage scope.
func (c *CredentialAPI) SubmitPersonhood(ctx context.Context, providerToken string) (*Credential, error) {
	body := map[string]interface{}{}
	if providerToken != "" {
		body["provider_token"] = providerToken
	}

	data, err := c.client.authedRequest(ctx, "POST", "/v1/credentials/personhood", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return parseCredential(data), nil
}

// List lists credentials for the authenticated entity. Requires atap:manage scope.
func (c *CredentialAPI) List(ctx context.Context) ([]Credential, error) {
	data, err := c.client.authedRequest(ctx, "GET", "/v1/credentials", nil)
	if err != nil {
		return nil, err
	}

	var creds []Credential
	if items, ok := data["credentials"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				creds = append(creds, *parseCredential(m))
			}
		}
	}
	return creds, nil
}

// StatusList gets the W3C Bitstring Status List VC (public endpoint).
func (c *CredentialAPI) StatusList(ctx context.Context, listID string) (map[string]interface{}, error) {
	if listID == "" {
		listID = "1"
	}
	return c.client.http.Request(ctx, "GET", "/v1/credentials/status/"+listID, nil)
}

func parseCredential(data map[string]interface{}) *Credential {
	return &Credential{
		ID:         getString(data, "id"),
		Type:       getString(data, "type"),
		Credential: getString(data, "credential"),
		IssuedAt:   getString(data, "issued_at"),
		RevokedAt:  getString(data, "revoked_at"),
	}
}
