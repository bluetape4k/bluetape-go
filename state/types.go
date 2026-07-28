package state

import "context"

// Guard func 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Guard[S comparable, E comparable] func(context.Context, S, E) error

// Transition struct 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Transition[S comparable, E comparable] struct {
	From  S
	Event E
	To    S
	Guard Guard[S, E]
}

// Result struct 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Result[S comparable, E comparable] struct {
	Previous S
	Event    E
	Current  S
}

// Option func 공개 타입이며 상태 전이, guard, final state 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Option[S comparable, E comparable] func(*config[S, E])

type config[S comparable, E comparable] struct {
	finalStates map[S]struct{}
}

// WithFinalStates WithFinalStates 공개 API의 동작을 수행하며 상태 전이, guard, final state 계약을 보존한다.
//
// 매개변수:
//   - states: WithFinalStates 동작에 필요한 states 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
