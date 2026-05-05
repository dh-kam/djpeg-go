package testutil

import (
	"encoding/base64"
)

// encodeBase64 encodes a byte slice to a standard base64 string.
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes a base64 string to a byte slice.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
