package collections_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/collections"
)

func TestBoundedStack(t *testing.T) {
	stack, err := collections.NewBoundedStack[int](3)
	if err != nil {
		t.Fatalf("NewBoundedStack returned error: %v", err)
	}
	if stack.Capacity() != 3 || stack.Len() != 0 || !stack.Empty() {
		t.Fatalf("initial stack state = capacity %d len %d empty %v", stack.Capacity(), stack.Len(), stack.Empty())
	}

	stack.Push(1)
	stack.PushAll(2, 3, 4)
	if stack.Len() != 3 || stack.Empty() {
		t.Fatalf("stack state after push = len %d empty %v", stack.Len(), stack.Empty())
	}
	if got := stack.Values(); !reflect.DeepEqual(got, []int{4, 3, 2}) {
		t.Fatalf("Values() = %#v, want top-to-bottom [4 3 2]", got)
	}
	if got, ok := stack.Peek(); !ok || got != 4 {
		t.Fatalf("Peek() = %v,%v; want 4,true", got, ok)
	}
	if got, ok := stack.At(0); !ok || got != 4 {
		t.Fatalf("At(0) = %v,%v; want 4,true", got, ok)
	}
	if got, ok := stack.At(2); !ok || got != 2 {
		t.Fatalf("At(2) = %v,%v; want 2,true", got, ok)
	}
	if _, ok := stack.At(3); ok {
		t.Fatal("At out of range should return false")
	}

	got := stack.Values()
	got[0] = 100
	if peek, _ := stack.Peek(); peek != 4 {
		t.Fatalf("Values should return a snapshot, peek = %d", peek)
	}

	if got, ok := stack.Pop(); !ok || got != 4 {
		t.Fatalf("Pop() = %v,%v; want 4,true", got, ok)
	}
	stack.Clear()
	if stack.Len() != 0 || !stack.Empty() {
		t.Fatalf("Clear state = len %d empty %v", stack.Len(), stack.Empty())
	}
	if _, ok := stack.Pop(); ok {
		t.Fatal("Pop empty should return false")
	}
	if _, ok := stack.Peek(); ok {
		t.Fatal("Peek empty should return false")
	}
}

func TestBoundedStackRejectsInvalidCapacity(t *testing.T) {
	if _, err := collections.NewBoundedStack[int](0); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("NewBoundedStack zero capacity error = %v, want ErrInvalidArgument", err)
	}
	if _, err := collections.NewBoundedStack[int](-1); !errors.Is(err, collections.ErrInvalidArgument) {
		t.Fatalf("NewBoundedStack negative capacity error = %v, want ErrInvalidArgument", err)
	}
}

func TestBoundedStackPreservesNilSliceValues(t *testing.T) {
	stack, err := collections.NewBoundedStack[[]int](2)
	if err != nil {
		t.Fatalf("NewBoundedStack returned error: %v", err)
	}
	stack.Push(nil)
	stack.Push([]int{})

	values := stack.Values()
	if values[0] == nil || len(values[0]) != 0 {
		t.Fatalf("top value = %#v, want empty non-nil slice", values[0])
	}
	if values[1] != nil {
		t.Fatalf("bottom value = %#v, want nil slice", values[1])
	}
}
