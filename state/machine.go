package state

import (
	"context"
	"errors"
	"sync"
)

type transitionKey[S comparable, E comparable] struct {
	from  S
	event E
}

type transitionTarget[S comparable, E comparable] struct {
	to    S
	guard Guard[S, E]
}

// Machine struct 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Machine[S comparable, E comparable] struct {
	mu          sync.RWMutex
	current     S
	transitions map[transitionKey[S, E]]transitionTarget[S, E]
	allowed     map[S][]E
	finalStates map[S]struct{}
}

// NewMachine NewMachine 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 매개변수:
//   - initial: NewMachine 동작에 필요한 initial 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - transitions: NewMachine가 순서와 snapshot 의미를 유지하며 읽는 transitions 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - options: NewMachine 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func NewMachine[S comparable, E comparable](
	initial S,
	transitions []Transition[S, E],
	options ...Option[S, E],
) (*Machine[S, E], error) {
	cfg := config[S, E]{finalStates: make(map[S]struct{})}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	registered := make(map[transitionKey[S, E]]transitionTarget[S, E], len(transitions))
	allowed := make(map[S][]E)
	knownStates := make(map[S]struct{})

	for state := range cfg.finalStates {
		knownStates[state] = struct{}{}
	}
	for _, transition := range transitions {
		key := transitionKey[S, E]{from: transition.From, event: transition.Event}
		if _, exists := registered[key]; exists {
			return nil, TransitionError[S, E]{
				Kind:  ErrDuplicateTransition,
				From:  transition.From,
				Event: transition.Event,
				To:    transition.To,
			}
		}
		registered[key] = transitionTarget[S, E]{
			to:    transition.To,
			guard: transition.Guard,
		}
		allowed[transition.From] = append(allowed[transition.From], transition.Event)
		knownStates[transition.From] = struct{}{}
		knownStates[transition.To] = struct{}{}
	}

	if _, exists := knownStates[initial]; !exists {
		return nil, TransitionError[S, E]{
			Kind: ErrUnknownInitialState,
			From: initial,
		}
	}

	return &Machine[S, E]{
		current:     initial,
		transitions: registered,
		allowed:     allowed,
		finalStates: cfg.finalStates,
	}, nil
}

// State State 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
func (m *Machine[S, E]) State() S {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Transition Transition 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - event: Transition 동작에 필요한 event 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m *Machine[S, E]) Transition(ctx context.Context, event E) (Result[S, E], error) {
	var zero Result[S, E]
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	from, target, err := m.lookup(event)
	if err != nil {
		return zero, err
	}

	if target.guard != nil {
		if err := target.guard(ctx, from, event); err != nil {
			return zero, TransitionError[S, E]{
				Kind:  ErrGuardRejected,
				From:  from,
				Event: event,
				To:    target.to,
				Cause: err,
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != from {
		return zero, TransitionError[S, E]{
			Kind:  ErrConcurrentTransition,
			From:  from,
			Event: event,
			To:    target.to,
		}
	}
	m.current = target.to
	return Result[S, E]{
		Previous: from,
		Event:    event,
		Current:  target.to,
	}, nil
}

// CanTransition CanTransition 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - event: CanTransition 동작에 필요한 event 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m *Machine[S, E]) CanTransition(ctx context.Context, event E) (bool, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}

	from, target, err := m.lookup(event)
	if err != nil {
		if errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrFinalState) {
			return false, nil
		}
		return false, err
	}

	if target.guard == nil {
		return true, nil
	}
	if err := target.guard(ctx, from, event); err != nil {
		return false, TransitionError[S, E]{
			Kind:  ErrGuardRejected,
			From:  from,
			Event: event,
			To:    target.to,
			Cause: err,
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return true, nil
}

// AllowedEvents AllowedEvents 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
func (m *Machine[S, E]) AllowedEvents() []E {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, final := m.finalStates[m.current]; final {
		return []E{}
	}
	events := m.allowed[m.current]
	return append([]E(nil), events...)
}

func (m *Machine[S, E]) lookup(event E) (S, transitionTarget[S, E], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from := m.current
	if _, final := m.finalStates[from]; final {
		return from, transitionTarget[S, E]{}, TransitionError[S, E]{
			Kind:  ErrFinalState,
			From:  from,
			Event: event,
		}
	}

	target, exists := m.transitions[transitionKey[S, E]{from: from, event: event}]
	if !exists {
		return from, transitionTarget[S, E]{}, TransitionError[S, E]{
			Kind:  ErrInvalidTransition,
			From:  from,
			Event: event,
		}
	}
	return from, target, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
