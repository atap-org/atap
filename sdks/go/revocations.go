package atap

import "context"

// RevocationAPI provides revocation submission and querying.
type RevocationAPI struct {
	client *Client
}

// Submit submits a revocation. Requires atap:revoke scope.
func (r *RevocationAPI) Submit(ctx context.Context, approvalID, signature string, validUntil string) (*Revocation, error) {
	body := map[string]interface{}{
		"approval_id": approvalID,
		"signature":   signature,
	}
	if validUntil != "" {
		body["valid_until"] = validUntil
	}

	data, err := r.client.authedRequest(ctx, "POST", "/v1/revocations", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return parseRevocation(data), nil
}

// List queries active revocations for an entity (public endpoint).
func (r *RevocationAPI) List(ctx context.Context, entityDID string) (*RevocationList, error) {
	data, err := r.client.http.Request(ctx, "GET", "/v1/revocations", &RequestOptions{
		Params: map[string]string{"entity": entityDID},
	})
	if err != nil {
		return nil, err
	}

	var revocations []Revocation
	if items, ok := data["revocations"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				revocations = append(revocations, *parseRevocation(m))
			}
		}
	}

	return &RevocationList{
		Entity:      getString(data, "entity"),
		Revocations: revocations,
		CheckedAt:   getString(data, "checked_at"),
	}, nil
}

func parseRevocation(data map[string]interface{}) *Revocation {
	return &Revocation{
		ID:          getString(data, "id"),
		ApprovalID:  getString(data, "approval_id"),
		ApproverDID: getString(data, "approver_did"),
		RevokedAt:   getString(data, "revoked_at"),
		ExpiresAt:   getString(data, "expires_at"),
	}
}
