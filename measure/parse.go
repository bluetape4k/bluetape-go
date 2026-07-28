package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse는 Parse 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - text: Parse가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - registry: Parse 동작에 필요한 registry 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
