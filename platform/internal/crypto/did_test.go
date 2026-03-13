package crypto

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/atap-dev/atap/platform/internal/models"
)

func TestBuildDID(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		entityType string
		entityID   string
		want       string
	}{
		{
			name:       "agent",
			domain:     "atap.app",
			entityType: "agent",
			entityID:   "01abc",
			want:       "did:web:atap.app:agent:01abc",
		},
		{
			name:       "human",
			domain:     "atap.app",
			entityType: "human",
			entityID:   "kzdvvj2umnduyauf",
			want:       "did:web:atap.app:human:kzdvvj2umnduyauf",
		},
		{
			name:       "machine",
			domain:     "atap.app",
			entityType: "machine",
			entityID:   "01xyz",
			want:       "did:web:atap.app:machine:01xyz",
		},
		{
			name:       "org",
			domain:     "atap.app",
			entityType: "org",
			entityID:   "01org",
			want:       "did:web:atap.app:org:01org",
		},
		{
			name:       "custom domain",
			domain:     "example.com",
			entityType: "agent",
			entityID:   "abc123",
			want:       "did:web:example.com:agent:abc123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildDID(tc.domain, tc.entityType, tc.entityID)
			if got != tc.want {
				t.Errorf("BuildDID(%q, %q, %q) = %q, want %q",
					tc.domain, tc.entityType, tc.entityID, got, tc.want)
			}
		})
	}
}

func TestEncodePublicKeyMultibase(t *testing.T) {
	t.Run("starts with z prefix", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error: %v", err)
		}
		encoded := EncodePublicKeyMultibase(pub)
		if !strings.HasPrefix(encoded, "z") {
			t.Errorf("EncodePublicKeyMultibase() = %q, want prefix 'z' (base58btc multibase)", encoded)
		}
	})

	t.Run("deterministic for same key", func(t *testing.T) {
		pub, _, err := GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error: %v", err)
		}
		e1 := EncodePublicKeyMultibase(pub)
		e2 := EncodePublicKeyMultibase(pub)
		if e1 != e2 {
			t.Error("EncodePublicKeyMultibase() not deterministic")
		}
	})

	t.Run("different keys produce different encodings", func(t *testing.T) {
		pub1, _, _ := GenerateKeyPair()
		pub2, _, _ := GenerateKeyPair()
		e1 := EncodePublicKeyMultibase(pub1)
		e2 := EncodePublicKeyMultibase(pub2)
		if e1 == e2 {
			t.Error("EncodePublicKeyMultibase() returned same encoding for different keys")
		}
	})

	t.Run("known vector", func(t *testing.T) {
		// Fixed seed key to produce deterministic output
		seed := make([]byte, 32)
		for i := range seed {
			seed[i] = byte(i)
		}
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)

		encoded := EncodePublicKeyMultibase(pub)
		if !strings.HasPrefix(encoded, "z") {
			t.Errorf("known vector: want 'z' prefix, got %q", encoded[:1])
		}
		// Must be a non-trivial string (base58 of 32 bytes is ~44 chars)
		if len(encoded) < 40 {
			t.Errorf("known vector: encoded too short: %d chars", len(encoded))
		}
	})
}

func TestBuildDIDDocument(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	t.Run("agent DID document", func(t *testing.T) {
		entity := &models.Entity{
			ID:               "01testid",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:01testid",
			PrincipalDID:     "did:web:atap.app:human:kzdvvj2umnduyauf",
			PublicKeyEd25519: pub,
			KeyID:            "key_ed25519_abcd1234",
		}
		kv := []models.KeyVersion{
			{
				ID:        "kv1",
				EntityID:  "01testid",
				PublicKey: pub,
				KeyIndex:  1,
			},
		}

		doc := BuildDIDDocument(entity, kv, "atap.app")

		// Check @context
		if len(doc.Context) != 3 {
			t.Errorf("@context length = %d, want 3", len(doc.Context))
		}
		if len(doc.Context) >= 1 && doc.Context[0] != "https://www.w3.org/ns/did/v1" {
			t.Errorf("context[0] = %q, want 'https://www.w3.org/ns/did/v1'", doc.Context[0])
		}
		if len(doc.Context) >= 2 && doc.Context[1] != "https://w3id.org/security/suites/ed25519-2020/v1" {
			t.Errorf("context[1] = %q, want ed25519-2020 context", doc.Context[1])
		}
		if len(doc.Context) >= 3 && doc.Context[2] != "https://atap.dev/ns/v1" {
			t.Errorf("context[2] = %q, want atap context", doc.Context[2])
		}

		// Check ID
		if doc.ID != entity.DID {
			t.Errorf("doc.ID = %q, want %q", doc.ID, entity.DID)
		}

		// Check verificationMethod
		if len(doc.VerificationMethod) != 1 {
			t.Errorf("verificationMethod length = %d, want 1", len(doc.VerificationMethod))
		}
		vm := doc.VerificationMethod[0]
		if vm.Type != "Ed25519VerificationKey2020" {
			t.Errorf("verificationMethod type = %q, want 'Ed25519VerificationKey2020'", vm.Type)
		}
		if vm.Controller != entity.DID {
			t.Errorf("verificationMethod controller = %q, want %q", vm.Controller, entity.DID)
		}
		if !strings.HasPrefix(vm.PublicKeyMultibase, "z") {
			t.Errorf("publicKeyMultibase = %q, want 'z' prefix", vm.PublicKeyMultibase)
		}

		// Check authentication references the key
		if len(doc.Authentication) != 1 {
			t.Errorf("authentication length = %d, want 1", len(doc.Authentication))
		}

		// Check assertionMethod references the key
		if len(doc.AssertionMethod) != 1 {
			t.Errorf("assertionMethod length = %d, want 1", len(doc.AssertionMethod))
		}

		// Check ATAP type
		if doc.ATAPType != "agent" {
			t.Errorf("atap:type = %q, want 'agent'", doc.ATAPType)
		}

		// Agent has principal
		if doc.ATAPPrincipal != entity.PrincipalDID {
			t.Errorf("atap:principal = %q, want %q", doc.ATAPPrincipal, entity.PrincipalDID)
		}
	})

	t.Run("human DID document has no principal", func(t *testing.T) {
		entity := &models.Entity{
			ID:               "kzdvvj2umnduyauf",
			Type:             models.EntityTypeHuman,
			DID:              "did:web:atap.app:human:kzdvvj2umnduyauf",
			PublicKeyEd25519: pub,
		}
		kv := []models.KeyVersion{
			{ID: "kv1", EntityID: "kzdvvj2umnduyauf", PublicKey: pub, KeyIndex: 1},
		}

		doc := BuildDIDDocument(entity, kv, "atap.app")

		if doc.ATAPPrincipal != "" {
			t.Errorf("human atap:principal = %q, want empty", doc.ATAPPrincipal)
		}
		if doc.ATAPType != "human" {
			t.Errorf("atap:type = %q, want 'human'", doc.ATAPType)
		}
	})

	t.Run("rotated keys include both versions", func(t *testing.T) {
		pub2, _, _ := GenerateKeyPair()
		now := nowUTC()
		entity := &models.Entity{
			ID:               "01testid",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:01testid",
			PublicKeyEd25519: pub2,
		}
		kv := []models.KeyVersion{
			{
				ID:         "kv1",
				EntityID:   "01testid",
				PublicKey:  pub,
				KeyIndex:   1,
				ValidUntil: &now,
			},
			{
				ID:        "kv2",
				EntityID:  "01testid",
				PublicKey: pub2,
				KeyIndex:  2,
			},
		}

		doc := BuildDIDDocument(entity, kv, "atap.app")

		// Both keys in verificationMethod
		if len(doc.VerificationMethod) != 2 {
			t.Errorf("verificationMethod length = %d, want 2 (for rotated keys)", len(doc.VerificationMethod))
		}

		// Only active key in authentication
		if len(doc.Authentication) != 1 {
			t.Errorf("authentication length = %d, want 1 (only active key)", len(doc.Authentication))
		}
		if len(doc.AssertionMethod) != 1 {
			t.Errorf("assertionMethod length = %d, want 1 (only active key)", len(doc.AssertionMethod))
		}
	})

	t.Run("entity without X25519 key has no keyAgreement or service", func(t *testing.T) {
		entity := &models.Entity{
			ID:               "01testid",
			Type:             models.EntityTypeAgent,
			DID:              "did:web:atap.app:agent:01testid",
			PublicKeyEd25519: pub,
			// X25519PublicKey intentionally nil
		}
		kv := []models.KeyVersion{
			{ID: "kv1", EntityID: "01testid", PublicKey: pub, KeyIndex: 1},
		}

		doc := BuildDIDDocument(entity, kv, "atap.app")

		if len(doc.KeyAgreement) != 0 {
			t.Errorf("keyAgreement = %v, want empty (no X25519 key)", doc.KeyAgreement)
		}
		if len(doc.Service) != 0 {
			t.Errorf("service = %v, want empty (no X25519 key)", doc.Service)
		}
	})
}

func TestBuildDIDDocument_WithX25519(t *testing.T) {
	pub, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error: %v", err)
	}

	x25519Priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ecdh.X25519().GenerateKey() error: %v", err)
	}
	x25519PubBytes := x25519Priv.PublicKey().Bytes()

	entity := &models.Entity{
		ID:               "01testid",
		Type:             models.EntityTypeAgent,
		DID:              "did:web:atap.app:agent:01testid",
		PrincipalDID:     "did:web:atap.app:human:kzdvvj2umnduyauf",
		PublicKeyEd25519: pub,
		X25519PublicKey:  x25519PubBytes,
	}
	kv := []models.KeyVersion{
		{ID: "kv1", EntityID: "01testid", PublicKey: pub, KeyIndex: 1},
	}

	doc := BuildDIDDocument(entity, kv, "atap.app")

	t.Run("keyAgreement field is present", func(t *testing.T) {
		if len(doc.KeyAgreement) == 0 {
			t.Fatal("keyAgreement is empty, want 1 entry")
		}
		wantID := "did:web:atap.app:agent:01testid#key-x25519-1"
		if doc.KeyAgreement[0] != wantID {
			t.Errorf("keyAgreement[0] = %q, want %q", doc.KeyAgreement[0], wantID)
		}
	})

	t.Run("X25519 verification method in verificationMethod", func(t *testing.T) {
		// Ed25519 key + X25519 key = 2 verification methods
		if len(doc.VerificationMethod) != 2 {
			t.Fatalf("verificationMethod length = %d, want 2 (Ed25519 + X25519)", len(doc.VerificationMethod))
		}
		// Last one should be X25519
		x25519VM := doc.VerificationMethod[1]
		if x25519VM.Type != "X25519KeyAgreementKey2020" {
			t.Errorf("X25519 VM type = %q, want X25519KeyAgreementKey2020", x25519VM.Type)
		}
		if x25519VM.ID != "did:web:atap.app:agent:01testid#key-x25519-1" {
			t.Errorf("X25519 VM ID = %q, want expected ID", x25519VM.ID)
		}
		if !strings.HasPrefix(x25519VM.PublicKeyMultibase, "z") {
			t.Errorf("X25519 publicKeyMultibase = %q, want 'z' prefix", x25519VM.PublicKeyMultibase)
		}
	})

	t.Run("DIDCommMessaging service endpoint present", func(t *testing.T) {
		if len(doc.Service) == 0 {
			t.Fatal("service is empty, want DIDCommMessaging service")
		}
		svc := doc.Service[0]
		if svc.Type != "DIDCommMessaging" {
			t.Errorf("service[0].type = %q, want DIDCommMessaging", svc.Type)
		}
		wantURI := "https://atap.app/v1/didcomm"
		if svc.ServiceEndpoint.URI != wantURI {
			t.Errorf("service URI = %q, want %q", svc.ServiceEndpoint.URI, wantURI)
		}
		if len(svc.ServiceEndpoint.Accept) == 0 || svc.ServiceEndpoint.Accept[0] != "didcomm/v2" {
			t.Errorf("service accept = %v, want [didcomm/v2]", svc.ServiceEndpoint.Accept)
		}
	})

	t.Run("Ed25519 keys still in authentication and assertionMethod", func(t *testing.T) {
		if len(doc.Authentication) != 1 {
			t.Errorf("authentication length = %d, want 1", len(doc.Authentication))
		}
		if len(doc.AssertionMethod) != 1 {
			t.Errorf("assertionMethod length = %d, want 1", len(doc.AssertionMethod))
		}
	})
}

func TestEncodeX25519PublicKeyMultibase(t *testing.T) {
	t.Run("starts with z prefix", func(t *testing.T) {
		priv, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("GenerateKey() error: %v", err)
		}
		encoded := EncodeX25519PublicKeyMultibase(priv.PublicKey().Bytes())
		if !strings.HasPrefix(encoded, "z") {
			t.Errorf("EncodeX25519PublicKeyMultibase() = %q, want prefix 'z' (base58btc multibase)", encoded)
		}
	})

	t.Run("32-byte X25519 key produces non-trivial string", func(t *testing.T) {
		priv, _ := ecdh.X25519().GenerateKey(rand.Reader)
		encoded := EncodeX25519PublicKeyMultibase(priv.PublicKey().Bytes())
		// base58 of 32 bytes is ~44 chars + "z" prefix = ~45 chars
		if len(encoded) < 40 {
			t.Errorf("encoded too short: %d chars", len(encoded))
		}
	})

	t.Run("deterministic for same key", func(t *testing.T) {
		priv, _ := ecdh.X25519().GenerateKey(rand.Reader)
		pub := priv.PublicKey().Bytes()
		e1 := EncodeX25519PublicKeyMultibase(pub)
		e2 := EncodeX25519PublicKeyMultibase(pub)
		if e1 != e2 {
			t.Error("EncodeX25519PublicKeyMultibase() not deterministic")
		}
	})

	t.Run("different keys produce different encodings", func(t *testing.T) {
		priv1, _ := ecdh.X25519().GenerateKey(rand.Reader)
		priv2, _ := ecdh.X25519().GenerateKey(rand.Reader)
		e1 := EncodeX25519PublicKeyMultibase(priv1.PublicKey().Bytes())
		e2 := EncodeX25519PublicKeyMultibase(priv2.PublicKey().Bytes())
		if e1 == e2 {
			t.Error("EncodeX25519PublicKeyMultibase() returned same encoding for different keys")
		}
	})
}
