package collections_test

import (
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func TestGroupBy(t *testing.T) {
	got, err := collections.GroupBy([]string{"a", "bb", "c"}, func(value string) int {
		return len(value)
	})
	if err != nil {
		t.Fatalf("GroupBy returned error: %v", err)
	}
	want := map[int][]string{
		1: {"a", "c"},
		2: {"bb"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GroupBy returned %#v", got)
	}

	got, err = collections.GroupBy[string, int](nil, func(value string) int {
		return len(value)
	})
	if err != nil {
		t.Fatalf("GroupBy nil returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("GroupBy nil returned %#v", got)
	}

	got, err = collections.GroupBy([]string{}, func(value string) int {
		return len(value)
	})
	if err != nil {
		t.Fatalf("GroupBy empty returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("GroupBy empty returned %#v", got)
	}
}

func TestCountBy(t *testing.T) {
	got, err := collections.CountBy([]string{"a", "bb", "c"}, func(value string) int {
		return len(value)
	})
	if err != nil {
		t.Fatalf("CountBy returned error: %v", err)
	}
	want := map[int]int{1: 2, 2: 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CountBy returned %#v", got)
	}
	if _, err := collections.CountBy[string, int]([]string{"a"}, nil); err == nil {
		t.Fatal("CountBy should reject nil key function")
	}
}

func TestMapHelpersNilEmptyAndCallbackContracts(t *testing.T) {
	if _, err := collections.GroupBy[string, int]([]string{"a"}, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function")
	}
	if _, err := collections.GroupBy[string, int](nil, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function before nil input")
	}
	if _, err := collections.GroupBy[string, int]([]string{}, nil); err == nil {
		t.Fatal("GroupBy should reject nil key function before empty input")
	}
	if got, err := collections.CountBy[string, int](nil, func(value string) int { return len(value) }); err != nil || got != nil {
		t.Fatalf("CountBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.CountBy([]string{}, func(value string) int { return len(value) }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("CountBy empty = %#v, %v; want empty map, nil", got, err)
	}
	if _, err := collections.CountBy[string, int](nil, nil); err == nil {
		t.Fatal("CountBy should reject nil key function before nil input")
	}
	if _, err := collections.CountBy[string, int]([]string{}, nil); err == nil {
		t.Fatal("CountBy should reject nil key function before empty input")
	}
}
