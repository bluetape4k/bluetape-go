package codec

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/core"
	googleuuid "github.com/google/uuid"
)

const uuidByteLength = 16
const maxUUIDURL62Length = 22

// EncodeUUIDURL62 encodes hyphenated UUID text with Kotlin Url62-compatible
// numeric Base62 normalization.
func EncodeUUIDURL62(value string) (string, error) {
	canonical, err := core.CanonicalUUID(value)
	if err != nil {
		return "", fmt.Errorf("encode UUID URL62: %w", err)
	}

	parsed, err := googleuuid.Parse(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: UUID[%q] parse failed: %w", core.ErrInvalidArgument, value, err)
	}

	numericBytes := bytes.TrimLeft(parsed[:], "\x00")
	if len(numericBytes) == 0 {
		return "0", nil
	}
	return EncodeURL62(numericBytes), nil
}

// DecodeUUIDURL62 decodes Kotlin Url62-compatible UUID text and returns
// lowercase canonical hyphenated UUID text.
func DecodeUUIDURL62(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: URL62 UUID must not be blank", core.ErrInvalidArgument)
	}
	if len(value) > maxUUIDURL62Length {
		return "", fmt.Errorf("%w: URL62 UUID[%q] exceeds compact UUID length", core.ErrInvalidArgument, value)
	}

	decoded, err := DecodeURL62(value)
	if err != nil {
		return "", fmt.Errorf("%w: URL62 UUID[%q] decode failed: %w", core.ErrInvalidArgument, value, err)
	}
	if len(decoded) > uuidByteLength {
		return "", fmt.Errorf("%w: URL62 UUID[%q] exceeds 128-bit UUID size", core.ErrInvalidArgument, value)
	}

	var parsed googleuuid.UUID
	copy(parsed[uuidByteLength-len(decoded):], decoded)
	if canonical := EncodeURL62(bytes.TrimLeft(parsed[:], "\x00")); canonical != value {
		if parsed == googleuuid.Nil && value == "0" {
			return parsed.String(), nil
		}
		return "", fmt.Errorf("%w: URL62 UUID[%q] is not canonical", core.ErrInvalidArgument, value)
	}
	return parsed.String(), nil
}
