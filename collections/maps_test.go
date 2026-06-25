package collections_test

import (
	"errors"
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
	if _, err := collections.CountBy[string, int]([]string{"a"}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("CountBy nil key error = %v, want ErrInvalidArgument", err)
	}
}

func TestMapHelpersNilEmptyAndCallbackContracts(t *testing.T) {
	if _, err := collections.GroupBy[string, int]([]string{"a"}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("GroupBy nil key error = %v, want ErrInvalidArgument", err)
	}
	if _, err := collections.GroupBy[string, int](nil, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("GroupBy nil input/key error = %v, want ErrInvalidArgument", err)
	}
	if _, err := collections.GroupBy[string, int]([]string{}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("GroupBy empty input nil key error = %v, want ErrInvalidArgument", err)
	}
	if got, err := collections.CountBy[string, int](nil, func(value string) int { return len(value) }); err != nil || got != nil {
		t.Fatalf("CountBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.CountBy([]string{}, func(value string) int { return len(value) }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("CountBy empty = %#v, %v; want empty map, nil", got, err)
	}
	if _, err := collections.CountBy[string, int](nil, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("CountBy nil input/key error = %v, want ErrInvalidArgument", err)
	}
	if _, err := collections.CountBy[string, int]([]string{}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("CountBy empty input nil key error = %v, want ErrInvalidArgument", err)
	}
}
