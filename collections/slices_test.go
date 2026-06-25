package collections_test

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func TestChunk(t *testing.T) {
	got, err := collections.Chunk([]int{1, 2, 3, 4, 5}, 2)
	if err != nil {
		t.Fatalf("Chunk returned error: %v", err)
	}
	want := [][]int{{1, 2}, {3, 4}, {5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Chunk returned %#v", got)
	}

	got, err = collections.Chunk[int](nil, 2)
	if err != nil {
		t.Fatalf("Chunk nil returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Chunk nil returned %#v", got)
	}

	got, err = collections.Chunk([]int{}, 2)
	if err != nil {
		t.Fatalf("Chunk empty returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Chunk empty returned %#v", got)
	}

	if _, err := collections.Chunk([]int{1}, 0); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("Chunk invalid size error = %v, want ErrInvalidArgument", err)
	}
}

func TestChunkLargeInput(t *testing.T) {
	values := make([]int, 10_001)
	for i := range values {
		values[i] = i
	}

	got, err := collections.Chunk(values, 1_000)
	if err != nil {
		t.Fatalf("Chunk returned error: %v", err)
	}
	if len(got) != 11 {
		t.Fatalf("chunk count = %d", len(got))
	}
	if len(got[10]) != 1 || got[10][0] != 10_000 {
		t.Fatalf("last chunk = %#v", got[10])
	}
}

func TestChunkBy(t *testing.T) {
	got, err := collections.ChunkBy([]int{1, 2, 3, 4, 5}, func(value int) bool {
		return value%3 == 0
	})
	if err != nil {
		t.Fatalf("ChunkBy returned error: %v", err)
	}
	want := [][]int{{1, 2}, {3, 4, 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChunkBy returned %#v", got)
	}
	if _, err := collections.ChunkBy([]int{1}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("ChunkBy nil predicate error = %v, want ErrInvalidArgument", err)
	}
}

func TestCollectionSliceHelpersWrapInvalidArgument(t *testing.T) {
	checks := []struct {
		name string
		err  error
	}{
		{name: "distinct by nil key", err: func() error { _, err := collections.DistinctBy[int, int]([]int{1}, nil); return err }()},
		{name: "map nil mapper", err: func() error { _, err := collections.MapErr[int, int]([]int{1}, nil); return err }()},
		{name: "filter nil predicate", err: func() error { _, err := collections.FilterErr([]int{1}, nil); return err }()},
		{name: "filter map nil mapper", err: func() error { _, err := collections.FilterMap[int, int]([]int{1}, nil); return err }()},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !errors.Is(check.err, collections.ErrInvalidArgument) {
				t.Fatalf("error = %v, want ErrInvalidArgument", check.err)
			}
		})
	}
}

func TestDistinct(t *testing.T) {
	got := collections.Distinct([]int{1, 2, 2, 3, 1})
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("Distinct returned %#v", got)
	}
	if got := collections.Distinct[int](nil); got != nil {
		t.Fatalf("Distinct nil returned %#v", got)
	}
	got = collections.Distinct([]int{})
	if got == nil || len(got) != 0 {
		t.Fatalf("Distinct empty returned %#v", got)
	}
}

func TestDistinctBy(t *testing.T) {
	type item struct {
		ID   int
		Name string
	}
	values := []item{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 1, Name: "c"}}
	got, err := collections.DistinctBy(values, func(value item) int {
		return value.ID
	})
	if err != nil {
		t.Fatalf("DistinctBy returned error: %v", err)
	}
	want := []item{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DistinctBy returned %#v", got)
	}
}

func TestMapErr(t *testing.T) {
	got, err := collections.MapErr([]int{1, 2, 3}, func(value int) (string, error) {
		return strconv.Itoa(value), nil
	})
	if err != nil {
		t.Fatalf("MapErr returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Fatalf("MapErr returned %#v", got)
	}

	expected := errors.New("stop")
	_, err = collections.MapErr([]int{1, 2}, func(value int) (int, error) {
		if value == 2 {
			return 0, expected
		}
		return value, nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("MapErr error = %v", err)
	}
}

func TestFilterErr(t *testing.T) {
	got, err := collections.FilterErr([]int{1, 2, 3, 4}, func(value int) (bool, error) {
		return value%2 == 0, nil
	})
	if err != nil {
		t.Fatalf("FilterErr returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{2, 4}) {
		t.Fatalf("FilterErr returned %#v", got)
	}
}

func TestFilterMap(t *testing.T) {
	got, err := collections.FilterMap([]string{"1", "x", "2"}, func(value string) (int, bool) {
		parsed, err := strconv.Atoi(value)
		return parsed, err == nil
	})
	if err != nil {
		t.Fatalf("FilterMap returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("FilterMap returned %#v", got)
	}
}

func TestCollectionSliceHelpersNilAndEmptyContracts(t *testing.T) {
	if got, err := collections.ChunkBy[int](nil, func(int) bool { return false }); err != nil || got != nil {
		t.Fatalf("ChunkBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.ChunkBy([]int{}, func(int) bool { return false }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("ChunkBy empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.DistinctBy[int, int](nil, func(value int) int { return value }); err != nil || got != nil {
		t.Fatalf("DistinctBy nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.DistinctBy([]int{}, func(value int) int { return value }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("DistinctBy empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.MapErr[int, int](nil, func(value int) (int, error) { return value, nil }); err != nil || got != nil {
		t.Fatalf("MapErr nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.MapErr([]int{}, func(value int) (int, error) { return value, nil }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("MapErr empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.FilterErr[int](nil, func(_ int) (bool, error) { return true, nil }); err != nil || got != nil {
		t.Fatalf("FilterErr nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.FilterErr([]int{}, func(_ int) (bool, error) { return true, nil }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FilterErr empty = %#v, %v; want empty slice, nil", got, err)
	}
	if got, err := collections.FilterMap[int, int](nil, func(value int) (int, bool) { return value, true }); err != nil || got != nil {
		t.Fatalf("FilterMap nil = %#v, %v; want nil, nil", got, err)
	}
	if got, err := collections.FilterMap([]int{}, func(value int) (int, bool) { return value, true }); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("FilterMap empty = %#v, %v; want empty slice, nil", got, err)
	}
}

func TestCollectionSliceHelpersRejectNilCallbacks(t *testing.T) {
	if _, err := collections.ChunkBy[int](nil, nil); err == nil {
		t.Fatal("ChunkBy should reject nil predicate before nil input")
	}
	if _, err := collections.ChunkBy([]int{}, nil); err == nil {
		t.Fatal("ChunkBy should reject nil predicate before empty input")
	}
	if _, err := collections.DistinctBy[int, int]([]int{1}, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function")
	}
	if _, err := collections.DistinctBy[int, int](nil, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function before nil input")
	}
	if _, err := collections.DistinctBy[int, int]([]int{}, nil); err == nil {
		t.Fatal("DistinctBy should reject nil key function before empty input")
	}
	if _, err := collections.MapErr[int, int]([]int{1}, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper")
	}
	if _, err := collections.MapErr[int, int](nil, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper before nil input")
	}
	if _, err := collections.MapErr[int, int]([]int{}, nil); err == nil {
		t.Fatal("MapErr should reject nil mapper before empty input")
	}
	if _, err := collections.FilterErr[int]([]int{1}, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate")
	}
	if _, err := collections.FilterErr[int](nil, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate before nil input")
	}
	if _, err := collections.FilterErr[int]([]int{}, nil); err == nil {
		t.Fatal("FilterErr should reject nil predicate before empty input")
	}
	if _, err := collections.FilterMap[int, int]([]int{1}, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper")
	}
	if _, err := collections.FilterMap[int, int](nil, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper before nil input")
	}
	if _, err := collections.FilterMap[int, int]([]int{}, nil); err == nil {
		t.Fatal("FilterMap should reject nil mapper before empty input")
	}
}
