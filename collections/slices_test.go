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

func TestCount(t *testing.T) {
	got := collections.Count([]string{"api", "job", "api", "worker", "job", "api"})
	want := map[string]int{"api": 3, "job": 2, "worker": 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Count returned %#v, want %#v", got, want)
	}
	if got := collections.Count[string](nil); got != nil {
		t.Fatalf("Count nil returned %#v, want nil", got)
	}
	if got := collections.Count([]string{}); got == nil || len(got) != 0 {
		t.Fatalf("Count empty returned %#v, want empty map", got)
	}
}

func TestPadTo(t *testing.T) {
	got, err := collections.PadTo([]int{1, 2}, 5, 0)
	if err != nil {
		t.Fatalf("PadTo returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 0, 0, 0}) {
		t.Fatalf("PadTo returned %#v", got)
	}
	got, err = collections.PadTo([]int{1, 2, 3}, 2, 0)
	if err != nil {
		t.Fatalf("PadTo shrink returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("PadTo shrink returned %#v", got)
	}
	got, err = collections.PadTo[int](nil, 3, 9)
	if err != nil {
		t.Fatalf("PadTo nil returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []int{9, 9, 9}) {
		t.Fatalf("PadTo nil returned %#v", got)
	}
	if _, err := collections.PadTo([]int{1}, -1, 0); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("PadTo negative size error = %v, want ErrInvalidArgument", err)
	}
}

func TestSafeSubslice(t *testing.T) {
	values := []int{1, 2, 3, 4, 5}
	if got := collections.SafeSubslice(values, -10, 100); !reflect.DeepEqual(got, values) {
		t.Fatalf("SafeSubslice wide returned %#v", got)
	}
	if got := collections.SafeSubslice(values, 1, 3); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("SafeSubslice middle returned %#v", got)
	}
	if got := collections.SafeSubslice(values, 3, 1); len(got) != 0 {
		t.Fatalf("SafeSubslice reversed returned %#v, want empty", got)
	}
	if got := collections.SafeSubslice[int](nil, 0, 3); got != nil {
		t.Fatalf("SafeSubslice nil returned %#v, want nil", got)
	}
	if got := collections.SafeSubslice([]int{}, -1, 3); got == nil || len(got) != 0 {
		t.Fatalf("SafeSubslice empty returned %#v, want empty slice", got)
	}
}

func TestZipWithIndex(t *testing.T) {
	got := collections.ZipWithIndex([]string{"a", "b"})
	want := []collections.Indexed[string]{
		{Index: 0, Value: "a"},
		{Index: 1, Value: "b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ZipWithIndex returned %#v, want %#v", got, want)
	}
	if got := collections.ZipWithIndex[string](nil); got != nil {
		t.Fatalf("ZipWithIndex nil returned %#v, want nil", got)
	}
	if got := collections.ZipWithIndex([]string{}); got == nil || len(got) != 0 {
		t.Fatalf("ZipWithIndex empty returned %#v, want empty slice", got)
	}
}

func TestSliding(t *testing.T) {
	got, err := collections.Sliding([]int{1, 2, 3, 4}, 3, true)
	if err != nil {
		t.Fatalf("Sliding returned error: %v", err)
	}
	want := [][]int{{1, 2, 3}, {2, 3, 4}, {3, 4}, {4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sliding returned %#v, want %#v", got, want)
	}
	got, err = collections.Sliding([]int{1, 2, 3, 4}, 3, false)
	if err != nil {
		t.Fatalf("Sliding full-only returned error: %v", err)
	}
	want = [][]int{{1, 2, 3}, {2, 3, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sliding full-only returned %#v, want %#v", got, want)
	}
	got, err = collections.Sliding[int](nil, 2, true)
	if err != nil {
		t.Fatalf("Sliding nil returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("Sliding nil returned %#v, want nil", got)
	}
	got, err = collections.Sliding([]int{}, 2, true)
	if err != nil {
		t.Fatalf("Sliding empty returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Sliding empty returned %#v, want empty slice", got)
	}
	if _, err := collections.Sliding([]int{1}, 0, true); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("Sliding invalid size error = %v, want ErrInvalidArgument", err)
	}
}

func TestForEachErr(t *testing.T) {
	var visited []int
	err := collections.ForEachErr([]int{1, 2, 3}, func(value int) error {
		visited = append(visited, value)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachErr returned error: %v", err)
	}
	if !reflect.DeepEqual(visited, []int{1, 2, 3}) {
		t.Fatalf("ForEachErr visited %#v", visited)
	}

	expected := errors.New("stop")
	visited = nil
	err = collections.ForEachErr([]int{1, 2, 3}, func(value int) error {
		visited = append(visited, value)
		if value == 2 {
			return expected
		}
		return nil
	})
	if !errors.Is(err, expected) {
		t.Fatalf("ForEachErr error = %v, want expected", err)
	}
	if !reflect.DeepEqual(visited, []int{1, 2}) {
		t.Fatalf("ForEachErr should stop at first error, visited %#v", visited)
	}
	if err := collections.ForEachErr[int](nil, func(int) error { return nil }); err != nil {
		t.Fatalf("ForEachErr nil returned error: %v", err)
	}
	if err := collections.ForEachErr([]int{}, func(int) error { return nil }); err != nil {
		t.Fatalf("ForEachErr empty returned error: %v", err)
	}
	if err := collections.ForEachErr([]int{1}, nil); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("ForEachErr nil action error = %v, want ErrInvalidArgument", err)
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
	if err := collections.ForEachErr[int](nil, nil); err == nil {
		t.Fatal("ForEachErr should reject nil action before nil input")
	}
	if err := collections.ForEachErr[int]([]int{}, nil); err == nil {
		t.Fatal("ForEachErr should reject nil action before empty input")
	}
}
