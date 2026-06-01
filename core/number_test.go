package core_test

import (
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestClamp(t *testing.T) {
	got, err := core.Clamp(5, 1, 10)
	if err != nil {
		t.Fatalf("Clamp returned error: %v", err)
	}
	if got != 5 {
		t.Fatalf("Clamp returned %d", got)
	}
	got, err = core.Clamp(-1, 1, 10)
	if err != nil {
		t.Fatalf("Clamp returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("Clamp below range returned %d", got)
	}
	got, err = core.Clamp(11, 1, 10)
	if err != nil {
		t.Fatalf("Clamp returned error: %v", err)
	}
	if got != 10 {
		t.Fatalf("Clamp above range returned %d", got)
	}
	if _, err := core.Clamp(1, 10, 1); err == nil {
		t.Fatal("Clamp should reject invalid ranges")
	}
}

func TestHexHelpers(t *testing.T) {
	for _, r := range []rune{'0', '9', 'a', 'f', 'A', 'F'} {
		if !core.IsHexDigit(r) {
			t.Fatalf("%q should be a hex digit", r)
		}
	}
	for _, r := range []rune{'g', 'x', '-', ' '} {
		if core.IsHexDigit(r) {
			t.Fatalf("%q should not be a hex digit", r)
		}
	}

	for _, value := range []string{"0x1234", "0XABCD", "#cafe", "-0xff"} {
		if !core.IsHexFormat(value) {
			t.Fatalf("%q should be hex format", value)
		}
	}
	for _, value := range []string{"1234", "0x", "#", "0xxyz"} {
		if core.IsHexFormat(value) {
			t.Fatalf("%q should not be hex format", value)
		}
	}
}
