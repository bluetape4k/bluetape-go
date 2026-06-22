package collections_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func TestPageOf(t *testing.T) {
	items := []int{1, 2, 3}
	page, err := collections.PageOf(items, 1, 3, 8)
	if err != nil {
		t.Fatalf("PageOf returned error: %v", err)
	}
	items[0] = 100

	if page.PageNumber() != 1 || page.PageSize() != 3 || page.TotalItems() != 8 {
		t.Fatalf("page metadata = page %d size %d total %d", page.PageNumber(), page.PageSize(), page.TotalItems())
	}
	if page.TotalPages() != 3 {
		t.Fatalf("TotalPages() = %d, want 3", page.TotalPages())
	}
	if page.Offset() != 3 {
		t.Fatalf("Offset() = %d, want 3", page.Offset())
	}
	if !page.HasNext() || !page.HasPrevious() {
		t.Fatalf("next/previous = %v/%v, want true/true", page.HasNext(), page.HasPrevious())
	}

	got := page.Items()
	if got[0] != 1 {
		t.Fatalf("Items snapshot first = %d, want 1", got[0])
	}
	got[0] = 200
	if again := page.Items(); again[0] != 1 {
		t.Fatalf("Items should return a fresh snapshot, first = %d", again[0])
	}
}

func TestPageOfNilAndEmptyItems(t *testing.T) {
	nilPage, err := collections.PageOf[int](nil, 0, 10, 0)
	if err != nil {
		t.Fatalf("PageOf nil returned error: %v", err)
	}
	if nilPage.Items() != nil {
		t.Fatalf("nil page Items() = %#v, want nil", nilPage.Items())
	}

	emptyPage, err := collections.PageOf([]int{}, 0, 10, 0)
	if err != nil {
		t.Fatalf("PageOf empty returned error: %v", err)
	}
	if items := emptyPage.Items(); items == nil || len(items) != 0 {
		t.Fatalf("empty page Items() = %#v, want empty non-nil slice", items)
	}
	if emptyPage.TotalPages() != 0 || emptyPage.Offset() != 0 || emptyPage.HasNext() || emptyPage.HasPrevious() {
		t.Fatalf("empty page metadata totalPages=%d offset=%d next=%v previous=%v", emptyPage.TotalPages(), emptyPage.Offset(), emptyPage.HasNext(), emptyPage.HasPrevious())
	}
}

func TestPageOfRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		page  int
		size  int
		total int64
	}{
		{name: "negative page", page: -1, size: 10, total: 0},
		{name: "zero size", page: 0, size: 0, total: 0},
		{name: "negative size", page: 0, size: -1, total: 0},
		{name: "negative total", page: 0, size: 10, total: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := collections.PageOf([]int{1}, tt.page, tt.size, tt.total); err == nil {
				t.Fatal("PageOf should reject invalid input")
			}
		})
	}

	if strconv.IntSize == 64 {
		maxInt := int(^uint(0) >> 1)
		if _, err := collections.PageOf([]int{1}, maxInt, 2, math.MaxInt64); err == nil {
			t.Fatal("PageOf should reject offset overflow")
		}
	}
}

func TestPageTotalPagesAvoidsOverflow(t *testing.T) {
	page, err := collections.PageOf([]int{1}, 0, 2, math.MaxInt64)
	if err != nil {
		t.Fatalf("PageOf returned error: %v", err)
	}
	want := int64(math.MaxInt64/2 + 1)
	if page.TotalPages() != want {
		t.Fatalf("TotalPages() = %d, want %d", page.TotalPages(), want)
	}
}

func TestPageHasNextAvoidsOverflow(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("max int page overflow regression is only relevant on 64-bit int platforms")
	}

	maxInt := int(^uint(0) >> 1)
	page, err := collections.PageOf([]int{1}, maxInt, 1, math.MaxInt64)
	if err != nil {
		t.Fatalf("PageOf returned error: %v", err)
	}
	if page.HasNext() {
		t.Fatal("HasNext should be false on the last representable page")
	}
}
