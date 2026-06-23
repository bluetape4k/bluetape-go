package server

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ErrInvalidEnvName reports that an environment export target is blank.
var ErrInvalidEnvName = errors.New("invalid environment variable name")

// ExportEnv maps connection details into test-scoped environment variables.
func ExportEnv(tb testing.TB, details ConnectionDetails, names map[string]string) error {
	tb.Helper()

	values := make(map[string]string, len(names))
	for key, envName := range names {
		if strings.TrimSpace(envName) == "" {
			return fmt.Errorf("%w: detail key %s", ErrInvalidEnvName, key)
		}
		value, ok := details.Get(key)
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingDetail, key)
		}
		values[envName] = value
	}

	for envName, value := range values {
		tb.Setenv(envName, value)
	}
	return nil
}
