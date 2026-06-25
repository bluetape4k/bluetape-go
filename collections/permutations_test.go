package collections_test

import (
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func collectPermutations[T any](seq func(func([]T) bool)) [][]T {
	var got [][]T
	seq(func(value []T) bool {
		got = append(got, value)
		return true
	})
	return got
}

func TestPermutations(t *testing.T) {
	got := collectPermutations(collections.Permutations([]int{1, 2, 3}))
	want := [][]int{
		{1, 2, 3},
		{1, 3, 2},
		{2, 1, 3},
		{2, 3, 1},
		{3, 2, 1},
		{3, 1, 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Permutations = %#v, want %#v", got, want)
	}
}

func TestPermutationsEmptyInputs(t *testing.T) {
	nilPermutations := collectPermutations(collections.Permutations[int](nil))
	if len(nilPermutations) != 1 || len(nilPermutations[0]) != 0 {
		t.Fatalf("nil permutations = %#v, want one empty permutation", nilPermutations)
	}

	emptyPermutations := collectPermutations(collections.Permutations([]int{}))
	if len(emptyPermutations) != 1 || len(emptyPermutations[0]) != 0 || emptyPermutations[0] == nil {
		t.Fatalf("empty permutations = %#v, want one empty non-nil permutation", emptyPermutations)
	}
}

func TestPermutationsDuplicateValuesArePositional(t *testing.T) {
	got := collectPermutations(collections.Permutations([]int{1, 1, 2}))
	if len(got) != 6 {
		t.Fatalf("duplicate input permutation count = %d, want 6", len(got))
	}
}

func TestPermutationsEarlyStopAndSnapshots(t *testing.T) {
	values := []int{1, 2, 3}
	seq := collections.Permutations(values)
	values[0] = 99

	var got [][]int
	yieldCount := 0
	seq(func(value []int) bool {
		yieldCount++
		got = append(got, append([]int(nil), value...))
		value[0] = 100
		return false
	})

	if yieldCount != 1 {
		t.Fatalf("yield count = %d, want 1", yieldCount)
	}
	if !reflect.DeepEqual(got, [][]int{{1, 2, 3}}) {
		t.Fatalf("first permutation = %#v, want original snapshot", got)
	}
}
