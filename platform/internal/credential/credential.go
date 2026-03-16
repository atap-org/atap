// Package credential provides W3C VC 2.0 issuance, AES-256-GCM encryption,
// and SD-JWT selective disclosure for ATAP credentials.
package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/trustbloc/kms-go/doc/jose"
	"github.com/trustbloc/vc-go/proof/creator"
	eddsakms "github.com/trustbloc/vc-go/proof/jwtproofs/eddsa"
	utiltime "github.com/trustbloc/did-go/doc/util/time"
	"github.com/trustbloc/vc-go/verifiable"
)

const (
	// atapContext is the ATAP custom JSON-LD context.
	atapContext = "https://atap.dev/ns/v1"
	// statusListBaseURL is the base URL for status list credential endpoints.
	statusListBaseURL = "https://api.atap.app/v1/credentials/status"
)

// ed25519JWTSigner is a jwt.ProofCreator-compatible signer for the creator.WithJWTAlg call.
type ed25519JWTSigner struct {
	privKey ed25519.PrivateKey
}

func (s *ed25519JWTSigner) Sign(data []byte) ([]byte, error) {
	return ed25519.Sign(s.privKey, data), nil
}

// ed25519JoseSigner implements jose.Signer for MakeSDJWT.
type ed25519JoseSigner struct {
	privKey ed25519.PrivateKey
}

func (s *ed25519JoseSigner) Sign(data []byte) ([]byte, error) {
	return ed25519.Sign(s.privKey, data), nil
}

func (s *ed25519JoseSigner) Headers() jose.Headers {
	return jose.Headers{
		jose.HeaderAlgorithm: "EdDSA",
	}
}

// newProofCreator creates a trustbloc/vc-go ProofCreator from an Ed25519 private key.
func newProofCreator(privKey ed25519.PrivateKey) *creator.ProofCreator {
	return creator.New(
		creator.WithJWTAlg(eddsakms.New(), &ed25519JWTSigner{privKey: privKey}),
	)
}

// credentialStatusEntry builds the credentialStatus custom field for a VC.
func credentialStatusEntry(statusIndex int, statusListID string) map[string]any {
	return map[string]any{
		"id":                   fmt.Sprintf("%s/%s#%d", statusListBaseURL, statusListID, statusIndex),
		"type":                 "BitstringStatusListEntry",
		"statusPurpose":        "revocation",
		"statusListIndex":      fmt.Sprintf("%d", statusIndex),
		"statusListCredential": fmt.Sprintf("%s/%s", statusListBaseURL, statusListID),
	}
}

// IssueCredential creates and signs a W3C VC 2.0 JWT for the given credential type and subject.
// The returned string is a compact JWS (header.payload.signature).
func IssueCredential(
	entityDID, credType, issuerDID, keyID string,
	pubKey ed25519.PublicKey,
	privKey ed25519.PrivateKey,
	subject map[string]any,
	statusIndex int,
	statusListID string,
) (string, error) {
	_ = pubKey // reserved for future verification method embedding

	subj := verifiable.Subject{
		ID:           entityDID,
		CustomFields: verifiable.CustomFields(subject),
	}

	now := time.Now().UTC()
	vc, err := verifiable.CreateCredential(verifiable.CredentialContents{
		Context: []string{
			"https://www.w3.org/ns/credentials/v2",
			atapContext,
		},
		Types:  []string{"VerifiableCredential", credType},
		Issuer: &verifiable.Issuer{ID: issuerDID},
		Issued: utiltime.NewTime(now),
		Subject: []verifiable.Subject{subj},
	}, verifiable.CustomFields{
		"credentialStatus": credentialStatusEntry(statusIndex, statusListID),
	})
	if err != nil {
		return "", fmt.Errorf("create credential: %w", err)
	}

	jwtClaims, err := vc.JWTClaims(false)
	if err != nil {
		return "", fmt.Errorf("jwt claims: %w", err)
	}

	proofCreator := newProofCreator(privKey)
	jwt, err := jwtClaims.MarshalJWSString(verifiable.EdDSA, proofCreator, keyID)
	if err != nil {
		return "", fmt.Errorf("marshal jws: %w", err)
	}
	return jwt, nil
}

// IssueEmailVC issues an ATAPEmailVerification credential.
func IssueEmailVC(entityDID, email, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPEmailVerification", issuerDID, keyID, pubKey, privKey,
		map[string]any{"email": email}, statusIndex, statusListID)
}

// IssuePhoneVC issues an ATAPPhoneVerification credential.
func IssuePhoneVC(entityDID, phone, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPPhoneVerification", issuerDID, keyID, pubKey, privKey,
		map[string]any{"phone": phone}, statusIndex, statusListID)
}

// IssuePersonhoodVC issues an ATAPPersonhood credential.
// No biometric data is included — the credential attests only to a ZK proof or admin assertion.
func IssuePersonhoodVC(entityDID, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPPersonhood", issuerDID, keyID, pubKey, privKey,
		map[string]any{"personhood": "verified"}, statusIndex, statusListID)
}

// IssuePrincipalVC issues an ATAPPrincipal credential (agent controlled by a principal entity).
func IssuePrincipalVC(entityDID, principalDID, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPPrincipal", issuerDID, keyID, pubKey, privKey,
		map[string]any{"principal": principalDID}, statusIndex, statusListID)
}

// IssueOrgMembershipVC issues an ATAPOrgMembership credential.
func IssueOrgMembershipVC(entityDID, orgDID, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPOrgMembership", issuerDID, keyID, pubKey, privKey,
		map[string]any{"org": orgDID}, statusIndex, statusListID)
}

// IssueIdentityVC issues an ATAPIdentity credential (full government-level identity verification).
func IssueIdentityVC(entityDID, name, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	return IssueCredential(entityDID, "ATAPIdentity", issuerDID, keyID, pubKey, privKey,
		map[string]any{"name": name}, statusIndex, statusListID)
}

// IssueEmailSDJWT issues an ATAPEmailVerification credential as an SD-JWT,
// with the email claim selectively disclosable (PRV-01, CRD-06).
func IssueEmailSDJWT(entityDID, email, issuerDID, keyID string, pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, statusIndex int, statusListID string) (string, error) {
	_ = pubKey

	subj := verifiable.Subject{
		ID:           entityDID,
		CustomFields: verifiable.CustomFields{"email": email},
	}

	now := time.Now().UTC()
	vc, err := verifiable.CreateCredential(verifiable.CredentialContents{
		Context: []string{
			"https://www.w3.org/ns/credentials/v2",
			atapContext,
		},
		Types:  []string{"VerifiableCredential", "ATAPEmailVerification"},
		Issuer: &verifiable.Issuer{ID: issuerDID},
		Issued: utiltime.NewTime(now),
		Subject: []verifiable.Subject{subj},
	}, verifiable.CustomFields{
		"credentialStatus": credentialStatusEntry(statusIndex, statusListID),
	})
	if err != nil {
		return "", fmt.Errorf("create sd-jwt credential: %w", err)
	}

	signer := &ed25519JoseSigner{privKey: privKey}
	sdjwt, err := vc.MakeSDJWT(signer, keyID)
	if err != nil {
		return "", fmt.Errorf("make sd-jwt: %w", err)
	}
	return sdjwt, nil
}

// EncryptCredential encrypts plaintext using AES-256-GCM with the given 32-byte key.
// The nonce is prepended to the ciphertext (nonce || ciphertext || auth-tag).
func EncryptCredential(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	// gcm.Seal appends ciphertext+auth-tag to nonce
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptCredential decrypts ciphertext produced by EncryptCredential.
// Returns an error if the key is wrong or the ciphertext is tampered.
func DecryptCredential(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plaintext, nil
}
