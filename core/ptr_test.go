package core_test

import (
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestPtrAndValueOr(t *testing.T) {
	ptr := core.Ptr("blue")
	if ptr == nil || *ptr != "blue" {
		t.Fatalf("Ptr returned %v", ptr)
	}
	if got := core.ValueOr(ptr, "fallback"); got != "blue" {
		t.Fatalf("ValueOr returned %q", got)
	}
	if got := core.ValueOr[string](nil, "fallback"); got != "fallback" {
		t.Fatalf("ValueOr nil returned %q", got)
	}
}

func TestValueOrZero(t *testing.T) {
	if got := core.ValueOrZero[int](nil); got != 0 {
		t.Fatalf("ValueOrZero nil returned %d", got)
	}
	if got := core.ValueOrZero(core.Ptr(42)); got != 42 {
		t.Fatalf("ValueOrZero returned %d", got)
	}
}
