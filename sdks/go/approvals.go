package atap

import "context"

// ApprovalAPI provides approval creation, response, listing, and revocation.
type ApprovalAPI struct {
	client *Client
}

// Create creates an approval request. Requires atap:send scope.
func (a *ApprovalAPI) Create(ctx context.Context, fromDID, toDID string, subject ApprovalSubject, via string) (*Approval, error) {
	body := map[string]interface{}{
		"from": fromDID,
		"to":   toDID,
		"subject": map[string]interface{}{
			"type":    subject.Type,
			"label":   subject.Label,
			"payload": subject.Payload,
		},
	}
	if via != "" {
		body["via"] = via
	}

	data, err := a.client.authedRequest(ctx, "POST", "/v1/approvals", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return parseApproval(data), nil
}

// Respond responds to an approval (approve). Requires atap:send scope.
func (a *ApprovalAPI) Respond(ctx context.Context, approvalID, signature string) (*Approval, error) {
	body := map[string]interface{}{
		"signature": signature,
	}
	data, err := a.client.authedRequest(ctx, "POST", "/v1/approvals/"+approvalID+"/respond", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return parseApproval(data), nil
}

// List lists approvals addressed to the authenticated entity. Requires atap:inbox scope.
func (a *ApprovalAPI) List(ctx context.Context) ([]Approval, error) {
	data, err := a.client.authedRequest(ctx, "GET", "/v1/approvals", nil)
	if err != nil {
		return nil, err
	}
	return parseApprovalList(data), nil
}

// Revoke revokes an approval. Requires atap:revoke scope.
func (a *ApprovalAPI) Revoke(ctx context.Context, approvalID string) (*Approval, error) {
	data, err := a.client.authedRequest(ctx, "DELETE", "/v1/approvals/"+approvalID, nil)
	if err != nil {
		return nil, err
	}
	return parseApproval(data), nil
}

func parseApproval(data map[string]interface{}) *Approval {
	approval := &Approval{
		ID:          getString(data, "id"),
		State:       getString(data, "state"),
		CreatedAt:   getString(data, "created_at"),
		ValidUntil:  getString(data, "valid_until"),
		FromDID:     getString(data, "from"),
		ToDID:       getString(data, "to"),
		Via:         getString(data, "via"),
		Parent:      getString(data, "parent"),
		TemplateURL: getString(data, "template_url"),
		RespondedAt: getString(data, "responded_at"),
	}

	if sigs, ok := data["signatures"].(map[string]interface{}); ok {
		approval.Signatures = make(map[string]string)
		for k, v := range sigs {
			if s, ok := v.(string); ok {
				approval.Signatures[k] = s
			}
		}
	}

	if fanOut, ok := data["fan_out"].(float64); ok {
		n := int(fanOut)
		approval.FanOut = &n
	}

	if subj, ok := data["subject"].(map[string]interface{}); ok {
		s := &ApprovalSubject{
			Type:  getString(subj, "type"),
			Label: getString(subj, "label"),
		}
		if r, ok := subj["reversible"].(bool); ok {
			s.Reversible = r
		}
		if p, ok := subj["payload"].(map[string]interface{}); ok {
			s.Payload = p
		}
		approval.Subject = s
	}

	return approval
}

func parseApprovalList(data map[string]interface{}) []Approval {
	// Handle array-like responses via items or approvals key.
	var items []interface{}
	if arr, ok := data["approvals"].([]interface{}); ok {
		items = arr
	} else if arr, ok := data["items"].([]interface{}); ok {
		items = arr
	}

	result := make([]Approval, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, *parseApproval(m))
		}
	}
	return result
}
