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

// ErrCallbackContractViolation 변수 공개 값이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 호출자는 이 식별자를 오류, 상태, 이벤트, 옵션, 또는 기본값 계약을 비교할 때 사용한다.
var ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")

// AtomicityPanic struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AtomicityPanic struct {
	panicValue any
}

// Error Error 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (*AtomicityPanic) Error() string { return "sql checkpoint: callback panic with unknown atomicity" }

// Unwrap Unwrap 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (*AtomicityPanic) Unwrap() error {
	return errors.Join(batch.ErrAtomicityUnknown, batch.ErrCommitUnknown)
}

// PanicValue PanicValue 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (p *AtomicityPanic) PanicValue() any {
	if p == nil {
		return nil
	}
	return p.panicValue
}

// OpError struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type OpError struct {
	operation string
	keyID     string
	err       error
}

// Error Error 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (e *OpError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap Unwrap 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family Family 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (*OpError) Family() string { return "sql checkpoint" }

// Operation Operation 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (e *OpError) Operation() string {
	if e == nil || e.operation == "" {
		return "operation"
	}
	return e.operation
}

// KeyID KeyID 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
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

// CodecError struct 공개 타입이며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CodecError struct {
	operation string
	err       error
}

// Error Error 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (e *CodecError) Error() string { return e.Family() + " " + e.Operation() + " failed" }

// Unwrap Unwrap 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e *CodecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Family Family 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
func (*CodecError) Family() string { return "checkpoint codec" }

// Operation Operation 공개 API의 동작을 수행하며 batch 단계, checkpoint, writer 안전성, 재시작 계약을 보존한다.
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
