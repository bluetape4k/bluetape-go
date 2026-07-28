package collections

import (
	"fmt"
	"math"
)

// Page는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Page[T any] struct {
	items  []T
	page   int
	size   int
	total  int64
	offset int64
}

// PageOf는 PageOf 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - items: PageOf가 읽거나 복사하는 items 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - page: PageOf 동작에 필요한 page 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - size: PageOf 동작에 필요한 size 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - total: PageOf 동작에 필요한 total 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Items는 Items 공개 API의 동작을 수행한다.
func (p Page[T]) Items() []T {
	if p.items == nil {
		return nil
	}
	result := make([]T, len(p.items))
	copy(result, p.items)
	return result
}

// PageNumber는 PageNumber 공개 API의 동작을 수행한다.
func (p Page[T]) PageNumber() int {
	return p.page
}

// PageSize는 PageSize 공개 API의 동작을 수행한다.
func (p Page[T]) PageSize() int {
	return p.size
}

// TotalItems는 TotalItems 공개 API의 동작을 수행한다.
func (p Page[T]) TotalItems() int64 {
	return p.total
}

// TotalPages는 TotalPages 공개 API의 동작을 수행한다.
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

// Offset는 Offset 공개 API의 동작을 수행한다.
func (p Page[T]) Offset() int64 {
	return p.offset
}

// HasNext는 HasNext 공개 API의 동작을 수행한다.
func (p Page[T]) HasNext() bool {
	totalPages := p.TotalPages()
	return totalPages > 0 && int64(p.page) < totalPages-1
}

// HasPrevious는 HasPrevious 공개 API의 동작을 수행한다.
func (p Page[T]) HasPrevious() bool {
	return p.page > 0
}
