package core_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/core"
)

func TestRequireNotBlank(t *testing.T) {
	if err := core.RequireNotBlank("name", "blue"); err != nil {
		t.Fatalf("RequireNotBlank returned error: %v", err)
	}
	if err := core.RequireNotBlank("name", " \t "); err == nil {
		t.Fatal("RequireNotBlank should reject blank text")
	}
}

func TestRequireNotEmpty(t *testing.T) {
	if err := core.RequireNotEmpty("name", " "); err != nil {
		t.Fatalf("RequireNotEmpty should allow whitespace: %v", err)
	}
	if err := core.RequireNotEmpty("name", ""); err == nil {
		t.Fatal("RequireNotEmpty should reject empty text")
	}
}

func TestRequireRanges(t *testing.T) {
	if err := core.RequireInRange("value", 5, 1, 5); err != nil {
		t.Fatalf("RequireInRange returned error: %v", err)
	}
	if err := core.RequireInRange("value", 6, 1, 5); err == nil {
		t.Fatal("RequireInRange should reject values above max")
	}
	if err := core.RequireInOpenRange("value", 5, 1, 5); err == nil {
		t.Fatal("RequireInOpenRange should reject end-exclusive value")
	}
	if err := core.RequireInOpenRange("value", 3, 1, 5); err != nil {
		t.Fatalf("RequireInOpenRange returned error: %v", err)
	}
}

func TestRequireNumbers(t *testing.T) {
	if err := core.RequirePositive("count", 1); err != nil {
		t.Fatalf("RequirePositive returned error: %v", err)
	}
	if err := core.RequirePositive("count", 0); err == nil {
		t.Fatal("RequirePositive should reject zero")
	}
	if err := core.RequireNonNegative("count", 0); err != nil {
		t.Fatalf("RequireNonNegative returned error: %v", err)
	}
	if err := core.RequireNonNegative("count", -1); err == nil {
		t.Fatal("RequireNonNegative should reject negative values")
	}
}

func TestValidationErrorsArePlainErrors(t *testing.T) {
	err := core.RequireNotBlank("name", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if errors.Unwrap(err) != nil {
		t.Fatalf("validation error should not wrap another error: %v", err)
	}
}
