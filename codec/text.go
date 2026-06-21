package codec

import (
	"fmt"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/core"
)

func stringFromUTF8Bytes(operation string, data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("%s: %w", operation, core.ErrInvalidUTF8)
	}
	return string(data), nil
}
