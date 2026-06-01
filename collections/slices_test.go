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

	if _, err := collections.Chunk([]int{1}, 0); err == nil {
		t.Fatal("Chunk should reject non-positive size")
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
	if _, err := collections.ChunkBy([]int{1}, nil); err == nil {
		t.Fatal("ChunkBy should reject nil predicate")
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
