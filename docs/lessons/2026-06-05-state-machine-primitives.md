# State Machine Primitives

Issue: #26
Milestone: 0.4.0

## Lesson

Go FSM guards are ordinary caller code, so inspection APIs need clear semantics:
`CanTransition` may execute a guard and therefore requires inquiry-safe guards,
while `AllowedEvents` must stay a structural registry query that does not
evaluate guards.

## Applied Guardrails

- Guards run outside the machine lock and the current state is rechecked before
  commit.
- Concurrent guarded transitions return one success and deterministic
  `ErrConcurrentTransition` errors for losers.
- `TransitionError` preserves both package sentinel errors and wrapped
  guard/context causes for `errors.Is`.
