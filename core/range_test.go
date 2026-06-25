package core_test

import (
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestRangeConstructorsAndContains(t *testing.T) {
	tests := []struct {
		name          string
		build         func() (core.Range[int], error)
		containsLower bool
		containsUpper bool
		text          string
	}{
		{
			name:          "closed",
			build:         func() (core.Range[int], error) { return core.ClosedRange(1, 3) },
			containsLower: true,
			containsUpper: true,
			text:          "[1,3]",
		},
		{
			name:          "closed open",
			build:         func() (core.Range[int], error) { return core.ClosedOpenRange(1, 3) },
			containsLower: true,
			containsUpper: false,
			text:          "[1,3)",
		},
		{
			name:          "open closed",
			build:         func() (core.Range[int], error) { return core.OpenClosedRange(1, 3) },
			containsLower: false,
			containsUpper: true,
			text:          "(1,3]",
		},
		{
			name:          "open open",
			build:         func() (core.Range[int], error) { return core.OpenOpenRange(1, 3) },
			containsLower: false,
			containsUpper: false,
			text:          "(1,3)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := tt.build()
			if err != nil {
				t.Fatalf("constructor returned error: %v", err)
			}
			if r.Lower() != 1 || r.Upper() != 3 {
				t.Fatalf("bounds = %v,%v; want 1,3", r.Lower(), r.Upper())
			}
			if r.Contains(1) != tt.containsLower {
				t.Fatalf("Contains(lower) = %v, want %v", r.Contains(1), tt.containsLower)
			}
			if !r.Contains(2) {
				t.Fatal("Contains(mid) = false, want true")
			}
			if r.Contains(3) != tt.containsUpper {
				t.Fatalf("Contains(upper) = %v, want %v", r.Contains(3), tt.containsUpper)
			}
			if got := r.String(); got != tt.text {
				t.Fatalf("String() = %q, want %q", got, tt.text)
			}
		})
	}
}

func TestRangeRejectsInvalidBounds(t *testing.T) {
	if _, err := core.ClosedRange(3, 1); err == nil {
		t.Fatal("ClosedRange should reject reversed bounds")
	}
	if _, err := core.ClosedOpenRange(1, 1); err == nil {
		t.Fatal("ClosedOpenRange should reject equal bounds")
	}
	if _, err := core.OpenClosedRange(1, 1); err == nil {
		t.Fatal("OpenClosedRange should reject equal bounds")
	}
	if _, err := core.OpenOpenRange(1, 1); err == nil {
		t.Fatal("OpenOpenRange should reject equal bounds")
	}
	if _, err := core.ClosedRange(math.NaN(), 1); err == nil {
		t.Fatal("ClosedRange should reject NaN lower bound")
	}
	if _, err := core.ClosedRange(float32(1), float32(math.NaN())); err == nil {
		t.Fatal("ClosedRange should reject NaN upper bound")
	}
}

func TestRangeContainsRejectsNaNValue(t *testing.T) {
	r, err := core.ClosedRange(1.0, 3.0)
	if err != nil {
		t.Fatalf("ClosedRange returned error: %v", err)
	}
	if r.Contains(math.NaN()) {
		t.Fatal("Contains(NaN) should be false")
	}
}

func TestRangeContainsRangeAndOverlaps(t *testing.T) {
	closed, err := core.ClosedRange(1, 5)
	if err != nil {
		t.Fatalf("ClosedRange returned error: %v", err)
	}
	inner, err := core.OpenClosedRange(1, 4)
	if err != nil {
		t.Fatalf("OpenClosedRange returned error: %v", err)
	}
	rightTouchClosed, err := core.ClosedOpenRange(5, 8)
	if err != nil {
		t.Fatalf("ClosedOpenRange returned error: %v", err)
	}
	rightTouchOpen, err := core.OpenOpenRange(5, 8)
	if err != nil {
		t.Fatalf("OpenOpenRange returned error: %v", err)
	}
	leftOpen, err := core.OpenClosedRange(0, 1)
	if err != nil {
		t.Fatalf("OpenClosedRange returned error: %v", err)
	}

	if !closed.ContainsRange(inner) {
		t.Fatal("closed should contain inner")
	}
	if closed.ContainsRange(rightTouchClosed) {
		t.Fatal("closed should not contain right-touching range")
	}
	if !closed.Overlaps(rightTouchClosed) {
		t.Fatal("closed should overlap range that includes shared endpoint")
	}
	if closed.Overlaps(rightTouchOpen) {
		t.Fatal("closed should not overlap range that excludes shared endpoint")
	}
	if !closed.Overlaps(leftOpen) {
		t.Fatal("closed should overlap left range that includes shared endpoint")
	}
}

func TestRangeZeroValueIsEmpty(t *testing.T) {
	var r core.Range[int]
	if !r.Empty() {
		t.Fatal("zero-value Range should be empty")
	}
	if r.Contains(0) {
		t.Fatal("empty range should not contain values")
	}

	other, err := core.ClosedRange(0, 1)
	if err != nil {
		t.Fatalf("ClosedRange returned error: %v", err)
	}
	if r.Overlaps(other) || other.Overlaps(r) {
		t.Fatal("empty range should not overlap")
	}
	if r.ContainsRange(other) || other.ContainsRange(r) {
		t.Fatal("ContainsRange should return false when either side is empty")
	}
}
