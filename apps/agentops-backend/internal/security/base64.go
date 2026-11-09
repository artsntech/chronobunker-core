package security

import (
	"encoding/base64"
	"strings"
)

// base64Encode encodes b as URL-safe base64 without padding.
func base64Encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// base64Decode decodes a URL-safe base64 string.
func base64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// splitToken splits a signed token on the first dot.
func splitToken(s string) (payload, signature string, ok bool) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
