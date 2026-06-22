package collections_test

import (
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func TestRingBuffer(t *testing.T) {
	ring, err := collections.NewRingBuffer[int](3)
	if err != nil {
		t.Fatalf("NewRingBuffer returned error: %v", err)
	}
	if ring.Capacity() != 3 || ring.Len() != 0 || !ring.Empty() {
		t.Fatalf("initial ring state = capacity %d len %d empty %v", ring.Capacity(), ring.Len(), ring.Empty())
	}

	ring.Add(1)
	ring.AddAll(2, 3, 4)
	if ring.Len() != 3 || ring.Empty() {
		t.Fatalf("ring state after add = len %d empty %v", ring.Len(), ring.Empty())
	}
	if got := ring.Values(); !reflect.DeepEqual(got, []int{2, 3, 4}) {
		t.Fatalf("Values() = %#v, want oldest-to-newest [2 3 4]", got)
	}
	if got, ok := ring.At(0); !ok || got != 2 {
		t.Fatalf("At(0) = %v,%v; want 2,true", got, ok)
	}
	if got, ok := ring.At(2); !ok || got != 4 {
		t.Fatalf("At(2) = %v,%v; want 4,true", got, ok)
	}
	if _, ok := ring.At(3); ok {
		t.Fatal("At out of range should return false")
	}

	got := ring.Values()
	got[0] = 100
	if first, _ := ring.At(0); first != 2 {
		t.Fatalf("Values should return a snapshot, first = %d", first)
	}

	if err := ring.Drop(0); err != nil {
		t.Fatalf("Drop(0) returned error: %v", err)
	}
	if err := ring.Drop(-1); err == nil {
		t.Fatal("Drop(-1) should reject negative n")
	}
	if err := ring.Drop(2); err != nil {
		t.Fatalf("Drop(2) returned error: %v", err)
	}
	if got := ring.Values(); !reflect.DeepEqual(got, []int{4}) {
		t.Fatalf("Values after Drop(2) = %#v, want [4]", got)
	}
	if err := ring.Drop(10); err != nil {
		t.Fatalf("Drop clearing returned error: %v", err)
	}
	if ring.Len() != 0 || !ring.Empty() {
		t.Fatalf("ring after clear drop = len %d empty %v", ring.Len(), ring.Empty())
	}
	ring.AddAll(5, 6)
	ring.Clear()
	if ring.Len() != 0 || !ring.Empty() {
		t.Fatalf("Clear state = len %d empty %v", ring.Len(), ring.Empty())
	}
}

func TestRingBufferRejectsInvalidCapacity(t *testing.T) {
	if _, err := collections.NewRingBuffer[int](0); err == nil {
		t.Fatal("NewRingBuffer should reject zero capacity")
	}
	if _, err := collections.NewRingBuffer[int](-1); err == nil {
		t.Fatal("NewRingBuffer should reject negative capacity")
	}
}
