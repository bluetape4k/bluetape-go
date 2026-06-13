package bloomhash

import "testing"

func TestIndexesAreDeterministic(t *testing.T) {
	t.Parallel()

	first := Indexes([]byte("alpha"), 7, 1024)
	second := Indexes([]byte("alpha"), 7, 1024)

	if len(first) != 7 {
		t.Fatalf("expected 7 indexes, got %d", len(first))
	}
	for i, index := range first {
		if index >= 1024 {
			t.Fatalf("index %d out of range: %d", i, index)
		}
		if index != second[i] {
			t.Fatalf("index %d is not deterministic: %d != %d", i, index, second[i])
		}
	}
}

func TestIndexesHandleZeroSecondHashFallback(t *testing.T) {
	t.Parallel()

	indexes := Indexes([]byte{}, 3, 64)

	if len(indexes) != 3 {
		t.Fatalf("expected 3 indexes, got %d", len(indexes))
	}
	for i, index := range indexes {
		if index >= 64 {
			t.Fatalf("index %d out of range: %d", i, index)
		}
	}
}
