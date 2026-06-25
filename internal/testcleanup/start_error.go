package testcleanup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// FormatStartError returns a caller-visible Testcontainers start failure
// message with an operational category.
func FormatStartError(name string, image string, err error) string {
	category := classifyStartError(err)
	if err == nil {
		return fmt.Sprintf("start %s container (%s): %s: <nil>", name, image, category)
	}
	return fmt.Sprintf("start %s container (%s): %s: %v", name, image, category, err)
}

func classifyStartError(err error) string {
	if err == nil {
		return "wrapper failure"
	}
	if errors.Is(err, context.Canceled) {
		return "context canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return "readiness timeout"
	}

	message := strings.ToLower(err.Error())
	switch {
	case containsAny(message,
		"cannot connect to the docker daemon",
		"docker daemon",
		"docker is not running",
		"docker host",
		"docker socket",
		"/var/run/docker.sock",
		"connection refused",
		"permission denied",
	):
		return "docker unavailable"
	case containsAny(message,
		"failed to pull image",
		"pull access denied",
		"manifest unknown",
		"repository does not exist",
		"requested access to the resource is denied",
		"image not found",
	):
		return "image pull failure"
	case containsAny(message,
		"deadline exceeded",
		"readiness",
		"timed out",
		"timeout",
		"wait strategy",
		"waiting for",
	):
		return "readiness timeout"
	default:
		return "wrapper failure"
	}
}

func containsAny(message string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(message, needle) {
			return true
		}
	}
	return false
}
