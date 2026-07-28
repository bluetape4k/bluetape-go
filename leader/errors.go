package leader

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

var (
	// ErrAlreadyLeader elector가 이미 leader일 때 반환된다.
	ErrAlreadyLeader = errors.New("leader: already leader")

	// ErrNotLeader 다른 member가 leader일 때 반환된다.
	ErrNotLeader = errors.New("leader: not leader")

	// ErrCampaignInProgress 같은 elector에서 campaign이 이미 진행 중일 때 반환된다.
	ErrCampaignInProgress = errors.New("leader: campaign in progress")

	// ErrCleanupPending 은 이전 leadership 정리가 완료되지 않았을 때 반환된다.
	ErrCleanupPending = errors.New("leader: cleanup pending")

	// ErrInvalidContext nil context가 허용되지 않는 작업에 전달됐을 때 반환된다.
	ErrInvalidContext = errors.New("leader: invalid context")

	// ErrCommitUnknown 은 backend 변경이 반영됐는지 확인할 수 없을 때 반환된다.
	ErrCommitUnknown = errors.New("leader: commit unknown")
)

// OperationError provider 작업 실패의 원인을 보존하면서 진단 문자열을 정제한다.
type OperationError struct {
	backend   string
	operation string
	cause     error
}

// NewOperationError backend 작업 실패를 정제된 오류로 감싼다.
//
// 잘못된 metadata 또는 nil cause에는 *OperationError 대신 검증 오류를 반환한다.
func NewOperationError(backend, operation string, cause error) error {
	if !validOperationLabel(backend) || !validOperationLabel(operation) || cause == nil {
		return errors.New("leader: invalid operation error")
	}
	return &OperationError{backend: backend, operation: operation, cause: cause}
}

// Error raw provider 오류 문자열을 포함하지 않는 진단을 반환한다.
func (e *OperationError) Error() string {
	if e == nil || !validOperationLabel(e.backend) || !validOperationLabel(e.operation) || e.cause == nil {
		return "leader operation failed"
	}
	return fmt.Sprintf("leader %s %s failed: %s", e.backend, e.operation, reflect.TypeOf(e.cause))
}

// Unwrap 은 원래 provider 오류를 반환한다.
func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 원래 provider 오류에 대한 errors.Is 판정을 보존한다.
func (e *OperationError) Is(target error) bool {
	return e != nil && errors.Is(e.cause, target)
}

// Backend 정제된 backend label을 반환한다.
func (e *OperationError) Backend() string {
	if e == nil || !validOperationLabel(e.backend) {
		return "unknown"
	}
	return e.backend
}

// Operation 은 정제된 operation label을 반환한다.
func (e *OperationError) Operation() string {
	if e == nil || !validOperationLabel(e.operation) {
		return "unknown"
	}
	return e.operation
}

func validOperationLabel(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 32 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
