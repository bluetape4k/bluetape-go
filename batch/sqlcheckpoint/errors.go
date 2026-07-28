package sqlcheckpoint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/bluetape4k/bluetape-go/batch"
)

// Stable OpError operation names for caller-side classification and bounded metrics.
const (
	OperationLoad           = "load"
	OperationBegin          = "begin"
	OperationSavepoint      = "savepoint"
	OperationCallback       = "callback"
	OperationOwnershipProbe = "ownership probe"
	OperationCheckpoint     = "checkpoint"
	OperationCommit         = "commit"
	OperationRollback       = "rollback"
)

// ErrCallbackContractViolation batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 공개 변수 값이다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")

// AtomicityPanic batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type AtomicityPanic struct {
	panicValue any
}

// Error 오류 메시지를 반환한다.
func (*AtomicityPanic) Error() string { return "sql checkpoint: callback panic with unknown atomicity" }

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (*AtomicityPanic) Unwrap() error {
	return errors.Join(batch.ErrAtomicityUnknown, batch.ErrCommitUnknown)
}

// PanicValue batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
func (p *AtomicityPanic) PanicValue() any {
	if p == nil {
		return nil
	}
	return p.panicValue
}

// OpError batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type OpError struct {
	operation string
	keyID     string
	err       error
}

// Error 오류 메시지를 반환한다.
func (e *OpError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
func (*OpError) Family() string { return "sql checkpoint" }

// Operation batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
func (e *OpError) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

// KeyID batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
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

// CodecError batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type CodecError struct {
	operation string
	err       error
}

// Error 오류 메시지를 반환한다.
func (e *CodecError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
func (*CodecError) Family() string { return "checkpoint codec" }

// Operation batch 단계, checkpoint, writer 안전성, 재시작의 식별 정보를 반환한다.
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
