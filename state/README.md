# state

`state` provides small finite state machine primitives for ordinary Go code.
It keeps the API framework-free: callers define comparable state and event
values, explicit transitions, optional guards, and final states.

```go
machine, err := state.NewMachine(
    "created",
    []state.Transition[string, string]{
        {From: "created", Event: "pay", To: "paid"},
    },
)
if err != nil {
    return err
}

result, err := machine.Transition(context.Background(), "pay")
```

## Contracts

- `Transition` applies one event or returns an error without mutating state.
- Guards receive the caller `context.Context` and can reject a transition by
  returning an error.
- Guards run outside the internal lock; the machine re-checks state before
  commit so concurrent callers get deterministic conflict errors.
- `CanTransition` may evaluate guard code without mutating the machine. Guards
  used for inquiry calls should avoid irreversible side effects.
- `AllowedEvents` returns registered events for the current state in
  registration order. It does not evaluate guards and does not guarantee that
  `Transition` will succeed.
- Sentinel errors work with `errors.Is`, including `ErrInvalidTransition`,
  `ErrGuardRejected`, `ErrFinalState`, `ErrConcurrentTransition`,
  `ErrDuplicateTransition`, and `ErrUnknownInitialState`.
