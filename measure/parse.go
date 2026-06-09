package measure

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse  registry suffix를 사용해 측정값 문자열을 파싱합니다.
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
