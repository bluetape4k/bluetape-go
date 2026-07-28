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

// TransitionError struct 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// Unwrap Unwrap 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (e TransitionError[S, E]) Unwrap() error {
	return e.Cause
}

// Is Is 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 매개변수:
//   - target: 검사하거나 감쌀 오류 값이다.
func (e TransitionError[S, E]) Is(target error) bool {
	return target == e.Kind
}
