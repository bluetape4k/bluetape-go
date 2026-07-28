package measure

import (
	"fmt"
	"math"
	"strings"
)

// Unit는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Unit[D any] struct {
	name   string
	suffix string
	ratio  float64
	space  bool
}

type unitConfig struct {
	space bool
}

// UnitOption는 func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type UnitOption[D any] func(*unitConfig)

// WithSpaceBeforeSuffix는 WithSpaceBeforeSuffix 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - space: WithSpaceBeforeSuffix 동작에 필요한 space 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func WithSpaceBeforeSuffix[D any](space bool) UnitOption[D] {
	return func(config *unitConfig) {
		config.space = space
	}
}

// NewUnit는 NewUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: NewUnit가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - suffix: NewUnit가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - ratio: NewUnit 동작에 필요한 ratio 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: NewUnit 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewUnit[D any](name, suffix string, ratio float64, options ...UnitOption[D]) (Unit[D], error) {
	config := unitConfig{space: true}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}

	unit := Unit[D]{
		name:   strings.TrimSpace(name),
		suffix: strings.TrimSpace(suffix),
		ratio:  ratio,
		space:  config.space,
	}
	if err := unit.validate(); err != nil {
		return Unit[D]{}, err
	}
	return unit, nil
}

// MustUnit는 MustUnit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - name: MustUnit가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - suffix: MustUnit가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - ratio: MustUnit 동작에 필요한 ratio 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: MustUnit 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustUnit[D any](name, suffix string, ratio float64, options ...UnitOption[D]) Unit[D] {
	unit, err := NewUnit[D](name, suffix, ratio, options...)
	if err != nil {
		panic(err)
	}
	return unit
}

func (u Unit[D]) validate() error {
	switch {
	case u.name == "":
		return fmt.Errorf("%w: name must not be blank", ErrInvalidUnit)
	case u.suffix == "":
		return fmt.Errorf("%w: suffix must not be blank", ErrInvalidUnit)
	case !finite(u.ratio):
		return fmt.Errorf("%w: ratio must be finite", ErrInvalidUnit)
	case u.ratio <= 0:
		return fmt.Errorf("%w: ratio must be positive", ErrInvalidUnit)
	default:
		return nil
	}
}

func (u Unit[D]) valid() bool {
	return u.validate() == nil
}

// Name는 Name 공개 API의 동작을 수행한다.
func (u Unit[D]) Name() string {
	return u.name
}

// Suffix는 Suffix 공개 API의 동작을 수행한다.
func (u Unit[D]) Suffix() string {
	return u.suffix
}

// Ratio는 Ratio 공개 API의 동작을 수행한다.
func (u Unit[D]) Ratio() float64 {
	return u.ratio
}

// SpaceBeforeSuffix는 SpaceBeforeSuffix 공개 API의 동작을 수행한다.
func (u Unit[D]) SpaceBeforeSuffix() bool {
	return u.space
}

// IsValid는 IsValid 공개 API의 동작을 수행한다.
func (u Unit[D]) IsValid() bool {
	return u.valid()
}

// String는 String 공개 API의 동작을 수행한다.
func (u Unit[D]) String() string {
	if !u.valid() {
		return "<invalid unit>"
	}
	return u.suffix
}

func (u Unit[D]) formatSuffix() string {
	if u.space {
		return " " + u.suffix
	}
	return u.suffix
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
