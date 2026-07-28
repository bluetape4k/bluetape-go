package measure

import (
	"fmt"
	"math"
	"strings"
)

// Unit 패키지에서 공개하는 구조체다.
type Unit[D any] struct {
	name   string
	suffix string
	ratio  float64
	space  bool
}

type unitConfig struct {
	space bool
}

// UnitOption func 공개 타입이다.
type UnitOption[D any] func(*unitConfig)

// WithSpaceBeforeSuffix SpaceBeforeSuffix 설정을 적용한 옵션을 반환한다.
//
// 매개변수:
//   - space: WithSpaceBeforeSuffix에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func WithSpaceBeforeSuffix[D any](space bool) UnitOption[D] {
	return func(config *unitConfig) {
		config.space = space
	}
}

// NewUnit Unit 인스턴스를 생성한다.
//
// 매개변수:
//   - name: NewUnit가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - suffix: NewUnit가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - ratio: NewUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MustUnit 단위 조회에 실패하면 panic한다.
//
// 매개변수:
//   - name: MustUnit가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - suffix: MustUnit가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - ratio: MustUnit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
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

// Name 식별자 이름을 반환한다.
func (u Unit[D]) Name() string {
	return u.name
}

// Suffix 값에 사용할 suffix 문자열을 반환한다.
func (u Unit[D]) Suffix() string {
	return u.suffix
}

// Ratio 두 측정값의 비율을 반환한다.
func (u Unit[D]) Ratio() float64 {
	return u.ratio
}

// SpaceBeforeSuffix suffix 앞 공백 사용 여부를 반환한다.
func (u Unit[D]) SpaceBeforeSuffix() bool {
	return u.space
}

// IsValid 값이 조건을 만족하는지 반환한다.
func (u Unit[D]) IsValid() bool {
	return u.valid()
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
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
