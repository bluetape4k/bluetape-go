package core

import (
	"cmp"
	"fmt"
	"strings"
)

// Number는 interface 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// RequireNotBlank는 RequireNotBlank 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireNotBlank가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireNotBlank가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireNotBlank(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s must not be blank", ErrInvalidArgument, name)
	}
	return nil
}

// RequireNotEmpty는 RequireNotEmpty 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireNotEmpty가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireNotEmpty가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireNotEmpty(name, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s must not be empty", ErrInvalidArgument, name)
	}
	return nil
}

// RequireInRange는 RequireInRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireInRange가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireInRange 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - lower: RequireInRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: RequireInRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireInRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower > upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be <= upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value > upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v]", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequireInOpenRange는 RequireInOpenRange 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireInOpenRange가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireInOpenRange 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - lower: RequireInOpenRange 동작에 필요한 lower 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - upper: RequireInOpenRange 동작에 필요한 upper 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireInOpenRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower >= upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be < upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value >= upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v)", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequirePositive는 RequirePositive 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequirePositive가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequirePositive 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequirePositive[T Number](name string, value T) error {
	var zero T
	if value <= zero {
		return fmt.Errorf("%w: %s[%v] must be positive", ErrInvalidArgument, name, value)
	}
	return nil
}

// RequireNonNegative는 RequireNonNegative 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: RequireNonNegative가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - value: RequireNonNegative 동작에 필요한 value 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func RequireNonNegative[T Number](name string, value T) error {
	var zero T
	if value < zero {
		return fmt.Errorf("%w: %s[%v] must be non-negative", ErrInvalidArgument, name, value)
	}
	return nil
}
