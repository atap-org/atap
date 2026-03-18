package atap

import "context"

// EntityAPI provides entity registration, retrieval, deletion, and key rotation.
type EntityAPI struct {
	client *Client
}

// RegisterOptions holds optional parameters for entity registration.
type RegisterOptions struct {
	Name         string
	PublicKey    string
	PrincipalDID string
}

// Register creates a new entity.
//
// entityType is one of "agent", "machine", "human", "org".
// For agent/machine, the response includes client_secret (returned once).
// If no public key is provided, the server generates a keypair and returns private_key (once).
func (e *EntityAPI) Register(ctx context.Context, entityType string, opts *RegisterOptions) (*Entity, error) {
	body := map[string]interface{}{
		"type": entityType,
	}
	if opts != nil {
		if opts.Name != "" {
			body["name"] = opts.Name
		}
		if opts.PublicKey != "" {
			body["public_key"] = opts.PublicKey
		}
		if opts.PrincipalDID != "" {
			body["principal_did"] = opts.PrincipalDID
		}
	}

	data, err := e.client.http.Request(ctx, "POST", "/v1/entities", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return parseEntity(data), nil
}

// Get retrieves public entity info by ID.
func (e *EntityAPI) Get(ctx context.Context, entityID string) (*Entity, error) {
	data, err := e.client.http.Request(ctx, "GET", "/v1/entities/"+entityID, nil)
	if err != nil {
		return nil, err
	}
	return parseEntity(data), nil
}

// Delete removes an entity (crypto-shred). Requires atap:manage scope.
func (e *EntityAPI) Delete(ctx context.Context, entityID string) error {
	_, err := e.client.authedRequest(ctx, "DELETE", "/v1/entities/"+entityID, nil)
	return err
}

// RotateKey rotates an entity's Ed25519 public key. Requires atap:manage scope.
func (e *EntityAPI) RotateKey(ctx context.Context, entityID, publicKey string) (*KeyVersion, error) {
	body := map[string]interface{}{
		"public_key": publicKey,
	}
	data, err := e.client.authedRequest(ctx, "POST", "/v1/entities/"+entityID+"/keys/rotate", &RequestOptions{JSONBody: body})
	if err != nil {
		return nil, err
	}
	return &KeyVersion{
		ID:         getString(data, "id"),
		EntityID:   getString(data, "entity_id"),
		KeyIndex:   getInt(data, "key_index", 0),
		ValidFrom:  getString(data, "valid_from"),
		ValidUntil: getString(data, "valid_until"),
		CreatedAt:  getString(data, "created_at"),
	}, nil
}

func parseEntity(data map[string]interface{}) *Entity {
	return &Entity{
		ID:           getString(data, "id"),
		Type:         getString(data, "type"),
		DID:          getString(data, "did"),
		PrincipalDID: getString(data, "principal_did"),
		Name:         getString(data, "name"),
		KeyID:        getString(data, "key_id"),
		PublicKey:    getString(data, "public_key"),
		TrustLevel:   getInt(data, "trust_level", 0),
		Registry:     getString(data, "registry"),
		CreatedAt:    getString(data, "created_at"),
		UpdatedAt:    getString(data, "updated_at"),
		ClientSecret: getString(data, "client_secret"),
		PrivateKey:   getString(data, "private_key"),
	}
}
