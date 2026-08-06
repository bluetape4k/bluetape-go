package measure

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidUnit  단위 이름, suffix, ratio, registry가 유효하지 않을 때 사용합니다.
	ErrInvalidUnit = errors.New("measure: invalid unit")
	// ErrInvalidMeasure  측정값 또는 온도 값이 유효하지 않을 때 사용합니다.
	ErrInvalidMeasure = errors.New("measure: invalid measure")
	// ErrIncompatibleUnit  단위 차원 또는 단위 관계가 호환되지 않을 때 사용합니다.
	ErrIncompatibleUnit = errors.New("measure: incompatible unit")
	// ErrInvalidParse  측정값 문자열을 파싱할 수 없을 때 사용합니다.
	ErrInvalidParse = errors.New("measure: invalid parse")
	// ErrDivideByZero  0으로 나누는 측정 연산에 사용합니다.
	ErrDivideByZero = errors.New("measure: divide by zero")
)

// ParseError 패키지에서 공개하는 구조체다.
type ParseError struct {
	Input  string
	Suffix string
	Err    error
}

// Error 오류 메시지를 반환한다.
func (e ParseError) Error() string {
	if e.Suffix == "" {
		return fmt.Sprintf("%s: %q", ErrInvalidParse, e.Input)
	}
	return fmt.Sprintf("%s: %q suffix %q", ErrInvalidParse, e.Input, e.Suffix)
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (e ParseError) Unwrap() error {
	if e.Err == nil {
		return ErrInvalidParse
	}
	return e.Err
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e ParseError) Is(target error) bool {
	return target == ErrInvalidParse || errors.Is(e.Err, target)
}
