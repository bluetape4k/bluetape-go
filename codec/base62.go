package codec

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var base62 = newAlphabetEncoding("Base62", base62Alphabet)

// EncodeBase62 encodes bytes with the 0-9A-Z-a-z Base62 alphabet.
func EncodeBase62(input []byte) string {
	return base62.encode(input)
}

// DecodeBase62 decodes a Base62 string.
func DecodeBase62(input string) ([]byte, error) {
	return base62.decode(input)
}

// EncodeBase62String encodes a UTF-8 string with Base62.
func EncodeBase62String(input string) string {
	return EncodeBase62([]byte(input))
}

// DecodeBase62String decodes Base62 bytes into a UTF-8 string.
func DecodeBase62String(input string) (string, error) {
	decoded, err := DecodeBase62(input)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// EncodeURL62 is an alias for EncodeBase62.
//
// Base62 output contains only ASCII letters and digits, so it is already safe
// for path segments, query values, and Redis key fragments without escaping.
func EncodeURL62(input []byte) string {
	return EncodeBase62(input)
}

// DecodeURL62 is an alias for DecodeBase62.
func DecodeURL62(input string) ([]byte, error) {
	return DecodeBase62(input)
}
