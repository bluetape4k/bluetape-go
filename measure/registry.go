package measure

import (
	"fmt"
	"sort"
)

// Registry struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Registry[D any] struct {
	units    []Unit[D]
	bySuffix map[string]Unit[D]
}

// NewRegistry NewRegistry 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - units: NewRegistry 동작에 필요한 units 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewRegistry[D any](units ...Unit[D]) (Registry[D], error) {
	if len(units) == 0 {
		return Registry[D]{}, fmt.Errorf("%w: at least one unit is required", ErrInvalidUnit)
	}

	copied := make([]Unit[D], 0, len(units))
	bySuffix := make(map[string]Unit[D], len(units))
	for _, unit := range units {
		if err := unit.validate(); err != nil {
			return Registry[D]{}, err
		}
		if _, ok := bySuffix[unit.suffix]; ok {
			return Registry[D]{}, fmt.Errorf("%w: duplicate suffix %q", ErrInvalidUnit, unit.suffix)
		}
		copied = append(copied, unit)
		bySuffix[unit.suffix] = unit
	}

	sort.Slice(copied, func(i, j int) bool {
		if len(copied[i].suffix) == len(copied[j].suffix) {
			return copied[i].suffix < copied[j].suffix
		}
		return len(copied[i].suffix) > len(copied[j].suffix)
	})
	return Registry[D]{units: copied, bySuffix: bySuffix}, nil
}

// MustRegistry MustRegistry 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - units: MustRegistry 동작에 필요한 units 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func MustRegistry[D any](units ...Unit[D]) Registry[D] {
	registry, err := NewRegistry(units...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (r Registry[D]) validate() error {
	if len(r.units) == 0 || len(r.bySuffix) == 0 {
		return fmt.Errorf("%w: registry is empty", ErrInvalidUnit)
	}
	return nil
}

// Units Units 공개 API의 동작을 수행한다.
func (r Registry[D]) Units() []Unit[D] {
	copied := make([]Unit[D], len(r.units))
	copy(copied, r.units)
	return copied
}

// Lookup Lookup 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - suffix: Lookup가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func (r Registry[D]) Lookup(suffix string) (Unit[D], bool) {
	unit, ok := r.bySuffix[suffix]
	return unit, ok
}
