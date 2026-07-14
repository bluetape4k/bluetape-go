package sqlcheckpoint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/bluetape4k/bluetape-go/batch"
)

// ErrCallbackContractViolation indicates positive evidence that a callback ended
// or replaced the transaction frame owned by the checkpoint writer.
var ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")

// AtomicityPanic is raised when a callback panics and transaction ownership
// cannot be proven. PanicValue is sensitive and intended only for trusted
// top-level recovery code.
type AtomicityPanic struct {
	panicValue any
}

// Error returns a fixed message that does not render the panic or provider cause.
func (*AtomicityPanic) Error() string { return "sql checkpoint: callback panic with unknown atomicity" }

// Unwrap returns only the sanitized recovery barriers.
func (*AtomicityPanic) Unwrap() error {
	return errors.Join(batch.ErrAtomicityUnknown, batch.ErrCommitUnknown)
}

// PanicValue returns the original callback panic value for trusted diagnostics.
func (p *AtomicityPanic) PanicValue() any {
	if p == nil {
		return nil
	}
	return p.panicValue
}

// OpError wraps a SQL checkpoint operation failure with redacted diagnostics.
type OpError struct {
	operation string
	keyID     string
	err       error
}

// Error returns a sanitized message without namespace, key, payload, SQL, connection, or cause text.
func (e *OpError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap returns the causal database error.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family returns the low-cardinality operation family.
func (*OpError) Family() string { return "sql checkpoint" }

// Operation returns the low-cardinality operation name.
func (e *OpError) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

// KeyID returns a sensitive pseudonymous diagnostic identifier.
// It must not be used as a metric label, authorization identifier, or external trust-boundary value.
func (e *OpError) KeyID() string {
	if e == nil || e.keyID == "" {
		return "sql-checkpoint-key:<missing>"
	}
	return e.keyID
}

func newOperationError(operation string, namespace, key []byte, err error) error {
	return &OpError{
		operation: operation,
		keyID:     redactedKeyID(namespace, key),
		err:       err,
	}
}

// CodecError wraps a checkpoint codec failure without rendering payload or cause text.
type CodecError struct {
	operation string
	err       error
}

// Error returns a sanitized codec operation message.
func (e *CodecError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap returns the causal codec error.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family returns the low-cardinality codec family.
func (*CodecError) Family() string { return "checkpoint codec" }

// Operation returns the low-cardinality codec operation name.
func (e *CodecError) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

func newCodecError(operation string, err error) error {
	return &CodecError{operation: operation, err: err}
}

func redactedKeyID(namespace, key []byte) string {
	hash := sha256.New()
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(namespace)))
	_, _ = hash.Write(size[:])
	_, _ = hash.Write(namespace)
	_, _ = hash.Write(key)
	return "sql-checkpoint-key:" + hex.EncodeToString(hash.Sum(nil)[:10])
}
