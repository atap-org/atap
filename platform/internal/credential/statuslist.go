package credential

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
)

// EncodeStatusList gzip-compresses the bitstring and returns a base64url-encoded string
// per W3C Bitstring Status List v1.0 §4.
func EncodeStatusList(bits []byte) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(bits); err != nil {
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// DecodeStatusList decodes a base64url-encoded, gzip-compressed bitstring.
func DecodeStatusList(encoded string) ([]byte, error) {
	compressed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64url decode: %w", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("gzip new reader: %w", err)
	}
	defer r.Close()
	bits, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	return bits, nil
}

// SetBit sets the bit at the given index (MSB-first per W3C Bitstring Status List spec).
// The index is the zero-based position in the bitstring.
func SetBit(bits []byte, index int) {
	bits[index/8] |= (1 << (7 - uint(index%8)))
}

// CheckBit returns true if the bit at the given index is set.
func CheckBit(bits []byte, index int) bool {
	return bits[index/8]&(1<<(7-uint(index%8))) != 0
}
