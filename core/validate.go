package core

import (
	"fmt"
	"strings"
)

// RequireNotBlank returns an error when value is empty or only whitespace.
func RequireNotBlank(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be blank", name)
	}
	return nil
}
