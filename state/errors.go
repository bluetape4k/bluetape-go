package state

import (
	"errors"
	"fmt"
)

// Sentinel errors used with errors.Is.
var (
	ErrInvalidTransition    = errors.New("invalid transition")
	ErrGuardRejected        = errors.New("guard rejected transition")
	ErrFinalState           = errors.New("state is final")
	ErrConcurrentTransition = errors.New("concurrent transition conflict")
	ErrDuplicateTransition  = errors.New("duplicate transition")
	ErrUnknownInitialState  = errors.New("unknown initial state")
)

// TransitionError 상태 전이, guard, final state에서 사용하는 구조체다.
type TransitionError[S comparable, E comparable] struct {
	Kind  error
	From  S
	Event E
	To    S
	Cause error
}

func (e TransitionError[S, E]) Error() string {
	message := fmt.Sprintf("%v: from=%v event=%v to=%v", e.Kind, e.From, e.Event, e.To)
	if e.Cause != nil {
		return message + ": " + e.Cause.Error()
	}
	return message
}

// Unwrap 감싼 원인 오류를 반환한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e TransitionError[S, E]) Unwrap() error {
	return e.Cause
}

// Is errors.Is 비교를 지원한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e TransitionError[S, E]) Is(target error) bool {
	return target == e.Kind
}
