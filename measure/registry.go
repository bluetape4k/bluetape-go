package measure

import (
	"fmt"
	"sort"
)

// Registry 패키지에서 공개하는 구조체다.
type Registry[D any] struct {
	units    []Unit[D]
	bySuffix map[string]Unit[D]
}

// NewRegistry Registry 인스턴스를 생성한다.
//
// 매개변수:
//   - units: NewRegistry에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MustRegistry 해당 차원의 단위 registry를 반환한다.
//
// 매개변수:
//   - units: MustRegistry에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// Units 값에 연결된 단위 정보를 반환한다.
func (r Registry[D]) Units() []Unit[D] {
	copied := make([]Unit[D], len(r.units))
	copy(copied, r.units)
	return copied
}

// Lookup registry에서 단위 이름을 조회한다.
//
// 매개변수:
//   - suffix: Lookup가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func (r Registry[D]) Lookup(suffix string) (Unit[D], bool) {
	unit, ok := r.bySuffix[suffix]
	return unit, ok
}
