package sqlratelimit

import (
	"errors"
	"fmt"
	"testing"
)

func TestConfigurationMismatchSupportsNestedInspection(t *testing.T) {
	if !errors.Is(fmt.Errorf("nested: %w", ErrConfigurationMismatch), ErrConfigurationMismatch) {
		t.Fatal("nested ErrConfigurationMismatch did not match")
	}
}
