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
//
// It converts the string to bytes before encoding and cannot report invalid UTF-8.
func EncodeHexString(input string) string {
	return EncodeHex([]byte(input))
}

// DecodeHexString decodes hexadecimal bytes into a UTF-8 string.
//
// It returns an error wrapping core.ErrInvalidUTF8 when decoded bytes are not valid UTF-8.
// Use DecodeHex for binary payloads.
func DecodeHexString(input string) (string, error) {
	decoded, err := DecodeHex(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Hex string", decoded)
}
