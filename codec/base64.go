package codec

import "encoding/base64"

// EncodeBase64 encodes bytes with standard padded Base64.
func EncodeBase64(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// DecodeBase64 decodes standard padded Base64.
func DecodeBase64(input string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(input)
}

// EncodeBase64URL encodes bytes with unpadded URL-safe Base64.
func EncodeBase64URL(input []byte) string {
	return base64.RawURLEncoding.EncodeToString(input)
}

// DecodeBase64URL decodes unpadded URL-safe Base64.
func DecodeBase64URL(input string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(input)
}

// EncodeBase64String encodes a UTF-8 string with standard padded Base64.
func EncodeBase64String(input string) string {
	return EncodeBase64([]byte(input))
}

// DecodeBase64String decodes standard Base64 bytes into a UTF-8 string.
func DecodeBase64String(input string) (string, error) {
	decoded, err := DecodeBase64(input)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
