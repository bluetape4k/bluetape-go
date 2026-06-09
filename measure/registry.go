package measure

import (
	"fmt"
	"sort"
)

// Registry  suffix 기반 측정값 파싱에 사용할 불변 단위 집합입니다.
type Registry[D any] struct {
	units    []Unit[D]
	bySuffix map[string]Unit[D]
}

// NewRegistry  단위 목록을 복사해 suffix lookup registry를 생성합니다.
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

// MustRegistry  registry 생성 실패 시 panic을 발생시킵니다.
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

// Units  registry 단위 목록의 복사본을 반환합니다.
func (r Registry[D]) Units() []Unit[D] {
	copied := make([]Unit[D], len(r.units))
	copy(copied, r.units)
	return copied
}

// Lookup  suffix와 일치하는 단위를 반환합니다.
func (r Registry[D]) Lookup(suffix string) (Unit[D], bool) {
	unit, ok := r.bySuffix[suffix]
	return unit, ok
}
