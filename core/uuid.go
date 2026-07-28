package core

import (
	"fmt"
	"regexp"

	googleuuid "github.com/google/uuid"
)

// ZeroUUID는 상수 공개 값이다.
// 호출자는 이 식별자를 패키지의 오류, 옵션, 상수, 또는 기본값 계약을 비교할 때 사용한다.
const ZeroUUID = "00000000-0000-0000-0000-000000000000"

var uuidTextPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID는 IsUUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: IsUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func IsUUID(value string) bool {
	if !uuidTextPattern.MatchString(value) {
		return false
	}
	_, err := googleuuid.Parse(value)
	return err == nil
}

// CanonicalUUID는 CanonicalUUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: CanonicalUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// RequireUUID는 RequireUUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireUUID(name, value string) error {
	if !IsUUID(value) {
		return fmt.Errorf("%w: %s[%q] must be hyphenated UUID text", ErrInvalidArgument, name, value)
	}
	return nil
}

// IsZeroUUID는 IsZeroUUID 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: IsZeroUUID가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func IsZeroUUID(value string) bool {
	canonical, err := CanonicalUUID(value)
	return err == nil && canonical == ZeroUUID
}
