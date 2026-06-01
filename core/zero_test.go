package core_test

import (
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestZeroHelpers(t *testing.T) {
	if got := core.Zero[int](); got != 0 {
		t.Fatalf("Zero returned %d", got)
	}
	if !core.IsZero("") {
		t.Fatal("IsZero should report empty string as zero")
	}
	if got := core.DefaultIfZero("", "fallback"); got != "fallback" {
		t.Fatalf("DefaultIfZero returned %q", got)
	}
	if got := core.DefaultIfZero("value", "fallback"); got != "value" {
		t.Fatalf("DefaultIfZero returned %q", got)
	}
	if got := core.FirstNonZero("", "", "blue", "green"); got != "blue" {
		t.Fatalf("FirstNonZero returned %q", got)
	}
}
