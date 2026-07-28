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

// EncodeUUIDURL62 EncodeUUIDURL62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: EncodeUUIDURL62가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// DecodeUUIDURL62 DecodeUUIDURL62 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: DecodeUUIDURL62가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
