package core

import (
	"fmt"
	"regexp"

	googleuuid "github.com/google/uuid"
)

// ZeroUUID 패키지에서 공개하는 상수 값이다.
const ZeroUUID = "00000000-0000-0000-0000-000000000000"

var uuidTextPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID 값이 조건을 만족하는지 반환한다.
//
// 매개변수:
//   - value: IsUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func IsUUID(value string) bool {
	if !uuidTextPattern.MatchString(value) {
		return false
	}
	_, err := googleuuid.Parse(value)
	return err == nil
}

// CanonicalUUID UUID 문자열을 canonical 형식으로 정규화한다.
//
// 매개변수:
//   - value: CanonicalUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func CanonicalUUID(value string) (string, error) {
	if !IsUUID(value) {
		return "", fmt.Errorf("%w: UUID[%q] must be hyphenated UUID text", ErrInvalidArgument, value)
	}
	parsed, err := googleuuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("%w: UUID[%q] parse failed: %w", ErrInvalidArgument, value, err)
	}
	return parsed.String(), nil
}

// RequireUUID 유효한 UUID 문자열인지 검사한다.
//
// 매개변수:
//   - name: RequireUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireUUID(name, value string) error {
	if !IsUUID(value) {
		return fmt.Errorf("%w: %s[%q] must be hyphenated UUID text", ErrInvalidArgument, name, value)
	}
	return nil
}

// IsZeroUUID 값이 조건을 만족하는지 반환한다.
//
// 매개변수:
//   - value: IsZeroUUID가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func IsZeroUUID(value string) bool {
	canonical, err := CanonicalUUID(value)
	return err == nil && canonical == ZeroUUID
}
