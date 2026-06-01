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
