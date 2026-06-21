package codec

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58 = newAlphabetEncoding("Base58", base58Alphabet)

// EncodeBase58 encodes bytes with the Bitcoin Base58 alphabet.
func EncodeBase58(input []byte) string {
	return base58.encode(input)
}

// DecodeBase58 decodes a Bitcoin Base58 string.
func DecodeBase58(input string) ([]byte, error) {
	return base58.decode(input)
}

// EncodeBase58String encodes a UTF-8 string with Base58.
//
// It converts the string to bytes before encoding and cannot report invalid UTF-8.
func EncodeBase58String(input string) string {
	return EncodeBase58([]byte(input))
}

// DecodeBase58String decodes Base58 bytes into a UTF-8 string.
//
// It returns an error wrapping core.ErrInvalidUTF8 when decoded bytes are not valid UTF-8.
// Use DecodeBase58 for binary payloads.
func DecodeBase58String(input string) (string, error) {
	decoded, err := DecodeBase58(input)
	if err != nil {
		return "", err
	}
	return stringFromUTF8Bytes("decode Base58 string", decoded)
}
