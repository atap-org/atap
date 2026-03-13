package didcomm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// GenerateX25519KeyPair generates a new X25519 key pair suitable for DIDComm key agreement.
func GenerateX25519KeyPair() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate X25519 key pair: %w", err)
	}
	return priv, priv.PublicKey(), nil
}

// jweProtectedHeader represents the DIDComm v2.1 JWE protected header fields
// for ECDH-1PU+A256KW authcrypt.
type jweProtectedHeader struct {
	Alg  string     `json:"alg"`
	Enc  string     `json:"enc"`
	EPK  jweEPKField `json:"epk"`
	APU  string     `json:"apu"`
	APV  string     `json:"apv"`
	SKID string     `json:"skid"`
}

// jweEPKField is the ephemeral public key in OKP (Octet Key Pair) format.
type jweEPKField struct {
	KTY string `json:"kty"`
	CRV string `json:"crv"`
	X   string `json:"x"`
}

// jweRecipientHeader is the per-recipient unprotected header.
type jweRecipientHeader struct {
	KID string `json:"kid"`
}

// jweRecipient is one entry in the JWE recipients array.
type jweRecipient struct {
	Header       jweRecipientHeader `json:"header"`
	EncryptedKey string             `json:"encrypted_key"`
}

// jweJSON is the full DIDComm v2.1 JSON serialization of a JWE envelope.
type jweJSON struct {
	Protected  string         `json:"protected"`
	Recipients []jweRecipient `json:"recipients"`
	IV         string         `json:"iv"`
	Ciphertext string         `json:"ciphertext"`
	Tag        string         `json:"tag"`
}

// Encrypt encrypts plaintext using ECDH-1PU+A256KW / A256CBC-HS512 authcrypt.
//
// Algorithm per DIDComm v2.1 spec + IETF draft-madden-jose-ecdh-1pu-04:
//   - Key agreement: X25519 ECDH
//   - Key wrapping: AES-256-Key-Wrap (RFC 3394)
//   - Content encryption: A256CBC-HS512 (AES-256-CBC + HMAC-SHA-512)
//   - Tag-in-KDF: ciphertext auth tag is included in Z before ConcatKDF (critical for ECDH-1PU compliance)
//
// The resulting JWE is in DIDComm JSON serialization format.
func Encrypt(plaintext []byte, senderPriv *ecdh.PrivateKey, senderPub *ecdh.PublicKey, recipientPub *ecdh.PublicKey, senderKID, recipientKID string) ([]byte, error) {
	// 1. Generate ephemeral X25519 keypair.
	ephemPriv, ephemPub, err := GenerateX25519KeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}

	// 2. Compute ECDH shared secrets.
	//    Ze = ECDH(ephemeral_priv, recipient_pub) — ephemeral-static
	Ze, err := ephemPriv.ECDH(recipientPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH Ze: %w", err)
	}

	//    Zs = ECDH(sender_static_priv, recipient_pub) — static-static
	Zs, err := senderPriv.ECDH(recipientPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH Zs: %w", err)
	}

	// 3. Generate random 64-byte CEK for A256CBC-HS512.
	//    A256CBC-HS512 splits the 64-byte key: first 32 bytes = HMAC key, last 32 bytes = AES key.
	cek := make([]byte, 64)
	if _, err := rand.Read(cek); err != nil {
		return nil, fmt.Errorf("generate CEK: %w", err)
	}

	// 4. Generate random 16-byte IV for AES-CBC.
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}

	// 5. Build the protected header (needed as AAD for content encryption).
	apu := base64.RawURLEncoding.EncodeToString([]byte(senderKID))

	// apv = base64url(sha256(recipientKID)) per DIDComm spec.
	apvHash := sha256.Sum256([]byte(recipientKID))
	apv := base64.RawURLEncoding.EncodeToString(apvHash[:])

	ephemPubBytes := ephemPub.Bytes()
	header := jweProtectedHeader{
		Alg: "ECDH-1PU+A256KW",
		Enc: "A256CBC-HS512",
		EPK: jweEPKField{
			KTY: "OKP",
			CRV: "X25519",
			X:   base64.RawURLEncoding.EncodeToString(ephemPubBytes),
		},
		APU:  apu,
		APV:  apv,
		SKID: senderKID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("marshal protected header: %w", err)
	}
	protectedB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	aad := []byte(protectedB64)

	// 6. Encrypt plaintext with A256CBC-HS512.
	//    The protected header (base64url) is the AAD.
	ciphertext, tag, err := encryptA256CBCHS512(cek, iv, plaintext, aad)
	if err != nil {
		return nil, fmt.Errorf("A256CBC-HS512 encrypt: %w", err)
	}

	// 7. Tag-in-KDF (critical ECDH-1PU compliance requirement per draft v4).
	//    Z = Ze || Zs || tag — include ciphertext auth tag before key derivation.
	Z := make([]byte, 0, len(Ze)+len(Zs)+len(tag))
	Z = append(Z, Ze...)
	Z = append(Z, Zs...)
	Z = append(Z, tag...)

	// 8. Derive the 256-bit wrapping key via ConcatKDF (SHA-512).
	wrappingKey := concatKDF(Z, "ECDH-1PU+A256KW", []byte(apu), []byte(apv), 256)

	// 9. Wrap CEK with AES-256-Key-Wrap (RFC 3394).
	encryptedKey, err := aesKeyWrap(wrappingKey, cek)
	if err != nil {
		return nil, fmt.Errorf("AES key wrap: %w", err)
	}

	// 10. Assemble JWE JSON.
	jwe := jweJSON{
		Protected: protectedB64,
		Recipients: []jweRecipient{
			{
				Header:       jweRecipientHeader{KID: recipientKID},
				EncryptedKey: base64.RawURLEncoding.EncodeToString(encryptedKey),
			},
		},
		IV:         base64.RawURLEncoding.EncodeToString(iv),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
		Tag:        base64.RawURLEncoding.EncodeToString(tag),
	}

	return json.Marshal(jwe)
}

// Decrypt decrypts a DIDComm v2.1 ECDH-1PU+A256KW / A256CBC-HS512 JWE envelope.
//
// The recipient uses their private key and the sender's public key to reconstruct
// the wrapping key and unwrap the CEK, then decrypt the payload.
func Decrypt(jweBytes []byte, recipientPriv *ecdh.PrivateKey, senderPub *ecdh.PublicKey) ([]byte, error) {
	var jwe jweJSON
	if err := json.Unmarshal(jweBytes, &jwe); err != nil {
		return nil, fmt.Errorf("unmarshal JWE: %w", err)
	}

	// 1. Decode the protected header.
	headerBytes, err := base64.RawURLEncoding.DecodeString(jwe.Protected)
	if err != nil {
		return nil, fmt.Errorf("decode protected header: %w", err)
	}

	var header jweProtectedHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("parse protected header: %w", err)
	}

	if header.Alg != "ECDH-1PU+A256KW" {
		return nil, fmt.Errorf("unsupported alg: %s (want ECDH-1PU+A256KW)", header.Alg)
	}
	if header.Enc != "A256CBC-HS512" {
		return nil, fmt.Errorf("unsupported enc: %s (want A256CBC-HS512)", header.Enc)
	}

	// 2. Decode the ephemeral public key from the EPK field.
	ephemPubBytes, err := base64.RawURLEncoding.DecodeString(header.EPK.X)
	if err != nil {
		return nil, fmt.Errorf("decode ephemeral public key: %w", err)
	}

	ephemPub, err := ecdh.X25519().NewPublicKey(ephemPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse ephemeral public key: %w", err)
	}

	// 3. Compute ECDH shared secrets (recipient's perspective).
	//    Ze = ECDH(recipient_priv, ephemeral_pub)
	Ze, err := recipientPriv.ECDH(ephemPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH Ze (recipient): %w", err)
	}

	//    Zs = ECDH(recipient_priv, sender_static_pub)
	Zs, err := recipientPriv.ECDH(senderPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH Zs (recipient): %w", err)
	}

	// 4. Decode IV, ciphertext, and tag.
	iv, err := base64.RawURLEncoding.DecodeString(jwe.IV)
	if err != nil {
		return nil, fmt.Errorf("decode IV: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(jwe.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	tag, err := base64.RawURLEncoding.DecodeString(jwe.Tag)
	if err != nil {
		return nil, fmt.Errorf("decode tag: %w", err)
	}

	// 5. Tag-in-KDF: Z = Ze || Zs || tag (same construction as Encrypt).
	Z := make([]byte, 0, len(Ze)+len(Zs)+len(tag))
	Z = append(Z, Ze...)
	Z = append(Z, Zs...)
	Z = append(Z, tag...)

	// 6. Derive wrapping key via ConcatKDF with same parameters.
	apu := header.APU
	apv := header.APV
	wrappingKey := concatKDF(Z, "ECDH-1PU+A256KW", []byte(apu), []byte(apv), 256)

	// 7. Unwrap the CEK.
	if len(jwe.Recipients) == 0 {
		return nil, errors.New("JWE has no recipients")
	}

	encryptedKey, err := base64.RawURLEncoding.DecodeString(jwe.Recipients[0].EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted key: %w", err)
	}

	cek, err := aesKeyUnwrap(wrappingKey, encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("AES key unwrap: %w", err)
	}

	// 8. Decrypt and verify with A256CBC-HS512.
	//    AAD is the protected header base64url (same as during encryption).
	aad := []byte(jwe.Protected)
	plaintext, err := decryptA256CBCHS512(cek, iv, ciphertext, aad, tag)
	if err != nil {
		return nil, fmt.Errorf("A256CBC-HS512 decrypt: %w", err)
	}

	return plaintext, nil
}

// concatKDF implements NIST SP 800-56C ConcatKDF using SHA-512.
// Used to derive the AES key-wrapping key from the ECDH shared secret Z.
//
// Parameters:
//   - z: the shared secret (Ze || Zs || tag for ECDH-1PU)
//   - alg: the algorithm identifier string (e.g. "ECDH-1PU+A256KW")
//   - apu: PartyUInfo (base64url-encoded, passed as raw bytes here)
//   - apv: PartyVInfo (base64url-encoded, passed as raw bytes here)
//   - keyLenBits: desired key length in bits (256 for A256KW)
func concatKDF(z []byte, alg string, apu, apv []byte, keyLenBits int) []byte {
	// OtherInfo = algID || apu || apv || keydatalen
	// Each component is length-prefixed with a 4-byte big-endian length.
	algBytes := []byte(alg)

	var otherInfo []byte
	otherInfo = appendLenPrefixed(otherInfo, algBytes)
	otherInfo = appendLenPrefixed(otherInfo, apu)
	otherInfo = appendLenPrefixed(otherInfo, apv)

	// keydatalen is the key length in bits as a 4-byte big-endian uint32.
	keyDataLen := make([]byte, 4)
	binary.BigEndian.PutUint32(keyDataLen, uint32(keyLenBits))
	otherInfo = append(otherInfo, keyDataLen...)

	// ConcatKDF with counter=1 (one round suffices for 256-bit key with SHA-512).
	counter := make([]byte, 4)
	binary.BigEndian.PutUint32(counter, 1)

	h := sha512.New()
	h.Write(counter)
	h.Write(z)
	h.Write(otherInfo)
	digest := h.Sum(nil)

	// Return first keyLenBits/8 bytes.
	return digest[:keyLenBits/8]
}

// appendLenPrefixed appends a 4-byte big-endian length prefix followed by the data to dst.
func appendLenPrefixed(dst, data []byte) []byte {
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	dst = append(dst, length...)
	dst = append(dst, data...)
	return dst
}

// aesKeyWrap implements RFC 3394 AES Key Wrap.
// kek must be 32 bytes (AES-256); plaintext must be a multiple of 8 bytes.
func aesKeyWrap(kek, plaintext []byte) ([]byte, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("aesKeyWrap: kek must be 32 bytes, got %d", len(kek))
	}

	// Ensure plaintext length is a multiple of 8.
	if len(plaintext)%8 != 0 {
		return nil, fmt.Errorf("aesKeyWrap: plaintext length must be multiple of 8, got %d", len(plaintext))
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("aesKeyWrap: create cipher: %w", err)
	}

	// RFC 3394 Algorithm:
	// n = number of 64-bit blocks in plaintext
	n := len(plaintext) / 8

	// A = initial value (IV) = 0xA6A6A6A6A6A6A6A6
	A := [8]byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}

	// R = copy of plaintext as 64-bit blocks
	R := make([][]byte, n)
	for i := range R {
		R[i] = make([]byte, 8)
		copy(R[i], plaintext[i*8:(i+1)*8])
	}

	// s = 6n rounds
	buf := make([]byte, 16)
	for j := 0; j < 6; j++ {
		for i := 0; i < n; i++ {
			// B = AES(A || R[i])
			copy(buf[:8], A[:])
			copy(buf[8:], R[i])
			block.Encrypt(buf, buf)

			// A = MSB(64,B) XOR t where t = n*j+i+1
			t := uint64(n*j + i + 1)
			for k := 0; k < 8; k++ {
				A[k] = buf[k] ^ byte(t>>(56-8*k))
			}

			// R[i] = LSB(64,B)
			copy(R[i], buf[8:])
		}
	}

	// Output = A || R[1] || R[2] || ... || R[n]
	result := make([]byte, 8*(n+1))
	copy(result[:8], A[:])
	for i, r := range R {
		copy(result[(i+1)*8:], r)
	}

	return result, nil
}

// aesKeyUnwrap implements RFC 3394 AES Key Unwrap.
// kek must be 32 bytes (AES-256).
func aesKeyUnwrap(kek, ciphertext []byte) ([]byte, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("aesKeyUnwrap: kek must be 32 bytes, got %d", len(kek))
	}

	if len(ciphertext) < 16 || len(ciphertext)%8 != 0 {
		return nil, fmt.Errorf("aesKeyUnwrap: invalid ciphertext length %d", len(ciphertext))
	}

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("aesKeyUnwrap: create cipher: %w", err)
	}

	n := len(ciphertext)/8 - 1

	// A = first 8 bytes of ciphertext
	var A [8]byte
	copy(A[:], ciphertext[:8])

	// R = remaining blocks
	R := make([][]byte, n)
	for i := range R {
		R[i] = make([]byte, 8)
		copy(R[i], ciphertext[(i+1)*8:(i+2)*8])
	}

	buf := make([]byte, 16)
	for j := 5; j >= 0; j-- {
		for i := n - 1; i >= 0; i-- {
			// t = n*j+i+1
			t := uint64(n*j + i + 1)

			// A = A XOR t
			var At [8]byte
			for k := 0; k < 8; k++ {
				At[k] = A[k] ^ byte(t>>(56-8*k))
			}

			// B = AES_inverse(A XOR t || R[i])
			copy(buf[:8], At[:])
			copy(buf[8:], R[i])
			block.Decrypt(buf, buf)

			// A = MSB(64,B)
			copy(A[:], buf[:8])

			// R[i] = LSB(64,B)
			copy(R[i], buf[8:])
		}
	}

	// Verify integrity check value
	expected := [8]byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}
	if subtle.ConstantTimeCompare(A[:], expected[:]) != 1 {
		return nil, errors.New("aesKeyUnwrap: integrity check failed (wrong key or corrupted ciphertext)")
	}

	// Reconstruct plaintext
	plaintext := make([]byte, n*8)
	for i, r := range R {
		copy(plaintext[i*8:], r)
	}

	return plaintext, nil
}

// encryptA256CBCHS512 encrypts plaintext using A256CBC-HS512:
// AES-256-CBC for encryption + HMAC-SHA-512 truncated to 256 bits for authentication.
//
// key must be 64 bytes: key[0:32] = MAC key, key[32:64] = AES encryption key.
// iv must be 16 bytes.
// aad is the additional authenticated data (the JWE protected header base64url).
//
// Returns (ciphertext, tag, error).
func encryptA256CBCHS512(key, iv, plaintext, aad []byte) (ciphertext, tag []byte, err error) {
	if len(key) != 64 {
		return nil, nil, fmt.Errorf("encryptA256CBCHS512: key must be 64 bytes, got %d", len(key))
	}
	if len(iv) != 16 {
		return nil, nil, fmt.Errorf("encryptA256CBCHS512: iv must be 16 bytes, got %d", len(iv))
	}

	macKey := key[:32]
	encKey := key[32:]

	// Pad plaintext to AES block size (PKCS7 padding).
	padded := pkcs7Pad(plaintext, aes.BlockSize)

	// AES-256-CBC encrypt.
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encryptA256CBCHS512: create cipher: %w", err)
	}

	ciphertext = make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Compute HMAC-SHA-512 over AAD || IV || ciphertext || AL.
	// AL = AAD length in bits as 8-byte big-endian uint64.
	al := make([]byte, 8)
	binary.BigEndian.PutUint64(al, uint64(len(aad))*8)

	mac := hmac.New(sha512.New, macKey)
	mac.Write(aad)
	mac.Write(iv)
	mac.Write(ciphertext)
	mac.Write(al)
	fullTag := mac.Sum(nil)

	// Truncate to 256 bits (32 bytes) per A256CBC-HS512 spec.
	tag = fullTag[:32]

	return ciphertext, tag, nil
}

// decryptA256CBCHS512 decrypts and verifies A256CBC-HS512 ciphertext.
//
// key must be 64 bytes: key[0:32] = MAC key, key[32:64] = AES encryption key.
func decryptA256CBCHS512(key, iv, ciphertext, aad, tag []byte) ([]byte, error) {
	if len(key) != 64 {
		return nil, fmt.Errorf("decryptA256CBCHS512: key must be 64 bytes, got %d", len(key))
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("decryptA256CBCHS512: iv must be 16 bytes, got %d", len(iv))
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("decryptA256CBCHS512: ciphertext length %d is not a multiple of block size", len(ciphertext))
	}

	macKey := key[:32]
	encKey := key[32:]

	// Verify HMAC-SHA-512 before decrypting (encrypt-then-MAC).
	al := make([]byte, 8)
	binary.BigEndian.PutUint64(al, uint64(len(aad))*8)

	mac := hmac.New(sha512.New, macKey)
	mac.Write(aad)
	mac.Write(iv)
	mac.Write(ciphertext)
	mac.Write(al)
	fullTag := mac.Sum(nil)
	expectedTag := fullTag[:32]

	if subtle.ConstantTimeCompare(tag, expectedTag) != 1 {
		return nil, errors.New("decryptA256CBCHS512: authentication failed (HMAC mismatch)")
	}

	// AES-256-CBC decrypt.
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("decryptA256CBCHS512: create cipher: %w", err)
	}

	padded := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(padded, ciphertext)

	// Remove PKCS7 padding.
	plaintext, err := pkcs7Unpad(padded)
	if err != nil {
		return nil, fmt.Errorf("decryptA256CBCHS512: %w", err)
	}

	return plaintext, nil
}

// pkcs7Pad pads plaintext to a multiple of blockSize using PKCS7.
func pkcs7Pad(plaintext []byte, blockSize int) []byte {
	padding := blockSize - len(plaintext)%blockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

// pkcs7Unpad removes PKCS7 padding from plaintext.
func pkcs7Unpad(padded []byte) ([]byte, error) {
	if len(padded) == 0 {
		return nil, errors.New("pkcs7Unpad: empty input")
	}
	padding := int(padded[len(padded)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, fmt.Errorf("pkcs7Unpad: invalid padding %d", padding)
	}
	if len(padded) < padding {
		return nil, fmt.Errorf("pkcs7Unpad: input too short for padding %d", padding)
	}
	// Verify all padding bytes are correct.
	for i := len(padded) - padding; i < len(padded); i++ {
		if padded[i] != byte(padding) {
			return nil, errors.New("pkcs7Unpad: invalid padding bytes")
		}
	}
	return padded[:len(padded)-padding], nil
}
