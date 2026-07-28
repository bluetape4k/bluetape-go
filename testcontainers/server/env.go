package server

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ErrInvalidEnvName는 Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
var ErrInvalidEnvName = errors.New("invalid environment variable name")

// ExportEnv는 Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
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
