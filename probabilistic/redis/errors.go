package redisbloom

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidOptions is returned when Redis Bloom options are invalid.
	ErrInvalidOptions = errors.New("redis bloom: invalid options")
	// ErrConfigMismatch is returned when stored metadata does not match caller config.
	ErrConfigMismatch = errors.New("redis bloom: config mismatch")
	// ErrConfigCorrupt is returned when stored metadata is missing or incomplete.
	ErrConfigCorrupt = errors.New("redis bloom: config corrupt")
)

// RedisError wraps an operational Redis failure with a redacted key id.
type RedisError struct {
	Operation string
	KeyID     string
	Err       error
}

func (e RedisError) Error() string {
	if e.KeyID == "" {
		return fmt.Sprintf("redis bloom %s: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("redis bloom %s %s: %v", e.Operation, e.KeyID, e.Err)
}

func (e RedisError) Unwrap() error {
	return e.Err
}

func mapScriptError(operation string, keyID string, err error) error {
	if err == nil {
		return nil
	}
	switch scriptErrorMarker(err) {
	case "config_mismatch":
		return fmt.Errorf("%w: %s", ErrConfigMismatch, keyID)
	case "config_corrupt":
		return fmt.Errorf("%w: %s", ErrConfigCorrupt, keyID)
	default:
		return RedisError{Operation: operation, KeyID: keyID, Err: err}
	}
}

func scriptErrorMarker(err error) string {
	message := strings.TrimSpace(err.Error())
	message = strings.TrimPrefix(message, "ERR ")
	switch message {
	case "config_mismatch", "config_corrupt":
		return message
	default:
		return ""
	}
}
