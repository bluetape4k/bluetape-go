package collections

import (
	"fmt"
	"math"
)

// Page 패키지에서 공개하는 구조체다.
type Page[T any] struct {
	items  []T
	page   int
	size   int
	total  int64
	offset int64
}

// PageOf 항목 목록과 page metadata로 Page를 만든다.
//
// 매개변수:
//   - items: page에 담을 항목 목록이다. nil과 빈 슬라이스는 빈 목록으로 다룬다.
//   - page: PageOf에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - size: PageOf에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - total: PageOf에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func PageOf[T any](items []T, page, size int, total int64) (Page[T], error) {
	var zero Page[T]
	if page < 0 {
		return zero, fmt.Errorf("%w: page[%d] must be non-negative", ErrInvalidArgument, page)
	}
	if size <= 0 {
		return zero, fmt.Errorf("%w: page size[%d] must be positive", ErrInvalidArgument, size)
	}
	if total < 0 {
		return zero, fmt.Errorf("%w: total[%d] must be non-negative", ErrInvalidArgument, total)
	}

	page64 := int64(page)
	size64 := int64(size)
	if page64 > math.MaxInt64/size64 {
		return zero, fmt.Errorf("%w: page offset overflows int64", ErrInvalidArgument)
	}

	var snapshot []T
	if items != nil {
		snapshot = make([]T, len(items))
		copy(snapshot, items)
	}
	return Page[T]{
		items:  snapshot,
		page:   page,
		size:   size,
		total:  total,
		offset: page64 * size64,
	}, nil
}

// Items 현재 값을 슬라이스로 반환한다.
func (p Page[T]) Items() []T {
	if p.items == nil {
		return nil
	}
	result := make([]T, len(p.items))
	copy(result, p.items)
	return result
}

// PageNumber 현재 page 번호를 반환한다.
func (p Page[T]) PageNumber() int {
	return p.page
}

// PageSize page 크기를 반환한다.
func (p Page[T]) PageSize() int {
	return p.size
}

// TotalItems 전체 항목 수를 반환한다.
func (p Page[T]) TotalItems() int64 {
	return p.total
}

// TotalPages 전체 page 수를 반환한다.
func (p Page[T]) TotalPages() int64 {
	if p.total <= 0 || p.size <= 0 {
		return 0
	}
	size := int64(p.size)
	pages := p.total / size
	if p.total%size != 0 {
		pages++
	}
	return pages
}

// Offset 현재 page의 시작 offset을 반환한다.
func (p Page[T]) Offset() int64 {
	return p.offset
}

// HasNext 해당 상태가 존재하는지 반환한다.
func (p Page[T]) HasNext() bool {
	totalPages := p.TotalPages()
	return totalPages > 0 && int64(p.page) < totalPages-1
}

// HasPrevious 해당 상태가 존재하는지 반환한다.
func (p Page[T]) HasPrevious() bool {
	return p.page > 0
}
