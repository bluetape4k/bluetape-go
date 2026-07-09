package btredis

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

var (
	// ErrInvalidKey is returned when a Redis key or key-related label is invalid.
	ErrInvalidKey = errors.New("redis: invalid key")
	// ErrInvalidHashTag is returned when a Redis Cluster hash tag is invalid.
	ErrInvalidHashTag = errors.New("redis: invalid hash tag")
	// ErrInvalidTTL is returned when a Redis TTL is invalid.
	ErrInvalidTTL = errors.New("redis: invalid ttl")
)

// OpLabels are low-cardinality labels for Redis operation diagnostics.
type OpLabels struct {
	Family    string
	Operation string
}

// OpError wraps a Redis operation failure with redacted diagnostics.
type OpError struct {
	family    string
	operation string
	keyID     string
	err       error
}

// NewOpError returns a redacted Redis operation error for a raw Redis key.
func NewOpError(labels OpLabels, rawKey string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, RedactedKeyID(rawKey), err)
}

// NewOpErrorWithRedactedKey returns a Redis operation error for a pre-redacted key id.
func NewOpErrorWithRedactedKey(labels OpLabels, redactedKeyID string, err error) error {
	if err := labels.validate(); err != nil {
		return err
	}
	if err := ValidateRedactedKeyID(redactedKeyID); err != nil {
		return err
	}
	return newOpErrorWithKeyID(labels, redactedKeyID, err)
}

// Error returns a sanitized message without raw keys, tokens, or provider text.
func (e *OpError) Error() string {
	if e == nil {
		return "redis operation failed"
	}
	cause := "<nil>"
	if e.err != nil {
		cause = reflect.TypeOf(e.err).String()
	}
	return fmt.Sprintf("%s %s failed for %s: %s", e.Family(), e.Operation(), e.KeyID(), cause)
}

// Unwrap returns the causal Redis/context error.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Is delegates matching to the causal error.
func (e *OpError) Is(target error) bool {
	if e == nil {
		return false
	}
	return errors.Is(e.err, target)
}

// Family returns the low-cardinality Redis operation family.
func (e *OpError) Family() string {
	if e == nil || !validLabel(e.family) {
		return "redis"
	}
	return e.family
}

// Operation returns the low-cardinality Redis operation name.
func (e *OpError) Operation() string {
	if e == nil || !validLabel(e.operation) {
		return "operation"
	}
	return e.operation
}

// KeyID returns the redacted Redis key correlation id.
func (e *OpError) KeyID() string {
	if e == nil || ValidateRedactedKeyID(e.keyID) != nil {
		return "redis-key:<missing>"
	}
	return e.keyID
}

func newOpErrorWithKeyID(labels OpLabels, keyID string, err error) error {
	return &OpError{
		family:    labels.Family,
		operation: labels.Operation,
		keyID:     keyID,
		err:       err,
	}
}

func (l OpLabels) validate() error {
	if !validLabel(l.Family) || !validLabel(l.Operation) {
		return invalidKey("operation labels")
	}
	return nil
}

func validLabel(label string) bool {
	if label != strings.TrimSpace(label) || label == "" || len(label) > 64 || strings.ContainsAny(label, "{}:") {
		return false
	}
	for _, r := range label {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
