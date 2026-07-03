package core_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestUUIDHelpers(t *testing.T) {
	const lower = "24738134-9d88-6645-4ec8-d63aa2031015"
	const upper = "24738134-9D88-6645-4EC8-D63AA2031015"

	if !core.IsUUID(lower) {
		t.Fatal("IsUUID should accept canonical lowercase UUID text")
	}
	if !core.IsUUID(upper) {
		t.Fatal("IsUUID should accept uppercase UUID text")
	}
	got, err := core.CanonicalUUID(upper)
	if err != nil {
		t.Fatalf("CanonicalUUID returned error: %v", err)
	}
	if got != lower {
		t.Fatalf("CanonicalUUID returned %q, want %q", got, lower)
	}
	if err := core.RequireUUID("id", upper); err != nil {
		t.Fatalf("RequireUUID returned error: %v", err)
	}
	if core.ZeroUUID != "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("ZeroUUID = %q", core.ZeroUUID)
	}
	if !core.IsZeroUUID(core.ZeroUUID) {
		t.Fatal("IsZeroUUID should accept ZeroUUID")
	}
	if core.IsZeroUUID(lower) {
		t.Fatal("IsZeroUUID should reject non-zero UUID")
	}
}

func TestUUIDHelpersRejectInvalidText(t *testing.T) {
	for _, value := range []string{
		"",
		"247381349d8866454ec8d63aa2031015",
		"24738134-9d88-6645-4ec8-d63aa2031015-43-ap",
		"24738134-9d88-6645-4ec8-d63aa203101z",
		" 24738134-9d88-6645-4ec8-d63aa2031015 ",
	} {
		if core.IsUUID(value) {
			t.Fatalf("IsUUID(%q) = true, want false", value)
		}
		if core.IsZeroUUID(value) {
			t.Fatalf("IsZeroUUID(%q) = true, want false", value)
		}
		if _, err := core.CanonicalUUID(value); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("CanonicalUUID(%q) error = %v, want ErrInvalidArgument", value, err)
		}
		if err := core.RequireUUID("id", value); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("RequireUUID(%q) error = %v, want ErrInvalidArgument", value, err)
		}
	}
}
