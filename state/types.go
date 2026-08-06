package state

import "context"

// Guard 상태 전이, guard, final state에서 사용하는 함수 타입이다.
type Guard[S comparable, E comparable] func(context.Context, S, E) error

// Transition 상태 전이, guard, final state에서 사용하는 구조체다.
type Transition[S comparable, E comparable] struct {
	From  S
	Event E
	To    S
	Guard Guard[S, E]
}

// Result 상태 전이, guard, final state에서 사용하는 구조체다.
type Result[S comparable, E comparable] struct {
	Previous S
	Event    E
	Current  S
}

// Option 상태 전이, guard, final state에서 사용하는 함수 타입이다.
type Option[S comparable, E comparable] func(*config[S, E])

type config[S comparable, E comparable] struct {
	finalStates map[S]struct{}
}

// WithFinalStates 상태 전이, guard, final state 옵션을 설정한다.
//
// 매개변수:
//   - states: 최종 상태로 취급할 상태 목록이다.
func WithFinalStates[S comparable, E comparable](states ...S) Option[S, E] {
	return func(cfg *config[S, E]) {
		if cfg.finalStates == nil {
			cfg.finalStates = make(map[S]struct{}, len(states))
		}
		for _, item := range states {
			cfg.finalStates[item] = struct{}{}
		}
	}
}
