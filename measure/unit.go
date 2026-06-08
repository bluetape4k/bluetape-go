package measure

import (
	"fmt"
	"math"
	"strings"
)

// Unit  기준 단위 대비 ratio를 가진 불변 단위 값입니다.
type Unit[D any] struct {
	name   string
	suffix string
	ratio  float64
	space  bool
}

type unitConfig struct {
	space bool
}

// UnitOption  단위 생성 옵션입니다.
type UnitOption[D any] func(*unitConfig)

// WithSpaceBeforeSuffix  수치와 suffix 사이 공백 출력 여부를 지정합니다.
func WithSpaceBeforeSuffix[D any](space bool) UnitOption[D] {
	return func(config *unitConfig) {
		config.space = space
	}
}

// NewUnit  유효성 검사를 거쳐 새 Unit을 생성합니다.
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

// MustUnit  유효하지 않은 단위면 panic을 발생시킵니다.
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

// Name  단위 이름을 반환합니다.
func (u Unit[D]) Name() string {
	return u.name
}

// Suffix  단위 suffix를 반환합니다.
func (u Unit[D]) Suffix() string {
	return u.suffix
}

// Ratio  기준 단위 대비 배율을 반환합니다.
func (u Unit[D]) Ratio() float64 {
	return u.ratio
}

// SpaceBeforeSuffix  포맷 시 수치와 suffix 사이 공백 여부를 반환합니다.
func (u Unit[D]) SpaceBeforeSuffix() bool {
	return u.space
}

// IsValid  단위가 생성자 계약을 만족하는지 반환합니다.
func (u Unit[D]) IsValid() bool {
	return u.valid()
}

// String  단위 suffix를 반환합니다.
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
