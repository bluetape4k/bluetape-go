# State Machine Primitives

Issue: #26
Milestone: 0.4.0

## Lesson

Go FSM guard는 ordinary caller code이므로 inspection API의 semantics가 명확해야
한다. `CanTransition`은 guard를 실행할 수 있으므로 inquiry-safe guard를 요구하고,
`AllowedEvents`는 guard를 평가하지 않는 structural registry query로 유지한다.

## Applied Guardrails

- guard는 machine lock 밖에서 실행하고, commit 전에 current state를 다시 확인한다.
- concurrent guarded transition은 하나만 success하고 loser에게 deterministic
  `ErrConcurrentTransition` error를 반환한다.
- `TransitionError`는 package sentinel error와 wrapped guard/context cause를 모두
  보존해 `errors.Is`가 동작하게 한다.
