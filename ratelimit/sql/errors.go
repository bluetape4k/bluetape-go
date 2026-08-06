package sqlratelimit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrConfigurationMismatch indicates that an existing bucket uses different options.
var ErrConfigurationMismatch = errors.New("sql rate limiter: configuration mismatch")

// OpError wraps a PostgreSQL rate-limit operation failure with redacted diagnostics.
type OpError struct {
	operation string
	keyID     string
	err       error
}

var _ ratelimit.OperationError = (*OpError)(nil)

// Error returns a sanitized message without key, connection, SQL, or cause text.
func (e *OpError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap returns the causal database or context error.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family returns the provider-neutral operation family.
func (*OpError) Family() string { return "rate limiter" }

// Operation returns the low-cardinality operation name.
func (e *OpError) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

// KeyID returns a redacted correlation identifier for sampled diagnostics.
func (e *OpError) KeyID() string {
	if e == nil || e.keyID == "" {
		return "sql-rate-key:<missing>"
	}
	return e.keyID
}

func newOperationError(operation, namespace, key string, err error) error {
	return &OpError{operation: operation, keyID: redactedKeyID(namespace, key), err: err}
}

func newCleanupOperationError(err error) error {
	return &OpError{operation: "cleanup", keyID: "sql-rate-key:<cleanup>", err: err}
}

func redactedKeyID(namespace, key string) string {
	hash := sha256.New()
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(namespace)))
	_, _ = hash.Write(size[:])
	_, _ = hash.Write([]byte(namespace))
	_, _ = hash.Write([]byte(key))
	return "sql-rate-key:" + hex.EncodeToString(hash.Sum(nil)[:10])
}

func classifyOperationError(operation, namespace, key string, err, contextErr error) error {
	cause := err
	if contextErr != nil {
		cause = errors.Join(err, contextErr)
	}
	opErr := newOperationError(operation, namespace, key, cause)
	var serverErr *pgconn.PgError
	if errors.As(err, &serverErr) {
		return opErr
	}
	return errors.Join(opErr, ratelimit.ErrCommitUnknown)
}

func classifyCleanupError(err, contextErr error) error {
	cause := err
	if contextErr != nil {
		cause = errors.Join(err, contextErr)
	}
	opErr := newCleanupOperationError(cause)
	var serverErr *pgconn.PgError
	if errors.As(err, &serverErr) {
		return opErr
	}
	return errors.Join(opErr, ratelimit.ErrCommitUnknown)
}
