package codec

import "encoding/hex"

// EncodeHex encodes bytes as lowercase hexadecimal.
func EncodeHex(input []byte) string {
	return hex.EncodeToString(input)
}

// DecodeHex decodes hexadecimal text.
func DecodeHex(input string) ([]byte, error) {
	return hex.DecodeString(input)
}

// EncodeHexString encodes a UTF-8 string as lowercase hexadecimal.
func EncodeHexString(input string) string {
	return EncodeHex([]byte(input))
}

// DecodeHexString decodes hexadecimal bytes into a UTF-8 string.
func DecodeHexString(input string) (string, error) {
	decoded, err := DecodeHex(input)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
