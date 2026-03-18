package atap

import (
	"context"
	"fmt"
)

// DIDCommAPI provides DIDComm messaging operations.
type DIDCommAPI struct {
	client *Client
}

// Send sends a DIDComm message (JWE envelope). Public endpoint.
func (d *DIDCommAPI) Send(ctx context.Context, jweBytes []byte) (map[string]interface{}, error) {
	data, err := d.client.http.Request(ctx, "POST", "/v1/didcomm", &RequestOptions{
		RawBody:     jweBytes,
		ContentType: "application/didcomm-encrypted+json",
	})
	if err != nil {
		return nil, fmt.Errorf("send didcomm message: %w", err)
	}
	return data, nil
}

// Inbox retrieves pending DIDComm messages. Requires atap:inbox scope.
func (d *DIDCommAPI) Inbox(ctx context.Context, limit int) (*DIDCommInbox, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	data, err := d.client.authedRequest(ctx, "GET", "/v1/didcomm/inbox", &RequestOptions{
		Params: map[string]string{"limit": fmt.Sprintf("%d", limit)},
	})
	if err != nil {
		return nil, err
	}

	var messages []DIDCommMessage
	if items, ok := data["messages"].([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				messages = append(messages, DIDCommMessage{
					ID:          getString(m, "id"),
					SenderDID:   getString(m, "sender_did"),
					MessageType: getString(m, "message_type"),
					Payload:     getString(m, "payload"),
					CreatedAt:   getString(m, "created_at"),
				})
			}
		}
	}

	count := getInt(data, "count", len(messages))
	return &DIDCommInbox{
		Messages: messages,
		Count:    count,
	}, nil
}
