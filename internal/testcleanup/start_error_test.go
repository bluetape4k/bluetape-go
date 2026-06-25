package testcleanup

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFormatStartErrorClassifiesFailures(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		category string
	}{
		{
			name:     "docker unavailable",
			err:      errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock"),
			category: "docker unavailable",
		},
		{
			name:     "image pull failure",
			err:      errors.New("pull access denied for redis:missing, repository does not exist"),
			category: "image pull failure",
		},
		{
			name:     "readiness timeout",
			err:      errors.Join(errors.New("wait strategy failed"), context.DeadlineExceeded),
			category: "readiness timeout",
		},
		{
			name:     "context cancelled",
			err:      context.Canceled,
			category: "context canceled",
		},
		{
			name:     "wrapper failure",
			err:      errors.New("invalid testcontainers option"),
			category: "wrapper failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStartError("redis", "redis:7.4-alpine", tt.err)

			for _, want := range []string{"redis", "redis:7.4-alpine", tt.category, tt.err.Error()} {
				if !strings.Contains(got, want) {
					t.Fatalf("FormatStartError() = %q, want substring %q", got, want)
				}
			}
		})
	}
}

func TestFormatStartErrorHandlesNilError(t *testing.T) {
	got := FormatStartError("redis", "redis:7.4-alpine", nil)

	if !strings.Contains(got, "wrapper failure") {
		t.Fatalf("FormatStartError() = %q, want wrapper failure", got)
	}
}
