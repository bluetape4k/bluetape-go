package core

import (
	"cmp"
	"fmt"
	"strings"
)

// Number 패키지에서 공개하는 인터페이스다.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// RequireNotBlank 문자열이 공백뿐이면 오류를 반환한다.
//
// 매개변수:
//   - name: RequireNotBlank가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireNotBlank가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireNotBlank(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s must not be blank", ErrInvalidArgument, name)
	}
	return nil
}

// RequireNotEmpty 문자열이 비어 있으면 오류를 반환한다.
//
// 매개변수:
//   - name: RequireNotEmpty가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireNotEmpty가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireNotEmpty(name, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s must not be empty", ErrInvalidArgument, name)
	}
	return nil
}

// RequireInRange 값이 닫힌 범위 안에 있는지 검사한다.
//
// 매개변수:
//   - name: RequireInRange가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireInRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - lower: RequireInRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: RequireInRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireInRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower > upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be <= upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value > upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v]", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequireInOpenRange 값이 열린 범위 안에 있는지 검사한다.
//
// 매개변수:
//   - name: RequireInOpenRange가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireInOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - lower: RequireInOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - upper: RequireInOpenRange에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireInOpenRange[T cmp.Ordered](name string, value, lower, upper T) error {
	if lower >= upper {
		return fmt.Errorf("%w: %s range is invalid: lower %v must be < upper %v", ErrInvalidArgument, name, lower, upper)
	}
	if value < lower || value >= upper {
		return fmt.Errorf("%w: %s[%v] must be in range [%v, %v)", ErrInvalidArgument, name, value, lower, upper)
	}
	return nil
}

// RequirePositive 값이 양수인지 검사한다.
//
// 매개변수:
//   - name: RequirePositive가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequirePositive에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequirePositive[T Number](name string, value T) error {
	var zero T
	if value <= zero {
		return fmt.Errorf("%w: %s[%v] must be positive", ErrInvalidArgument, name, value)
	}
	return nil
}

// RequireNonNegative 값이 음수가 아닌지 검사한다.
//
// 매개변수:
//   - name: RequireNonNegative가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - value: RequireNonNegative에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func RequireNonNegative[T Number](name string, value T) error {
	var zero T
	if value < zero {
		return fmt.Errorf("%w: %s[%v] must be non-negative", ErrInvalidArgument, name, value)
	}
	return nil
}
