package codec

import (
	"fmt"
)

type alphabetEncoding struct {
	name     string
	alphabet []byte
	indexes  [128]int
}

func newAlphabetEncoding(name string, alphabet string) alphabetEncoding {
	if len(alphabet) < 2 {
		panic("codec alphabet must contain at least two characters")
	}

	encoding := alphabetEncoding{
		name:     name,
		alphabet: []byte(alphabet),
	}
	for index := range encoding.indexes {
		encoding.indexes[index] = -1
	}
	for index, char := range encoding.alphabet {
		if char >= 128 {
			panic("codec alphabet must contain ASCII characters only")
		}
		if encoding.indexes[char] >= 0 {
			panic("codec alphabet must not contain duplicate characters")
		}
		encoding.indexes[char] = index
	}
	return encoding
}

func (e alphabetEncoding) encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}

	source := make([]byte, len(input))
	copy(source, input)

	encoded := make([]byte, len(input)*138/100+2)
	outputStart := len(encoded)
	inputStart := zeros

	for inputStart < len(source) {
		remainder := divmod(source, inputStart, 256, len(e.alphabet))
		outputStart--
		encoded[outputStart] = e.alphabet[remainder]
		if source[inputStart] == 0 {
			inputStart++
		}
	}

	for outputStart < len(encoded) && encoded[outputStart] == e.alphabet[0] {
		outputStart++
	}
	for range zeros {
		outputStart--
		encoded[outputStart] = e.alphabet[0]
	}

	return string(encoded[outputStart:])
}

func (e alphabetEncoding) decode(input string) ([]byte, error) {
	if input == "" {
		return []byte{}, nil
	}

	source := make([]byte, len(input))
	for index, char := range []byte(input) {
		if char >= 128 || e.indexes[char] < 0 {
			return nil, fmt.Errorf("invalid %s character %q at position %d", e.name, char, index)
		}
		source[index] = byte(e.indexes[char])
	}

	zeros := 0
	for zeros < len(source) && source[zeros] == 0 {
		zeros++
	}

	decoded := make([]byte, len(input))
	outputStart := len(decoded)
	inputStart := zeros

	for inputStart < len(source) {
		remainder := divmod(source, inputStart, len(e.alphabet), 256)
		outputStart--
		decoded[outputStart] = byte(remainder)
		if source[inputStart] == 0 {
			inputStart++
		}
	}

	for outputStart < len(decoded) && decoded[outputStart] == 0 {
		outputStart++
	}

	result := make([]byte, zeros+len(decoded)-outputStart)
	copy(result[zeros:], decoded[outputStart:])
	return result, nil
}

func divmod(number []byte, firstDigit int, base int, divisor int) int {
	remainder := 0
	for index := firstDigit; index < len(number); index++ {
		digit := int(number[index])
		temp := remainder*base + digit
		number[index] = byte(temp / divisor)
		remainder = temp % divisor
	}
	return remainder
}
