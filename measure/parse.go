package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - text: Parse가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - registry: Parse에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Parse[D any](text string, registry Registry[D]) (Measure[D], error) {
	if err := registry.validate(); err != nil {
		return Measure[D]{}, ParseError{Input: text, Err: err}
	}

	input := strings.TrimSpace(text)
	if input == "" {
		return Measure[D]{}, ParseError{Input: text, Err: ErrInvalidParse}
	}

	for _, unit := range registry.units {
		if !strings.HasSuffix(input, unit.suffix) {
			continue
		}
		numberPart := strings.TrimSpace(strings.TrimSuffix(input, unit.suffix))
		if numberPart == "" {
			return Measure[D]{}, ParseError{Input: text, Suffix: unit.suffix, Err: ErrInvalidParse}
		}
		value, err := strconv.ParseFloat(numberPart, 64)
		if err != nil || !finite(value) {
			return Measure[D]{}, ParseError{Input: text, Suffix: unit.suffix, Err: fmt.Errorf("%w: number", ErrInvalidParse)}
		}
		measure, err := New(value, unit)
		if err != nil {
			return Measure[D]{}, ParseError{Input: text, Suffix: unit.suffix, Err: err}
		}
		return measure, nil
	}

	return Measure[D]{}, ParseError{Input: text, Err: fmt.Errorf("%w: unknown suffix", ErrInvalidUnit)}
}
