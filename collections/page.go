package collections

import (
	"fmt"
	"math"
)

// Page contains one page of items plus 0-based pagination metadata.
type Page[T any] struct {
	items  []T
	page   int
	size   int
	total  int64
	offset int64
}

// PageOf creates a page value after validating metadata and snapshotting items.
func PageOf[T any](items []T, page, size int, total int64) (Page[T], error) {
	var zero Page[T]
	if page < 0 {
		return zero, fmt.Errorf("page[%d] must be non-negative", page)
	}
	if size <= 0 {
		return zero, fmt.Errorf("page size[%d] must be positive", size)
	}
	if total < 0 {
		return zero, fmt.Errorf("total[%d] must be non-negative", total)
	}

	page64 := int64(page)
	size64 := int64(size)
	if page64 > math.MaxInt64/size64 {
		return zero, fmt.Errorf("page offset overflows int64")
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

// Items returns a shallow snapshot of the page items.
func (p Page[T]) Items() []T {
	if p.items == nil {
		return nil
	}
	result := make([]T, len(p.items))
	copy(result, p.items)
	return result
}

// PageNumber returns the 0-based page number.
func (p Page[T]) PageNumber() int {
	return p.page
}

// PageSize returns the requested page size.
func (p Page[T]) PageSize() int {
	return p.size
}

// TotalItems returns the total item count.
func (p Page[T]) TotalItems() int64 {
	return p.total
}

// TotalPages returns the total number of pages.
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

// Offset returns the 0-based item offset for this page.
func (p Page[T]) Offset() int64 {
	return p.offset
}

// HasNext reports whether another page exists after this page.
func (p Page[T]) HasNext() bool {
	totalPages := p.TotalPages()
	return totalPages > 0 && int64(p.page) < totalPages-1
}

// HasPrevious reports whether a previous page exists before this page.
func (p Page[T]) HasPrevious() bool {
	return p.page > 0
}
