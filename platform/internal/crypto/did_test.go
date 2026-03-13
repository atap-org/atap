package crypto

import (
	"crypto/ed25519"
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
				ID:       "kv2",
				EntityID: "01testid",
				PublicKey: pub2,
				KeyIndex: 2,
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
}
