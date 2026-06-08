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
	// ErrInvalidParse  측정값 문자열을 파싱할 수 없을 때 사용합니다.
	ErrInvalidParse = errors.New("measure: invalid parse")
)

// ParseError  파싱 실패 입력과 원인을 보존합니다.
type ParseError struct {
	Input  string
	Suffix string
	Err    error
}

// Error  파싱 실패 메시지를 반환합니다.
func (e ParseError) Error() string {
	if e.Suffix == "" {
		return fmt.Sprintf("%s: %q", ErrInvalidParse, e.Input)
	}
	return fmt.Sprintf("%s: %q suffix %q", ErrInvalidParse, e.Input, e.Suffix)
}

// Unwrap  errors.Is/errors.As가 원인 에러를 찾도록 합니다.
func (e ParseError) Unwrap() error {
	if e.Err == nil {
		return ErrInvalidParse
	}
	return e.Err
}

// Is  ParseError가 ErrInvalidParse와 원인 sentinel 모두에 match되도록 합니다.
func (e ParseError) Is(target error) bool {
	return target == ErrInvalidParse || errors.Is(e.Err, target)
}
