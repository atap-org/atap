package atap

import "context"

// DiscoveryAPI provides server discovery and DID document resolution.
type DiscoveryAPI struct {
	client *Client
}

// Discover fetches the server discovery document from /.well-known/atap.json.
func (d *DiscoveryAPI) Discover(ctx context.Context) (*DiscoveryDocument, error) {
	data, err := d.client.http.Request(ctx, "GET", "/.well-known/atap.json", nil)
	if err != nil {
		return nil, err
	}
	doc := &DiscoveryDocument{
		Domain:          getString(data, "domain"),
		APIBase:         getString(data, "api_base"),
		DIDCommEndpoint: getString(data, "didcomm_endpoint"),
		MaxApprovalTTL:  getString(data, "max_approval_ttl"),
		TrustLevel:      getInt(data, "trust_level", 0),
	}
	if ct, ok := data["claim_types"].([]interface{}); ok {
		for _, v := range ct {
			if s, ok := v.(string); ok {
				doc.ClaimTypes = append(doc.ClaimTypes, s)
			}
		}
	}
	if oauth, ok := data["oauth"].(map[string]interface{}); ok {
		doc.OAuth = oauth
	}
	return doc, nil
}

// ResolveDID resolves an entity's DID Document.
func (d *DiscoveryAPI) ResolveDID(ctx context.Context, entityType, entityID string) (*DIDDocument, error) {
	data, err := d.client.http.Request(ctx, "GET", "/"+entityType+"/"+entityID+"/did.json", nil)
	if err != nil {
		return nil, err
	}
	return parseDIDDocument(data), nil
}

// ServerDID fetches the server's DID Document.
func (d *DiscoveryAPI) ServerDID(ctx context.Context) (*DIDDocument, error) {
	data, err := d.client.http.Request(ctx, "GET", "/server/platform/did.json", nil)
	if err != nil {
		return nil, err
	}
	return parseDIDDocument(data), nil
}

// Health checks server health.
func (d *DiscoveryAPI) Health(ctx context.Context) (map[string]interface{}, error) {
	return d.client.http.Request(ctx, "GET", "/v1/health", nil)
}

func parseDIDDocument(data map[string]interface{}) *DIDDocument {
	doc := &DIDDocument{
		ID:            getString(data, "id"),
		ATAPType:      getString(data, "atap:type"),
		ATAPPrincipal: getString(data, "atap:principal"),
	}

	if ctx, ok := data["@context"].([]interface{}); ok {
		for _, v := range ctx {
			if s, ok := v.(string); ok {
				doc.Context = append(doc.Context, s)
			}
		}
	}

	if vms, ok := data["verificationMethod"].([]interface{}); ok {
		for _, vm := range vms {
			if m, ok := vm.(map[string]interface{}); ok {
				doc.VerificationMethod = append(doc.VerificationMethod, VerificationMethod{
					ID:                 getString(m, "id"),
					Type:               getString(m, "type"),
					Controller:         getString(m, "controller"),
					PublicKeyMultibase: getString(m, "publicKeyMultibase"),
				})
			}
		}
	}

	if auth, ok := data["authentication"].([]interface{}); ok {
		for _, v := range auth {
			if s, ok := v.(string); ok {
				doc.Authentication = append(doc.Authentication, s)
			}
		}
	}

	if am, ok := data["assertionMethod"].([]interface{}); ok {
		for _, v := range am {
			if s, ok := v.(string); ok {
				doc.AssertionMethod = append(doc.AssertionMethod, s)
			}
		}
	}

	if ka, ok := data["keyAgreement"].([]interface{}); ok {
		for _, v := range ka {
			if s, ok := v.(string); ok {
				doc.KeyAgreement = append(doc.KeyAgreement, s)
			}
		}
	}

	if svc, ok := data["service"].([]interface{}); ok {
		for _, v := range svc {
			if m, ok := v.(map[string]interface{}); ok {
				doc.Service = append(doc.Service, m)
			}
		}
	}

	return doc
}
